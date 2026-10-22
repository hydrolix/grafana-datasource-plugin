package plugin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headerLog records the credential headers each request to a server carried.
type headerLog struct {
	mu   sync.Mutex
	seen []http.Header
}

func (l *headerLog) record(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := http.Header{}
	for _, k := range []string{"Authorization", "X-ClickHouse-User", "X-ClickHouse-Key"} {
		if v := r.Header.Get(k); v != "" {
			h.Set(k, v)
		}
	}
	l.seen = append(l.seen, h)
}

func (l *headerLog) requests() []http.Header {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]http.Header(nil), l.seen...)
}

// redirectingPair is a cluster at https://example.com answering every request
// with a redirect to http://sub.example.com: a subdomain, and a downgrade to
// plain http. Both are hops on which http.Client keeps Authorization, and
// X-ClickHouse-Key travels on any hop at all. The two names resolve to the two
// listeners through the transport's dialer; httptest's certificate is issued
// for example.com, so the TLS leg is verified, not skipped.
type redirectingPair struct {
	cluster, elsewhere *httptest.Server
	elsewhereSaw       *headerLog
	roots              *x509.CertPool
}

func newRedirectingPair(t *testing.T) *redirectingPair {
	t.Helper()
	p := &redirectingPair{elsewhereSaw: &headerLog{}}
	p.elsewhere = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.elsewhereSaw.record(r)
		_, _ = fmt.Fprintln(w, "captured")
	}))
	t.Cleanup(p.elsewhere.Close)
	_, elsewherePort, _ := net.SplitHostPort(p.elsewhere.Listener.Addr().String())
	p.cluster = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://sub.example.com:"+elsewherePort+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	t.Cleanup(p.cluster.Close)
	p.roots = x509.NewCertPool()
	p.roots.AddCert(p.cluster.Certificate())
	return p
}

func (p *redirectingPair) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	switch host {
	case "example.com":
		return d.DialContext(ctx, network, p.cluster.Listener.Addr().String())
	case "sub.example.com":
		return d.DialContext(ctx, network, p.elsewhere.Listener.Addr().String())
	}
	return nil, fmt.Errorf("unexpected dial to %s", addr)
}

// options opens the cluster over TLS with a username and password, the mode
// in which the fork sends the password as X-ClickHouse-Key, through the given
// transport wrapper.
func (p *redirectingPair) options(wrap func(http.RoundTripper) http.RoundTripper) *clickhouse.Options {
	_, port, _ := net.SplitHostPort(p.cluster.Listener.Addr().String())
	return &clickhouse.Options{
		Protocol:    clickhouse.HTTP,
		Addr:        []string{"example.com:" + port},
		Auth:        clickhouse.Auth{Username: "alice", Password: "s3cret"},
		TLS:         &tls.Config{RootCAs: p.roots},
		Compression: &clickhouse.Compression{Method: clickhouse.CompressionNone},
		DialTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second,
		TransportFunc: func(tr *http.Transport) (http.RoundTripper, error) {
			tr.DialContext = p.dial
			return wrap(tr), nil
		},
	}
}

func TestRedirectsAreRefusedBeforeTheCredentialCanFollow(t *testing.T) {
	t.Run("witness: without the guard the password follows the redirect", func(t *testing.T) {
		// Pins the exposure the guard exists for, on the real request path.
		p := newRedirectingPair(t)
		db := clickhouse.OpenDB(p.options(func(rt http.RoundTripper) http.RoundTripper { return rt }))
		defer func() { _ = db.Close() }()
		_ = db.PingContext(context.Background())

		hops := p.elsewhereSaw.requests()
		require.NotEmpty(t, hops, "the redirect was not followed, so the exposure was not exercised")
		assert.Equal(t, "s3cret", hops[0].Get("X-ClickHouse-Key"), "the password followed the redirect")
	})

	t.Run("with the guard nothing reaches the redirect target", func(t *testing.T) {
		p := newRedirectingPair(t)
		db := clickhouse.OpenDB(p.options(refuseRedirects))
		defer func() { _ = db.Close() }()
		err := db.PingContext(context.Background())

		require.Error(t, err)
		var redirect *RedirectError
		require.True(t, errors.As(err, &redirect), "got %v", err)
		assert.Equal(t, http.StatusTemporaryRedirect, redirect.Status)
		assert.Contains(t, redirect.Location, "http://sub.example.com:")
		assert.Empty(t, p.elsewhereSaw.requests(), "a request reached the redirect target")
	})
}

func TestConnectRefusesARedirectInThePasswordMode(t *testing.T) {
	// Through the driver's own Connect, so the wiring is what is tested: the
	// datasource points at a TLS listener on 127.0.0.1 that redirects to a
	// plain-http listener on the same host, a hop on which http.Client keeps
	// every header. Skip-verify is on because Connect checks the system
	// roots, which do not hold httptest's certificate.
	elsewhereSaw := &headerLog{}
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhereSaw.record(r)
		_, _ = fmt.Fprintln(w, "captured")
	}))
	defer elsewhere.Close()
	cluster := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+r.URL.RequestURI(), http.StatusPermanentRedirect)
	}))
	defer cluster.Close()
	_, port, _ := net.SplitHostPort(cluster.Listener.Addr().String())
	portNum, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)

	settings := models.PluginSettings{
		Host: "127.0.0.1", Port: uint16(portNum), Protocol: "http",
		Secure: true, SkipTlsVerify: true,
		CredentialsType: "userAccount", UserName: "alice",
		DialTimeout: "5", QueryTimeout: "5",
	}
	jsonData, err := json.Marshal(settings)
	require.NoError(t, err)

	_, err = NewHydrolix().Connect(context.Background(), backend.DataSourceInstanceSettings{
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{"password": "s3cret"},
	}, nil)

	require.Error(t, err)
	var redirect *RedirectError
	require.True(t, errors.As(err, &redirect), "got %v", err)
	assert.Equal(t, http.StatusPermanentRedirect, redirect.Status)
	assert.Empty(t, elsewhereSaw.requests(), "a request reached the redirect target")

	// The panel gets the reason alone, not the fork's request plumbing.
	mutated := NewHydrolix().MutateQueryError(fmt.Errorf("sendQuery: %w", err))
	assert.True(t, backend.IsDownstreamError(mutated))
	assert.True(t, strings.HasPrefix(mutated.Error(), "the cluster answered HTTP 308 redirecting to"), mutated.Error())
}

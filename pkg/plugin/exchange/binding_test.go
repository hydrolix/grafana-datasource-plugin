package exchange

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// recordingBase records what reached the wire and answers a scripted status.
type recordingBase struct {
	mu       sync.Mutex
	bearers  []string
	statuses []int
	calls    int
}

func (r *recordingBase) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bearers = append(r.bearers, req.Header.Get("Authorization"))
	status := http.StatusOK
	if r.calls < len(r.statuses) {
		status = r.statuses[r.calls]
	}
	r.calls++
	return &http.Response{
		StatusCode: status,
		Body:       http.NoBody,
		Header:     http.Header{},
		Request:    req,
	}, nil
}

func (r *recordingBase) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.bearers...)
}

func principalCtx() context.Context {
	return WithPrincipal(context.Background(), Principal{
		Audience: aud, Subject: sub, SubjectToken: stok,
	})
}

func newRequest(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://cluster.example/query",
		strings.NewReader("SELECT 1"))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// unbound is a binding for a connection that belongs to nobody: only a context
// principal can identify a request on it.
func unbound(s *Source) *Binding { return Bind(s, nil, "", "") }

func TestTheJWTIsTheExchangedTokenAndNeverTheForwardedOne(t *testing.T) {
	s, _, _ := newFixture(300 * time.Second)

	jwt, err := unbound(s).JWT(principalCtx())
	if err != nil {
		t.Fatal(err)
	}
	if jwt != "cluster-token-1" {
		t.Fatalf("jwt %q", jwt)
	}
	// The invariant: the console-realm token must never reach a cluster.
	if strings.Contains(jwt, stok) {
		t.Fatalf("the forwarded token was handed to the driver: %q", jwt)
	}
}

func TestARequestWithNoPrincipalIsRefused(t *testing.T) {
	s, ex, _ := newFixture(300 * time.Second)

	_, err := unbound(s).JWT(context.Background())
	if !errors.Is(err, ErrRefused) {
		t.Fatalf("expected a refusal, got %v", err)
	}
	if ex.count() != 0 {
		t.Fatal("a request with no identity caused an exchange")
	}
}

func TestAFailedExchangeNeverFallsBackToTheForwardedToken(t *testing.T) {
	// The whole lane rests on this: if the exchange fails, the query fails.
	ex := &fakeExchanger{lifetime: 300 * time.Second, err: ErrUnavailable}
	s := NewSource(ex, nil, nil)

	jwt, err := unbound(s).JWT(principalCtx())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
	if jwt != "" {
		t.Fatalf("a token was handed out despite a failed exchange: %q", jwt)
	}
}

func TestAClusterUnauthorizedDropsTheTokenSoTheNextRequestMintsAfresh(t *testing.T) {
	// The watch drops the entry; sqlds retries the query (driver.Settings lists
	// the 401 as retryable), and that retry's JWT call is a fresh mint.
	s, ex, _ := newFixture(300 * time.Second)
	b := unbound(s)
	base := &recordingBase{statuses: []int{http.StatusUnauthorized, http.StatusOK}}
	watched := b.Watch(base)

	first, err := b.JWT(principalCtx())
	if err != nil {
		t.Fatal(err)
	}
	resp, err := watched.RoundTrip(newRequest(t, principalCtx()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("the cluster's 401 must reach the caller unchanged, got %d", resp.StatusCode)
	}
	if tok := s.cachedToken(aud, sub); tok != "" {
		t.Fatalf("the refused token is still cached: %q", tok)
	}

	second, err := b.JWT(principalCtx())
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatalf("expected a fresh mint after the 401, got the same token %q", second)
	}
	if ex.count() != 2 {
		t.Fatalf("expected exactly one re-mint, got %d exchanges", ex.count())
	}
}

func TestAnOKResponseLeavesTheTokenAlone(t *testing.T) {
	s, ex, _ := newFixture(300 * time.Second)
	b := unbound(s)
	if _, err := b.JWT(principalCtx()); err != nil {
		t.Fatal(err)
	}

	if _, err := b.Watch(&recordingBase{}).RoundTrip(newRequest(t, principalCtx())); err != nil {
		t.Fatal(err)
	}
	if _, err := b.JWT(principalCtx()); err != nil {
		t.Fatal(err)
	}
	if ex.count() != 1 {
		t.Fatalf("a 200 must not cause a re-mint, got %d exchanges", ex.count())
	}
}

func TestTheWatchNeitherAttachesNorRemovesACredential(t *testing.T) {
	// Attaching is the fork's job, from the JWT hook, on the initial request.
	// The watch must not become a second place that handles the header — that
	// is exactly what would re-send the token on a redirected hop.
	s, _, _ := newFixture(300 * time.Second)
	base := &recordingBase{}
	watched := unbound(s).Watch(base)

	bare := newRequest(t, principalCtx())
	if _, err := watched.RoundTrip(bare); err != nil {
		t.Fatal(err)
	}
	set := newRequest(t, principalCtx())
	set.Header.Set("Authorization", "Bearer as-the-fork-built-it")
	if _, err := watched.RoundTrip(set); err != nil {
		t.Fatal(err)
	}

	seen := base.seen()
	if len(seen) != 2 || seen[0] != "" || seen[1] != "Bearer as-the-fork-built-it" {
		t.Fatalf("the watch touched the credential: %v", seen)
	}
	if got := set.Header.Get("Authorization"); got != "Bearer as-the-fork-built-it" {
		t.Fatalf("the caller's request was mutated: %q", got)
	}
}

func TestARedirectToAnotherHostDoesNotCarryTheToken(t *testing.T) {
	// Through clickhouse-go's own HTTP client, wired as the driver wires it:
	// the GetJWT hook puts the header on the initial request, http.Client
	// drops Authorization when a redirect leaves the host, and nothing below
	// it puts the header back. This is the property a token-attaching
	// RoundTripper broke, so it is pinned on the real request path rather
	// than on a bare client with the header set by hand. The cluster is
	// recorded too, so a run where the hook never attached anything cannot
	// pass by accident.
	var mu sync.Mutex
	var clusterSaw, elsewhereSaw []string
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		elsewhereSaw = append(elsewhereSaw, r.Header.Get("Authorization"))
		mu.Unlock()
		_, _ = fmt.Fprintln(w, "captured")
	}))
	defer elsewhere.Close()
	// httptest listens on 127.0.0.1; naming it "localhost" makes the redirect
	// cross hosts as far as http.Client's rule is concerned.
	elsewhereURL := strings.Replace(elsewhere.URL, "127.0.0.1", "localhost", 1)

	// TLS, because the hook is consulted only over TLS. The client trusts the
	// test server's certificate; a redirect to plain http is followed as is.
	cluster := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		clusterSaw = append(clusterSaw, r.Header.Get("Authorization"))
		mu.Unlock()
		http.Redirect(w, r, elsewhereURL+r.URL.RequestURI(), http.StatusTemporaryRedirect)
	}))
	defer cluster.Close()
	roots := x509.NewCertPool()
	roots.AddCert(cluster.Certificate())

	s, _, _ := newFixture(300 * time.Second)
	b := unbound(s)
	db := clickhouse.OpenDB(&clickhouse.Options{
		Protocol:    clickhouse.HTTP,
		Addr:        []string{cluster.Listener.Addr().String()},
		TLS:         &tls.Config{RootCAs: roots},
		Compression: &clickhouse.Compression{Method: clickhouse.CompressionNone},
		DialTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second,
		GetJWT:      b.JWT,
		TransportFunc: func(t *http.Transport) (http.RoundTripper, error) {
			return b.Watch(t), nil
		},
	})
	defer func() { _ = db.Close() }()

	// The answer from "elsewhere" is not a ClickHouse handshake, so the query
	// fails; what matters is where the header went on the way.
	rows, err := db.QueryContext(principalCtx(), "SELECT 1")
	if err == nil {
		_ = rows.Close()
	}

	mu.Lock()
	defer mu.Unlock()
	if len(clusterSaw) == 0 {
		t.Fatal("no request reached the cluster")
	}
	for _, auth := range clusterSaw {
		if auth != "Bearer cluster-token-1" {
			t.Fatalf("the cluster saw %q: the hook did not attach the exchanged token on the initial request", auth)
		}
	}
	if len(elsewhereSaw) == 0 {
		t.Fatal("the redirect was not followed, so the property was not exercised")
	}
	for _, auth := range elsewhereSaw {
		if auth != "" {
			t.Fatalf("the cluster token followed a redirect off the cluster: %q", auth)
		}
	}
}

func TestTheJWTWorksAgainstARealServer(t *testing.T) {
	// Through net/http rather than a fake, so the header really is on the wire
	// and the context really does survive to the watch.
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = fmt.Fprintln(w, "42")
	}))
	defer srv.Close()

	s, _, _ := newFixture(300 * time.Second)
	b := unbound(s)
	client := &http.Client{Transport: b.Watch(http.DefaultTransport)}
	req, err := http.NewRequestWithContext(principalCtx(), http.MethodPost, srv.URL, strings.NewReader("SELECT 1"))
	if err != nil {
		t.Fatal(err)
	}
	jwt, err := b.JWT(req.Context())
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got != "Bearer cluster-token-1" {
		t.Fatalf("the wire carried %q", got)
	}
}

func TestARequestWithNoContextIsIdentifiedByTheConnectionItBelongsTo(t *testing.T) {
	// What the first live run hit: the driver's own handshake ("server hello")
	// runs when database/sql establishes the connection, and the context it gets
	// is not the query's. The request was refused as carrying no signed-in user
	// even though Grafana had forwarded a perfectly good token.
	s, _, _ := newFixture(300 * time.Second)
	principals := NewPrincipals(0, nil)
	principals.Remember(Principal{Audience: aud, Subject: sub, SubjectToken: stok})

	jwt, err := Bind(s, principals, aud, sub).JWT(context.Background())
	if err != nil {
		t.Fatalf("a handshake on a bound connection must be identified: %v", err)
	}
	if jwt != "cluster-token-1" {
		t.Fatalf("jwt %q", jwt)
	}
}

func TestTheContextWinsOverTheConnectionsBinding(t *testing.T) {
	// The context carries the token of the very request being served, so it is
	// the better answer where both exist.
	s, _, _ := newFixture(300 * time.Second)
	principals := NewPrincipals(0, nil)
	principals.Remember(Principal{Audience: aud, Subject: "stale-subject", SubjectToken: "stale-token"})

	if _, err := Bind(s, principals, aud, "stale-subject").JWT(principalCtx()); err != nil {
		t.Fatal(err)
	}
	// One exchange, for the context's subject rather than the bound one.
	if tok := s.cachedToken(aud, sub); tok == "" {
		t.Fatal("the context's subject was not the one exchanged for")
	}
	if tok := s.cachedToken(aud, "stale-subject"); tok != "" {
		t.Fatal("the bound subject was used despite a context principal")
	}
}

func TestAnUnboundConnectionWithNoContextIsStillRefused(t *testing.T) {
	// The fallback must not become a way for an unidentified request to borrow
	// somebody's token: it only answers for the connection's own subject.
	s, _, _ := newFixture(300 * time.Second)
	principals := NewPrincipals(0, nil)
	principals.Remember(Principal{Audience: aud, Subject: sub, SubjectToken: stok})

	b := Bind(s, principals, "", "") // registry present, binding absent
	if _, err := b.JWT(context.Background()); !errors.Is(err, ErrRefused) {
		t.Fatalf("expected a refusal, got %v", err)
	}
}

func TestAForgottenPrincipalIsNotServed(t *testing.T) {
	// An entry unused past its idle window is dropped: a token Grafana has
	// stopped refreshing is of no further use, and keeping it would mean serving
	// a session nobody is in.
	s, _, ck := newFixture(300 * time.Second)
	principals := NewPrincipals(time.Minute, ck.now)
	principals.Remember(Principal{Audience: aud, Subject: sub, SubjectToken: stok})

	ck.advance(2 * time.Minute)

	if _, err := Bind(s, principals, aud, sub).JWT(context.Background()); !errors.Is(err, ErrRefused) {
		t.Fatalf("expected a refusal once the entry lapsed, got %v", err)
	}
}

func TestAPrincipalIsDroppedWhenItsTokenExpires(t *testing.T) {
	// Past its `exp` the facade would refuse the token, so the registry keeps it
	// no longer than that, however recently it was used.
	s, _, ck := newFixture(300 * time.Second)
	principals := NewPrincipals(time.Hour, ck.now)
	exp := ck.now().Add(5 * time.Minute).Unix()
	token := "header." + base64url(fmt.Sprintf(`{"iss":"https://kc","sub":"alice","exp":%d}`, exp)) + ".signature"
	principals.Remember(Principal{Audience: aud, Subject: sub, SubjectToken: token})

	if _, ok := principals.Lookup(aud, sub); !ok {
		t.Fatal("a live token was not served")
	}
	ck.advance(6 * time.Minute)

	if _, err := Bind(s, principals, aud, sub).JWT(context.Background()); !errors.Is(err, ErrRefused) {
		t.Fatalf("expected a refusal once the token expired, got %v", err)
	}
}

func TestSubjectOfReadsTheClaimWithoutVerifying(t *testing.T) {
	// A real token's middle segment, unsigned: the facade is the verifier.
	payload := `{"sub":"kc-sub-alice","aud":"grafana-api"}`
	jwt := "header." + base64url(payload) + ".signature"
	if got := SubjectOf(jwt); got != "kc-sub-alice" {
		t.Fatalf("subject %q", got)
	}
	for _, bad := range []string{"", "not-a-jwt", "a.b", "a.!!!.c"} {
		if got := SubjectOf(bad); got != "" {
			t.Fatalf("%q yielded a subject %q", bad, got)
		}
	}
}

func TestIdentityOfQualifiesTheSubjectWithItsIssuer(t *testing.T) {
	a := IdentityOf("h." + base64url(`{"iss":"https://kc.example/realms/console","sub":"1234"}`) + ".s")
	b := IdentityOf("h." + base64url(`{"iss":"https://accounts.google.com","sub":"1234"}`) + ".s")
	if a == "" || b == "" || a == b {
		t.Fatalf("the same sub from two issuers must be two identities: %q %q", a, b)
	}
	for _, missing := range []string{
		`{"sub":"1234"}`,
		`{"iss":"https://kc.example/realms/console"}`,
	} {
		if got := IdentityOf("h." + base64url(missing) + ".s"); got != "" {
			t.Fatalf("%s yielded an identity %q", missing, got)
		}
	}
}

func base64url(s string) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	out := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], s[i:])
		out = append(out, alphabet[chunk[0]>>2])
		out = append(out, alphabet[(chunk[0]&0x03)<<4|chunk[1]>>4])
		if n > 1 {
			out = append(out, alphabet[(chunk[1]&0x0f)<<2|chunk[2]>>6])
		}
		if n > 2 {
			out = append(out, alphabet[chunk[2]&0x3f])
		}
	}
	return string(out)
}

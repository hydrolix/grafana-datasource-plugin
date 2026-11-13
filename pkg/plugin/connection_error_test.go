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
	"net/url"
	"strconv"
	"syscall"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v5"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What a raw error would tell someone about the network: none of it may
// reach a message.
var networkDetails = []string{"10.0.0.5", "9440", "127.0.0.1", "127.0.0.11", "internal.example", "dial tcp", "lookup", "alice"}

func assertNoNetworkDetails(t *testing.T, message string) {
	t.Helper()
	for _, detail := range networkDetails {
		assert.NotContains(t, message, detail)
	}
}

func dialError(err error) error {
	return &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 9440}, Err: err}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestTransportMessageNamesTheKindOfFailureOnly(t *testing.T) {
	inURL := func(err error) error {
		return &url.Error{Op: "Post", URL: "https://alice:***@internal.example:9440/query?default_format=Native", Err: err}
	}
	cases := map[string]struct {
		err  error
		want string
	}{
		"unresolved host": {
			err:  dialError(&net.DNSError{Err: "no such host", Name: "internal.example", Server: "127.0.0.11:53", IsNotFound: true}),
			want: msgHostUnresolved,
		},
		"refused":                {err: dialError(syscall.ECONNREFUSED), want: msgRefused},
		"refused over http":      {err: inURL(dialError(syscall.ECONNREFUSED)), want: msgRefused},
		"dial timeout":           {err: dialError(timeoutError{}), want: msgConnectTimeout},
		"deadline while dialing": {err: dialError(context.DeadlineExceeded), want: msgConnectTimeout},
		"deadline on a request":  {err: inURL(context.DeadlineExceeded), want: msgRequestTimeout},
		"read timeout": {
			err:  &net.OpError{Op: "read", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 9440}, Err: timeoutError{}},
			want: msgRequestTimeout,
		},
		"untrusted certificate": {
			err:  inURL(&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}),
			want: msgTLS,
		},
		"plain port behind tls":      {err: tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, want: msgTLS},
		"http answer to https":       {err: inURL(http.ErrSchemeMismatch), want: msgTLS},
		"connection reset":           {err: &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}, want: msgCannotConnect},
		"no route":                   {err: dialError(syscall.EHOSTUNREACH), want: msgCannotConnect},
		"anything else under a url":  {err: inURL(errors.New("EOF")), want: msgCannotConnect},
		"refused, as sqlds's text":   {err: errors.New("dial tcp 10.0.0.5:9440: connect: connection refused"), want: msgRefused},
		"unresolved, as text":        {err: errors.New("dial tcp: lookup internal.example on 127.0.0.11:53: no such host"), want: msgHostUnresolved},
		"dial timeout, as text":      {err: errors.New("dial tcp 10.0.0.5:9440: i/o timeout"), want: msgConnectTimeout},
		"tls, as text":               {err: errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority"), want: msgTLS},
		"wrapped by the query layer": {err: fmt.Errorf("%w: %w", sqlds.ErrorQuery, dialError(syscall.ECONNREFUSED)), want: msgRefused},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := transportMessage(c.err)
			require.True(t, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestTransportMessageLeavesAnAnswerAlone(t *testing.T) {
	for name, err := range map[string]error{
		"clickhouse exception": &clickhouse.Exception{Code: 62, Name: "SYNTAX_ERROR", Message: "Syntax error: failed at position 8"},
		"http error with body": errors.New(`sendQuery: [HTTP 400] response body: "Code: 62. DB::Exception: Syntax error"`),
		"plain error":          errors.New("unexpected column type"),
		"http error echoing a query": errors.New(
			`sendQuery: [HTTP 400] response body: "Code: 62. DB::Exception: Syntax error: failed at position 7: SELEC 1 WHERE message ILIKE '%dial tcp: connection refused, no such host, i/o timeout, tls: x509: context deadline exceeded%'"`,
		),
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := transportMessage(err)
			assert.False(t, ok)
		})
	}
}

func TestConnectFailureRepeatsNothingOfWhatAnswered(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"wrong password, native": {
			err:  &clickhouse.Exception{Code: 516, Message: "alice: Authentication failed: password is incorrect"},
			want: msgAuthFailed,
		},
		"unknown user, as text": {err: errors.New("code: 192, message: There is no user alice"), want: msgAuthFailed},
		"another exception": {
			err:  &clickhouse.Exception{Code: 81, Message: "Database internal.example does not exist"},
			want: "The server refused the connection (ClickHouse error code 81)",
		},
		"wrong password, http": {
			err:  errors.New(`failed to query server hello: sendQuery: [HTTP 403] response body: "Code: 516. DB::Exception: alice: Authentication failed"`),
			want: msgAuthFailed,
		},
		"unauthorized": {err: errors.New(`sendQuery: [HTTP 401] response body: ""`), want: msgAuthFailed},
		"some other service's body": {
			err:  errors.New(`sendQuery: [HTTP 404] response body: "{\"message\": \"internal.example says no such host\"}"`),
			want: "The server answered HTTP 404",
		},
		"a body shaped like an exception": {
			err:  errors.New(`sendQuery: [HTTP 500] response body: "code: 516, message: internal.example"`),
			want: "The server answered HTTP 500",
		},
		"a body that could not be read": {
			err:  errors.New(`sendQuery: [HTTP 500] failed to read response: read tcp 10.0.0.5:9440: connection reset by peer`),
			want: "The server answered HTTP 500",
		},
		"refused":               {err: dialError(syscall.ECONNREFUSED), want: msgRefused},
		"deadline, any step":    {err: context.DeadlineExceeded, want: msgConnectTimeout},
		"nothing recognised":    {err: errors.New("unable to get default db connection"), want: msgCannotConnect},
		"something unexpected":  {err: errors.New("read: internal.example closed the stream"), want: msgCannotConnect},
		"exception inside text": {err: fmt.Errorf("handshake: %w", &clickhouse.Exception{Code: 193, Message: "wrong password for alice"}), want: msgAuthFailed},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := connectFailure(c.err)
			assert.Equal(t, c.want, got.Error())
			assertNoNetworkDetails(t, got.Error())
			assert.ErrorIs(t, got, c.err, "the cause stays underneath")
		})
	}

	t.Run("a redirect keeps its own message", func(t *testing.T) {
		redirect := &RedirectError{Status: http.StatusFound, Location: "http://internal.example/admin/"}
		got := connectFailure(fmt.Errorf("sendQuery: %w", redirect))
		assert.Same(t, redirect, got)
		assertNoNetworkDetails(t, got.Error())
	})
}

func instanceSettingsFor(t *testing.T, addr string, secure bool, protocol string) backend.DataSourceInstanceSettings {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	portNum, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)
	jsonData, err := json.Marshal(models.PluginSettings{
		Host: "127.0.0.1", Port: uint16(portNum), Protocol: protocol, Secure: secure,
		CredentialsType: "userAccount", UserName: "alice",
		DialTimeout: "5", QueryTimeout: "5",
	})
	require.NoError(t, err)
	return backend.DataSourceInstanceSettings{
		UID:                     "unreachable",
		JSONData:                jsonData,
		DecryptedSecureJSONData: map[string]string{"password": "s3cret"},
	}
}

// Through the instance factory, because that is where Save & test gets its
// message: a connect error fails the instance, and the SDK answers the health
// check with the instance error's text.
func TestAnInstanceThatCannotConnectFailsWithAFixedMessage(t *testing.T) {
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedAddr := closed.Addr().String()
	require.NoError(t, closed.Close())

	answering := func(status int, body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, body)
		}))
	}
	forbidden := answering(http.StatusForbidden, "Code: 516. DB::Exception: alice: Authentication failed (AUTHENTICATION_FAILED)")
	defer forbidden.Close()
	other := answering(http.StatusNotFound, `{"message": "internal service says not found"}`)
	defer other.Close()

	cases := map[string]struct {
		settings backend.DataSourceInstanceSettings
		want     string
	}{
		"refused, native":       {instanceSettingsFor(t, closedAddr, false, "native"), msgRefused},
		"refused, http":         {instanceSettingsFor(t, closedAddr, false, "http"), msgRefused},
		"tls to a plain port":   {instanceSettingsFor(t, other.Listener.Addr().String(), true, "http"), msgTLS},
		"wrong password":        {instanceSettingsFor(t, forbidden.Listener.Addr().String(), false, "http"), msgAuthFailed},
		"not a cluster at all":  {instanceSettingsFor(t, other.Listener.Addr().String(), false, "http"), "The server answered HTTP 404"},
		"refused, tls expected": {instanceSettingsFor(t, closedAddr, true, "native"), msgRefused},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewDatasource(context.Background(), c.settings)
			require.Error(t, err)
			assert.Equal(t, c.want, err.Error())
			assert.True(t, backend.IsDownstreamError(err))
			assertNoNetworkDetails(t, err.Error())
			assert.NotContains(t, err.Error(), "internal service")
		})
	}
}

func TestMutateQueryErrorHidesWhereAQueryFailedToReach(t *testing.T) {
	h := NewHydrolix()

	t.Run("a dial error becomes its kind", func(t *testing.T) {
		raw := fmt.Errorf("%w: %w", sqlds.ErrorQuery, dialError(syscall.ECONNREFUSED))
		got := h.MutateQueryError(raw)
		assert.Equal(t, msgRefused, got.Error())
		assert.Equal(t, backend.ErrorSourceDownstream, got.ErrorSource())
		assert.ErrorIs(t, got, sqlds.ErrorQuery, "sqlds still recognises a query error")
		assertNoNetworkDetails(t, got.Error())
	})

	t.Run("a deadline stays a deadline for sqlds", func(t *testing.T) {
		raw := fmt.Errorf("%w: %w", sqlds.ErrorQuery, &url.Error{Op: "Post", URL: "http://internal.example:9440", Err: context.DeadlineExceeded})
		got := h.MutateQueryError(raw)
		assert.Equal(t, msgRequestTimeout, got.Error())
		assert.ErrorIs(t, got, context.DeadlineExceeded)
	})

	t.Run("the cluster's own answer passes through", func(t *testing.T) {
		raw := fmt.Errorf("%w: %w", sqlds.ErrorQuery, errors.New(`sendQuery: [HTTP 400] response body: "Code: 62. DB::Exception: Syntax error"`))
		got := h.MutateQueryError(raw)
		assert.Contains(t, got.Error(), "Syntax error")
	})

	t.Run("an answer that repeats a query about connections passes through", func(t *testing.T) {
		raw := fmt.Errorf("%w: %w", sqlds.ErrorQuery, errors.New(`sendQuery: [HTTP 400] response body: "Code: 62. DB::Exception: Syntax error: failed at position 7: SELEC 1 WHERE message ILIKE '%connection refused%'"`))
		got := h.MutateQueryError(raw)
		assert.Contains(t, got.Error(), "Syntax error")
	})

	t.Run("a body that could not be read is still the network's", func(t *testing.T) {
		raw := fmt.Errorf("%w: %w", sqlds.ErrorQuery, errors.New(`sendQuery: [HTTP 500] failed to read response: read tcp 10.0.0.5:9440: connection reset by peer`))
		got := h.MutateQueryError(raw)
		assert.Equal(t, msgCannotConnect, got.Error())
		assertNoNetworkDetails(t, got.Error())
	})
}

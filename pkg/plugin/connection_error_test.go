package plugin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/hydrolix/plugin/pkg/plugin/exchange"
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

// clickhouse-go's error for a non-200 answer, under the handshake's plumbing.
func httpAnswer(status int, inner error) error {
	return fmt.Errorf("failed to query server hello: failed to query server hello info: sendQuery: %w",
		&clickhouse.HTTPError{StatusCode: status, Err: inner})
}

// The same, under a query's plumbing as sqlds hands it to MutateQueryError.
func httpQueryAnswer(status int, inner error) error {
	return fmt.Errorf("%w: %w", sqlds.ErrorQuery, fmt.Errorf("sendQuery: %w", &clickhouse.HTTPError{StatusCode: status, Err: inner}))
}

// clickhouse-go's inner error for a body that is not a ClickHouse exception.
func bodyOf(body string) error {
	return fmt.Errorf("response body: %q", body)
}

// clickhouse-go's inner error when X-ClickHouse-Exception-Code was set but the
// body could not be read: an exception whose message is the read error.
var undeliveredException = &clickhouse.Exception{
	Code:    516,
	Message: "failed to read response body: read tcp 10.0.0.5:9440: connection reset by peer",
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
		"resolver timed out": {
			err:  dialError(&net.DNSError{Err: "i/o timeout", Name: "internal.example", Server: "127.0.0.11:53", IsTimeout: true}),
			want: msgConnectTimeout,
		},
		"resolver timed out, as text": {err: errors.New("dial tcp: lookup internal.example on 127.0.0.11:53: read udp 127.0.0.1:5353->127.0.0.11:53: i/o timeout"), want: msgConnectTimeout},
		"refused":                     {err: dialError(syscall.ECONNREFUSED), want: msgRefused},
		"refused over http":           {err: inURL(dialError(syscall.ECONNREFUSED)), want: msgRefused},
		"dial timeout":                {err: dialError(timeoutError{}), want: msgConnectTimeout},
		"deadline while dialing":      {err: dialError(context.DeadlineExceeded), want: msgConnectTimeout},
		"deadline on a request":       {err: inURL(context.DeadlineExceeded), want: msgRequestTimeout},
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

		// clickhouse-go's typed errors: the status decides, not the exception
		// clickhouse-go parsed out of the body.
		"wrong password, typed http": {
			err:  httpAnswer(http.StatusForbidden, &clickhouse.Exception{Code: 516, Name: "DB::Exception", Message: "alice: Authentication failed"}),
			want: msgAuthFailed,
		},
		"unauthorized, typed": {err: httpAnswer(http.StatusUnauthorized, bodyOf("")), want: msgAuthFailed},
		"some other service's body, typed": {
			err:  httpAnswer(http.StatusNotFound, bodyOf(`{"message": "internal.example says no such host"}`)),
			want: "The server answered HTTP 404",
		},
		"a typed body shaped like an exception": {
			err:  httpAnswer(http.StatusInternalServerError, &clickhouse.Exception{Code: 516, Name: "DB::Exception", Message: "internal.example"}),
			want: "The server answered HTTP 500",
		},
		"a syntax error at connect, typed": {
			err:  httpAnswer(http.StatusBadRequest, &clickhouse.Exception{Code: 62, Name: "DB::Exception", Message: "Syntax error"}),
			want: "The server answered HTTP 400",
		},
		"an exception announced but not delivered": {
			err:  httpAnswer(http.StatusInternalServerError, undeliveredException),
			want: "The server answered HTTP 500",
		},
		"a body that could not be read, typed": {
			err: httpAnswer(http.StatusInternalServerError, fmt.Errorf("failed to read response: %w",
				&net.OpError{Op: "read", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 9440}, Err: syscall.ECONNRESET})),
			want: "The server answered HTTP 500",
		},
		// What the health check sees of the same: the typed error's text.
		"a typed body shaped like an exception, as text": {
			err:  errors.New(httpAnswer(http.StatusInternalServerError, &clickhouse.Exception{Code: 516, Message: "internal.example"}).Error()),
			want: "The server answered HTTP 500",
		},
		"an exception announced but not delivered, as text": {
			err:  errors.New(httpAnswer(http.StatusInternalServerError, undeliveredException).Error()),
			want: "The server answered HTTP 500",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := connectFailure(c.err)
			assert.Equal(t, c.want, got.Error())
			assertNoNetworkDetails(t, got.Error())
			assert.ErrorIs(t, got, c.err, "the cause stays underneath")
		})
	}

	// What sqlds's health check hands over: the ping's error, already a string.
	t.Run("a redirect that arrives as text keeps its message", func(t *testing.T) {
		redirect := &RedirectError{Status: http.StatusFound, Location: "http://internal.example/admin/"}
		raw := &url.Error{Op: "Post", URL: "http://internal.example:9440/?default_format=Native", Err: redirect}
		got := connectFailure(errors.New(raw.Error()))
		assert.Equal(t, redirect.Error(), got.Error())
		assertNoNetworkDetails(t, got.Error())
	})

	t.Run("a body that quotes the redirect message is still a body", func(t *testing.T) {
		got := connectFailure(errors.New(`sendQuery: [HTTP 500] response body: "the cluster answered HTTP 302, a redirect"`))
		assert.Equal(t, "The server answered HTTP 500", got.Error())
	})

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
	exceptionShaped := answering(http.StatusInternalServerError, "Code: 516. DB::Exception: internal service: Authentication failed (AUTHENTICATION_FAILED)")
	defer exceptionShaped.Close()
	syntaxError := answering(http.StatusBadRequest, "Code: 62. DB::Exception: internal service: Syntax error (SYNTAX_ERROR)")
	defer syntaxError.Close()
	// The header promises an exception; the body is cut short, so reading it fails.
	undelivered := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-ClickHouse-Exception-Code", "516")
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "Code: 516.")
	}))
	defer undelivered.Close()

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
		"a body shaped like an exception": {
			instanceSettingsFor(t, exceptionShaped.Listener.Addr().String(), false, "http"), "The server answered HTTP 500",
		},
		"a syntax error at connect": {
			instanceSettingsFor(t, syntaxError.Listener.Addr().String(), false, "http"), "The server answered HTTP 400",
		},
		"an exception announced but not delivered": {
			instanceSettingsFor(t, undelivered.Listener.Addr().String(), false, "http"), "The server answered HTTP 500",
		},
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

	t.Run("an exception announced but not delivered is the network's", func(t *testing.T) {
		got := h.MutateQueryError(httpQueryAnswer(http.StatusInternalServerError, undeliveredException))
		assert.Equal(t, msgCannotConnect, got.Error())
		assert.Equal(t, backend.ErrorSourceDownstream, got.ErrorSource())
		assertNoNetworkDetails(t, got.Error())
	})

	// clickhouse-go's other error for a body it could not read, which keeps the
	// read error's type: its kind is named where it has one.
	for name, c := range map[string]struct {
		read error
		want string
	}{
		"reset":     {&net.OpError{Op: "read", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 9440}, Err: syscall.ECONNRESET}, msgCannotConnect},
		"timed out": {&net.OpError{Op: "read", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 9440}, Err: timeoutError{}}, msgRequestTimeout},
		"cut short": {io.ErrUnexpectedEOF, msgCannotConnect},
	} {
		t.Run("a typed answer whose body "+name+" is the network's", func(t *testing.T) {
			got := h.MutateQueryError(httpQueryAnswer(http.StatusInternalServerError, fmt.Errorf("failed to read response: %w", c.read)))
			assert.Equal(t, c.want, got.Error())
			assert.ErrorIs(t, got, sqlds.ErrorQuery)
			assertNoNetworkDetails(t, got.Error())
		})
	}

	t.Run("a typed answer that repeats a query about connections passes through", func(t *testing.T) {
		got := h.MutateQueryError(httpQueryAnswer(http.StatusBadRequest, &clickhouse.Exception{
			Code: 62, Name: "DB::Exception", Message: "Syntax error: failed at position 7: SELEC 1 WHERE message ILIKE '%connection refused%'",
		}))
		assert.Contains(t, got.Error(), "Syntax error")
	})
}

func TestMutateQueryErrorShowsAnHTTPAnswerAsTheClusterWroteIt(t *testing.T) {
	h := NewHydrolix()

	t.Run("a clickhouse exception reads like a native one", func(t *testing.T) {
		got := h.MutateQueryError(httpQueryAnswer(http.StatusBadRequest, &clickhouse.Exception{
			Code: 62, Name: "DB::Exception", Message: "Syntax error: failed at position 1 ('SELEC')",
		}))
		assert.Contains(t, got.Error(), "[HTTP 400]")
		assert.Contains(t, got.Error(), "Code: 62. DB::Exception: Syntax error: failed at position 1 ('SELEC')")
		assert.Equal(t, backend.ErrorSourceDownstream, got.ErrorSource())
	})

	// Fixtures from src/errors/errorBeautifier.test.ts: the frontend finds the
	// JSON in the message and parses it, which an escaped body defeats.
	for name, body := range map[string]string{
		"a turbine authentication failure": `{"error": "<TurbineApiAuthenticatorError api login failed with provided username/password 'admin'. (Hydrolix)>"}`,
		"a body over several lines":        "<ChTurbineDatabaseError table unknown not found>\nsecond line with \"quotes\"",
	} {
		t.Run(name+" is not escaped", func(t *testing.T) {
			got := h.MutateQueryError(httpQueryAnswer(http.StatusUnauthorized, bodyOf(body)))
			assert.Contains(t, got.Error(), "[HTTP 401]")
			assert.Contains(t, got.Error(), body)
		})
	}

	// Turbine sends X-ClickHouse-Exception-Code with some JSON answers.
	// clickhouse-go then cuts the body at "Code: N." and parses the rest, so
	// the exception's message ends in the JSON's tail, still escaped.
	t.Run("a turbine body clickhouse-go cut into an exception is put back whole", func(t *testing.T) {
		got := h.MutateQueryError(httpQueryAnswer(http.StatusBadRequest, &clickhouse.Exception{
			Code: 47, Name: "DB::Exception", CodeName: "UNKNOWN_IDENTIFIER",
			Message: `Missing columns: 'a' while processing query: 'SELECT \"a\"\nFROM t', required columns: 'a'. (UNKNOWN_IDENTIFIER)",` + "\n" + `    "query": "SELECT \"a\"\nFROM t"` + "\n}",
		}))
		assert.Equal(t,
			`[HTTP 400] response body: {"error": "Code: 47. DB::Exception: Missing columns: 'a' while processing query: 'SELECT \"a\"\nFROM t', required columns: 'a'. (UNKNOWN_IDENTIFIER)",`+"\n"+`    "query": "SELECT \"a\"\nFROM t"`+"\n}",
			got.Error())
	})

	t.Run("an exception that only ends like JSON is left as it is", func(t *testing.T) {
		got := h.MutateQueryError(httpQueryAnswer(http.StatusBadRequest, &clickhouse.Exception{
			Code: 62, Name: "DB::Exception", Message: `Syntax error: failed at position 8 ('"a"}'): "a"}`,
		}))
		assert.Equal(t, `[HTTP 400] Code: 62. DB::Exception: Syntax error: failed at position 8 ('"a"}'): "a"}`, got.Error())
	})

	// Turbine's answer to a wrong password: the header gives the code and the
	// body has no "Code: N." to cut at, so the exception has no name.
	t.Run("an exception with no name is printed without one", func(t *testing.T) {
		body := `{"error": "internal authentication failure", "query": "SELECT 1"}`
		got := h.MutateQueryError(httpQueryAnswer(http.StatusBadRequest, &clickhouse.Exception{Code: 1001, Message: body}))
		assert.Equal(t, "[HTTP 400] Code: 1001. "+body, got.Error())
	})

	t.Run("a native exception with no name is printed without one", func(t *testing.T) {
		got := h.MutateQueryError(fmt.Errorf("%w: %w", sqlds.ErrorQuery, &clickhouse.Exception{Code: 1001, Message: "std::exception"}))
		assert.Equal(t, "Code: 1001. std::exception", got.Error())
	})

	t.Run("a body clickhouse-go did not quote is shown as it came", func(t *testing.T) {
		got := h.MutateQueryError(httpQueryAnswer(http.StatusBadGateway, errors.New("response body: upstream gave up")))
		assert.Equal(t, "[HTTP 502] response body: upstream gave up", got.Error())
	})

	// sqlds retries a query once when its mutated error is a query error whose
	// text holds the marker; exchange mode relies on that to replace a token
	// the cluster stopped accepting.
	for name, inner := range map[string]error{
		"with an exception": &clickhouse.Exception{Code: 516, Name: "DB::Exception", Message: "token expired"},
		"with a body":       bodyOf(`{"error": "token expired"}`),
	} {
		t.Run("a cluster 401 "+name+" is still retried", func(t *testing.T) {
			got := h.MutateQueryError(httpQueryAnswer(http.StatusUnauthorized, inner))
			assert.Contains(t, got.Error(), exchange.UnauthorizedMarker)
			assert.ErrorIs(t, got, sqlds.ErrorQuery)
			assert.Equal(t, backend.ErrorSourceDownstream, got.ErrorSource())
		})
	}
}

// The cases above build clickhouse-go's errors by hand. These get them from
// clickhouse-go, so that a release which words or shapes an answer differently
// fails here and not in a panel: the unreadable answers are known only by
// clickhouse-go's text, and a miss there prints a read error, address included.
func TestMutateQueryErrorOnErrorsFromClickhouseGo(t *testing.T) {
	turbineBody := `{"error": "Code: 47. DB::Exception: Missing columns: 'a' while processing query: 'SELECT \"a\"\nFROM numbers(1)', required columns: 'a'. (UNKNOWN_IDENTIFIER)",` + "\n" + `    "query": "SELECT \"a\"\nFROM numbers(1)"` + "\n}"
	answerWith := func(status int, code, body string) func(http.ResponseWriter) {
		return func(w http.ResponseWriter) {
			if code != "" {
				w.Header().Set("X-ClickHouse-Exception-Code", code)
			}
			w.WriteHeader(status)
			_, _ = fmt.Fprint(w, body)
		}
	}
	// More is promised than is sent, so reading the body fails.
	cutShort := func(code string) func(http.ResponseWriter) {
		return func(w http.ResponseWriter) {
			if code != "" {
				w.Header().Set("X-ClickHouse-Exception-Code", code)
			}
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, "Code: 516.")
		}
	}

	cases := map[string]struct {
		answer func(http.ResponseWriter)
		want   string
	}{
		"a clickhouse exception": {
			answerWith(http.StatusNotFound, "60", "Code: 60. DB::Exception: Unknown table expression identifier 'nope'. (UNKNOWN_TABLE) (version 24.8.1.1 (official build))\n"),
			"[HTTP 404] Code: 60. DB::Exception: Unknown table expression identifier 'nope'. (UNKNOWN_TABLE) (version 24.8.1.1 (official build))",
		},
		"a turbine body": {
			answerWith(http.StatusBadRequest, "", turbineBody),
			"[HTTP 400] response body: " + turbineBody,
		},
		"a turbine body announced by the header": {
			answerWith(http.StatusBadRequest, "47", turbineBody),
			"[HTTP 400] response body: " + turbineBody,
		},
		"a turbine body with a code only in the header": {
			answerWith(http.StatusBadRequest, "1001", `{"error": "internal authentication failure", "query": "SELECT 1"}`),
			`[HTTP 400] Code: 1001. {"error": "internal authentication failure", "query": "SELECT 1"}`,
		},
		"a body cut short":                          {cutShort(""), msgCannotConnect},
		"a body cut short, announced by the header": {cutShort("516"), msgCannotConnect},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cluster := clusterAnswering(t, c.answer)
			h := NewHydrolix()
			db, err := h.Connect(context.Background(), instanceSettingsFor(t, cluster.Listener.Addr().String(), false, "http"), nil)
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			rows, err := db.QueryContext(context.Background(), "SELECT number FROM numbers(1)")
			if err == nil {
				for rows.Next() {
				}
				err = rows.Err()
				_ = rows.Close()
			}
			require.Error(t, err)

			got := h.MutateQueryError(fmt.Errorf("%w: %w", sqlds.ErrorQuery, err))
			assert.Equal(t, c.want, got.Error())
			assert.ErrorIs(t, got, sqlds.ErrorQuery)
		})
	}
}

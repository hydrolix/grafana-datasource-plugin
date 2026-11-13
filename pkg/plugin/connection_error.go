package plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// What a person reads when the plugin cannot reach, or cannot talk to,
// whatever answers at the configured address. One fixed sentence per kind of
// failure, and nothing from the error itself.
const (
	msgHostUnresolved = "Host name could not be resolved"
	msgRefused        = "Connection refused"
	msgConnectTimeout = "Connection timed out"
	msgRequestTimeout = "The request timed out"
	msgTLS            = "TLS handshake failed"
	msgAuthFailed     = "Authentication failed"
	msgCannotConnect  = "Could not connect to the server"
)

// connectionError replaces a transport or handshake error with a fixed
// message.
//
// The host is the operator's to set, and on a shared Grafana an org admin is
// not the one who owns the network the plugin runs in. A raw error names the
// address it dialled, the resolver it asked, the request URL, and for an HTTP
// answer the body: "dial tcp 10.0.0.5:9440: connect: connection refused"
// tells whoever pressed Save & test that something listens, or does not, at an
// address they could not otherwise reach. So the message says which kind of
// failure it was and no more; the original goes to the log.
//
// The cause stays underneath for errors.Is, which sqlds uses to tell a
// deadline from an error worth retrying.
type connectionError struct {
	message string
	cause   error
}

func (e *connectionError) Error() string { return e.message }
func (e *connectionError) Unwrap() error { return e.cause }

// ClickHouse's codes for a credential it did not accept: UNKNOWN_USER,
// WRONG_PASSWORD, REQUIRED_PASSWORD, AUTHENTICATION_FAILED.
var authFailureCodes = map[int32]bool{192: true, 193: true, 194: true, 516: true}

var (
	// The fork reports an HTTP failure as text, "[HTTP 403] response body: …".
	httpStatusText = regexp.MustCompile(`\[HTTP (\d{3})\]`)
	// The same, where the body was read: everything after it is the answer's.
	// The fork's other form, "[HTTP 500] failed to read response: …", ends in
	// a transport error and is not matched.
	httpBodyText = regexp.MustCompile(`\[HTTP \d{3}\] response body:`)
	// clickhouse.Exception's own text, for an error that arrives as a string.
	exceptionText = regexp.MustCompile(`code: (\d+), message:`)
)

// transportMessage names the kind of failure when err never got as far as an
// answer from the server: resolving, dialling, the TLS handshake, or the
// connection dropping under a request. It reports false for anything else,
// which is then somebody's answer rather than the network's.
//
// Each kind is recognised by type where the error still has one and by its
// text otherwise: sqlds hands the health check a message, not an error.
func transportMessage(err error) (string, bool) {
	text := err.Error()
	// An HTTP answer is text too, and its body can hold any of the phrases
	// below: a syntax error repeats the query, string literals included.
	if httpBodyText.MatchString(text) {
		return "", false
	}
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(text, p) {
				return true
			}
		}
		return false
	}

	// A resolver that did not answer in time is a timeout, below: the name may
	// be a good one.
	var dns *net.DNSError
	if (errors.As(err, &dns) && !dns.IsTimeout) || has("no such host") {
		return msgHostUnresolved, true
	}

	var certificate *tls.CertificateVerificationError
	var record tls.RecordHeaderError
	var alert tls.AlertError
	if errors.As(err, &certificate) || errors.As(err, &record) || errors.As(err, &alert) ||
		errors.Is(err, http.ErrSchemeMismatch) || has("tls: ", "x509: ", http.ErrSchemeMismatch.Error()) {
		return msgTLS, true
	}

	if errors.Is(err, syscall.ECONNREFUSED) || has("connection refused") {
		return msgRefused, true
	}

	var op *net.OpError
	isOp := errors.As(err, &op)
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) ||
		has("i/o timeout", context.DeadlineExceeded.Error()) {
		if (isOp && op.Op == "dial") || has("dial tcp") {
			return msgConnectTimeout, true
		}
		return msgRequestTimeout, true
	}

	// Whatever else the network did. Both types print the address or the URL.
	var request *url.Error
	if isOp || errors.As(err, &request) || has("dial tcp", "read tcp", "write tcp") {
		return msgCannotConnect, true
	}
	return "", false
}

// connectFailure is the error Connect returns when the handshake fails, and
// the message Save & test shows. Nothing of what answered is repeated: a
// status or a ClickHouse code says which kind of answer it was, and the body
// or the exception text, which whatever sits at that address wrote, does not
// travel.
func connectFailure(err error) error {
	var redirect *RedirectError
	if errors.As(err, &redirect) {
		return redirect
	}

	fixed := func(message string) error { return &connectionError{message: message, cause: err} }

	var ex *clickhouse.Exception
	if errors.As(err, &ex) {
		return fixed(exceptionMessage(ex.Code))
	}
	// Before anything else that reads the text: this text ends in a body
	// somebody else wrote, which can look like an exception or a transport
	// error.
	if m := httpStatusText.FindStringSubmatch(err.Error()); m != nil {
		switch m[1] {
		case "401", "403":
			return fixed(msgAuthFailed)
		}
		return fixed("The server answered HTTP " + m[1])
	}

	if m := exceptionText.FindStringSubmatch(err.Error()); m != nil {
		if code, convErr := strconv.ParseInt(m[1], 10, 32); convErr == nil {
			return fixed(exceptionMessage(int32(code)))
		}
	}

	// A redirect refused on a connection the instance already held reaches
	// the health check as text, under the request URL.
	if redirect, ok := redirectFromText(err.Error()); ok {
		return redirect
	}

	if message, ok := transportMessage(err); ok {
		// A deadline here is the connect timeout, whichever step it ran out on.
		if message == msgRequestTimeout {
			message = msgConnectTimeout
		}
		return fixed(message)
	}

	return fixed(msgCannotConnect)
}

func exceptionMessage(code int32) string {
	if authFailureCodes[code] {
		return msgAuthFailed
	}
	return fmt.Sprintf("The server refused the connection (ClickHouse error code %d)", code)
}

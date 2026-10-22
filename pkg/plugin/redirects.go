package plugin

import (
	"fmt"
	"io"
	"net/http"
)

// RedirectError is the answer to the cluster, or whatever answers at its
// address, redirecting a request. The plugin follows none.
//
// http.Client's rule for a redirect hop keeps Authorization on a hop to a
// subdomain of the host and on an https-to-http downgrade, and copies every
// other header, X-ClickHouse-Key among them, to any host at all. So a proxy
// in front of the cluster, or a mis-pointed datasource, could send the
// credential of any mode somewhere nobody configured. ClickHouse itself never
// redirects; a 3xx is the operator's problem and is reported as one, naming
// where it pointed.
type RedirectError struct {
	Status   int
	Location string
}

func (e *RedirectError) Error() string {
	if e.Location == "" {
		return fmt.Sprintf("the cluster answered HTTP %d, a redirect, which the plugin does not follow", e.Status)
	}
	return fmt.Sprintf("the cluster answered HTTP %d redirecting to %q, which the plugin does not follow: a redirect would carry the credential off the configured host", e.Status, e.Location)
}

// refuseRedirects wraps a transport so that a response http.Client would
// follow becomes a RedirectError instead. It sits at the transport, below
// http.Client's redirect logic, because the client that holds that logic is
// built inside clickhouse-go and exposes no CheckRedirect.
func refuseRedirects(base http.RoundTripper) http.RoundTripper {
	return &noRedirects{base: base}
}

type noRedirects struct {
	base http.RoundTripper
}

// http.Client finds CloseIdleConnections by a type assertion on its
// transport, so a wrapper without the method silently loses it, and the
// fork's httpConnect.close() has nothing else with which to release the
// socket. The assertion keeps the method from being dropped by the next edit.
var _ interface{ CloseIdleConnections() } = (*noRedirects)(nil)

func (n *noRedirects) CloseIdleConnections() {
	if c, ok := n.base.(interface{ CloseIdleConnections() }); ok {
		c.CloseIdleConnections()
	}
}

// RoundTrip breaks the http.RoundTripper contract on purpose: a response it
// obtained is returned as an error. The contract reserves errors for failing
// to obtain a response and leaves redirects to http.Client, but the client
// here is built inside clickhouse-go with no CheckRedirect, so the transport
// is the only place a hop can be refused before the client copies headers
// onto it. Do not "fix" this by passing the response through.
func (n *noRedirects) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := n.base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	switch resp.StatusCode {
	// The statuses http.Client acts on (net/http redirectBehavior). 300 and
	// 304 are not followed and pass through to clickhouse-go's own non-200
	// handling.
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		location := resp.Header.Get("Location")
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		return nil, &RedirectError{Status: resp.StatusCode, Location: location}
	}
	return resp, nil
}

// transportFor is the one TransportFunc every credentials mode installs.
// clickhouse-go builds its Transport with DisableCompression: true; the plugin
// keeps Go's transparent gzip on, as it always has. No redirect is followed,
// in any mode. wrap, when given, sits above that, so a watch sees the
// cluster's own status codes and never a redirect hop.
func transportFor(wrap func(http.RoundTripper) http.RoundTripper) func(*http.Transport) (http.RoundTripper, error) {
	return func(t *http.Transport) (http.RoundTripper, error) {
		t.DisableCompression = false
		rt := refuseRedirects(t)
		if wrap != nil {
			rt = wrap(rt)
		}
		return rt, nil
	}
}

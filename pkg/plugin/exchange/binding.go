package exchange

import (
	"context"
	"fmt"
	"net/http"
)

// UnauthorizedMarker is the text the clickhouse-go fork leaves in an error for a
// cluster's 401: `[HTTP 401] response body: "..."` (conn_http.go,
// executeRequest). sqlds decides whether to retry a query by substring of the
// error text, and this is the substring the driver lists.
const UnauthorizedMarker = "[HTTP 401]"

// Binding is one connection's supply of cluster tokens: where they come from,
// and who the connection belongs to.
//
// The driver asks it for a token on every outbound request through
// clickhouse-go's `Options.GetJWT` hook, so the credential is read at request
// time rather than frozen into the connection. It has to be: the connection
// cache holds a *sql.DB for up to an hour while a cluster token lives minutes,
// so a credential built when the connection was opened would be dead for most
// of that connection's life. Supplying it per request makes a token refresh
// invisible to pooling — and mirrors the console's browser client, which
// attaches a token from its session manager to each request.
//
// Going through the fork's hook rather than a RoundTripper of our own matters
// for redirects. The hook puts the header on the initial request, which is
// where `http.Client` decides whether an Authorization header may follow a
// redirect to another host. A RoundTripper that attaches the token itself sees
// every hop, the redirected ones included, after that decision was made, and
// would send the token wherever the Location header pointed.
type Binding struct {
	Source *Source
	// Bound is the connection's own (audience, subject), known because the
	// subject keys the connection cache. It lets a request that arrives without
	// a context principal — the driver's own handshake, which runs when the
	// connection is established — still be identified.
	Bound Principal
	// Principals holds the newest forwarded token per subject, for that case.
	Principals *Principals
}

// Bind describes a connection belonging to one person at one cluster. An empty
// subject binds nobody: such a connection answers only requests whose context
// carries a principal.
func Bind(src *Source, principals *Principals, audience, subject string) *Binding {
	return &Binding{
		Source:     src,
		Principals: principals,
		Bound:      Principal{Audience: audience, Subject: subject},
	}
}

// principalFor answers who a request runs as. The context is tried first
// because it is exact — it carries the token of the very request being served.
// The connection's bound identity is the fallback, and the registry supplies
// that person's newest token.
func (b *Binding) principalFor(ctx context.Context) (Principal, bool) {
	if p, ok := PrincipalFrom(ctx); ok {
		return p, true
	}
	if b.Principals == nil || b.Bound.Subject == "" || b.Bound.Audience == "" {
		return Principal{}, false
	}
	return b.Principals.Lookup(b.Bound.Audience, b.Bound.Subject)
}

// JWT is the fork's GetJWT hook: the cluster token for the request this context
// belongs to.
//
// A request with no identity is refused rather than sent with whatever the
// connection was built with — a query with no signed-in user has no business
// reaching a cluster on this lane. And a failed exchange is a failed request,
// never a fallback to the forwarded token: sending the console-realm token to
// the cluster is the one thing this whole lane exists to avoid.
func (b *Binding) JWT(ctx context.Context) (string, error) {
	p, ok := b.principalFor(ctx)
	if !ok {
		return "", fmt.Errorf("%w: this request carries no signed-in user", ErrRefused)
	}
	return b.Source.Token(ctx, p.Audience, p.Subject, p.SubjectToken)
}

// Watch wraps the connection's round tripper so that a cluster's 401 drops the
// token the cluster refused. It attaches nothing and changes nothing: the
// request goes out as the fork built it, and the response comes back as the
// cluster sent it.
//
// The cluster is the authority on whether a token is still good, whatever the
// cache believes. The retry itself is sqlds's: the driver lists the 401 as
// retryable (see `Settings` in driver.go), so sqlds reconnects and runs the
// query once more, and the next JWT call mints afresh because the entry is
// gone. A second 401 is the answer.
func (b *Binding) Watch(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &unauthorizedWatch{base: base, binding: b}
}

type unauthorizedWatch struct {
	base    http.RoundTripper
	binding *Binding
}

func (w *unauthorizedWatch) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := w.base.RoundTrip(req)
	if err == nil && resp != nil && resp.StatusCode == http.StatusUnauthorized {
		if p, ok := w.binding.principalFor(req.Context()); ok {
			w.binding.Source.Invalidate(p.Audience, p.Subject)
		}
	}
	return resp, err
}

// Package identity carries the signed-in user's forwarded token on the
// context.
//
// It is its own package because both ends need it and they cannot import each
// other: `pkg/plugin` already imports `pkg/api` to install the resource
// routes, so the route handler cannot reach back into `pkg/plugin`.
package identity

import (
	"context"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// The forwarded identity travels on the CONTEXT, because the two places that
// need it downstream cannot be reached any other way.
//
// It used to travel on `models.HdxQuery.Headers`, which does not work and was
// the defect behind CFB-2612's cache fix:
//
//   - the `/interpolate` route sets that field and then marshals the query,
//     and the field is tagged `json:"-"`, so it is dropped before the
//     interpolator ever unmarshals it;
//   - the QueryData path never fills it at all, because sqlds hands the
//     interpolator `(*sqlutil.Query, req.JSON)` and neither carries headers.
//
// So `query.Headers` is nil in production at every macro call site, the
// metadata cache scope was always empty, and every user shared one entry —
// exactly the disclosure the fix was meant to close. `interpolator.go` has
// said for some time that the macros "pull headers from context"; this is the
// code that makes that true.
//
// `ForwardHeaders: true` is NOT the alternative. It writes the whole HTTP
// header map into `ConnectionArgs`, which is the connection pool's cache key,
// so per-request header noise would fragment the pool (`driver.go`'s own
// note). Dropping the `json:"-"` tag is not either: it would serialise a
// bearer token into query JSON, and it would fix only the route path.
type forwardedTokenKey struct{}

// WithForwardedToken returns a context carrying the signed-in user's
// forwarded token. An empty token returns the context unchanged, so a mode
// that forwards no identity cannot be told apart from one that was never
// asked — both read as absent downstream.
func WithForwardedToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, forwardedTokenKey{}, token)
}

// ForwardedTokenFrom answers the signed-in user's forwarded token, and whether
// there was one.
func ForwardedTokenFrom(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(forwardedTokenKey{}).(string)
	return token, ok && token != ""
}

// forwardedTokenOf reads the bearer token out of a header set, bare of its
// scheme. The one place that knows how the token is spelled on the wire.
func TokenOf(headers http.Header) string {
	if headers == nil {
		return ""
	}
	return strings.TrimPrefix(headers.Get(backend.OAuthIdentityTokenHeaderName), "Bearer ")
}

// ForwardedTokenOfRequest reads the forwarded token from an inbound HTTP
// request — the `/interpolate` resource call's own, where the identity enters
// the plugin for the editor's preview.
func TokenOfRequest(req *http.Request) string {
	if req == nil {
		return ""
	}
	return TokenOf(req.Header)
}

package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/plugin/pkg/identity"
	"github.com/hydrolix/plugin/pkg/plugin/exchange"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These drive the REAL entry points. The first version of the per-user cache
// fix passed its tests and did nothing in production, because those tests
// called GetKeys/GetPK directly and handed them a header set that no
// production path ever supplies (CFB-2612). A test that cannot fail when the
// identity is dropped on the way in is not testing the fix.

func bearer(sub string) string {
	return "Bearer " + jwtWithSubject(sub)
}

// MutateQueryData is where a panel's query enters. If the identity does not
// leave it on the context, every downstream lookup is anonymous and the cache
// is shared.
func TestEntryPoint_MutateQueryDataPutsTheIdentityOnTheContext(t *testing.T) {
	req := makeQueryDataReq(t, "forwardOAuth", nil, `{"rawSql":"SELECT 1"}`)
	req.SetHTTPHeader(backend.OAuthIdentityTokenHeaderName, bearer("kc-sub-alice"))

	ctx, _ := NewHydrolix().MutateQueryData(context.Background(), req)

	token, ok := identity.ForwardedTokenFrom(ctx)
	require.True(t, ok, "without this the metadata cache scope is empty for everyone")
	assert.Equal(t, "kc-sub-alice", exchange.SubjectOf(token))
}

// The cache scope is what the two users actually get. Derived from the context
// the entry point produced, not from a header set a test invented.
func TestEntryPoint_TwoUsersGetDifferentCacheScopes(t *testing.T) {
	scopeFor := func(sub string) string {
		req := makeQueryDataReq(t, "forwardOAuth", nil, `{"rawSql":"SELECT 1"}`)
		req.SetHTTPHeader(backend.OAuthIdentityTokenHeaderName, bearer(sub))
		ctx, _ := NewHydrolix().MutateQueryData(context.Background(), req)
		return cacheScope(ctx, nil)
	}

	alice, bob := scopeFor("kc-sub-alice"), scopeFor("kc-sub-bob")

	assert.NotEmpty(t, alice, "an empty scope is the bug: it is the bare key, shared by everyone")
	assert.NotEqual(t, alice, bob, "two people must not share a metadata cache entry")
	assert.NotEqual(t, scopedKey(alice, "db_t"), scopedKey(bob, "db_t"))
}

// A mode that forwards no identity has one credential and therefore one
// legitimate view. It must keep sharing an entry, or every query re-reads the
// schema for nothing.
func TestEntryPoint_AModeThatForwardsNothingStillSharesOneEntry(t *testing.T) {
	req := makeQueryDataReq(t, "serviceAccount", nil, `{"rawSql":"SELECT 1"}`)

	ctx, _ := NewHydrolix().MutateQueryData(context.Background(), req)

	assert.Empty(t, cacheScope(ctx, nil))
}

// The /interpolate route is the editor's entry point. It sets the identity on
// an HdxQuery field tagged `json:"-"` and then marshals it — which is where
// the identity used to disappear. The context is what survives.
func TestEntryPoint_TheInterpolateRouteCarriesTheIdentityOnTheContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/interpolate", nil)
	req.Header.Set(backend.OAuthIdentityTokenHeaderName, bearer("kc-sub-carol"))

	ctx := identity.WithForwardedToken(req.Context(), identity.TokenOfRequest(req))

	assert.NotEmpty(t, cacheScope(ctx, nil),
		"the editor's lookups are scoped to the person previewing the query")
	token, ok := identity.ForwardedTokenFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, "kc-sub-carol", exchange.SubjectOf(token))
}

// A request with no forwarded identity must not inherit somebody else's from
// a context that happens to be lying around.
func TestEntryPoint_NoIdentityMeansNoScope(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/interpolate", nil)

	ctx := identity.WithForwardedToken(req.Context(), identity.TokenOfRequest(req))

	_, ok := identity.ForwardedTokenFrom(ctx)
	assert.False(t, ok)
	assert.Empty(t, cacheScope(ctx, nil))
}

// The exchanging mode's half of the same fix (CFB-2612 / CFB-2553).
//
// A macro's metadata lookup is built by MetadataProvider.executeQuery, which
// gets no headers of its own — sqlds hands the interpolator none. Before this
// fix that inner request reached the cluster carrying nothing, so on a cache
// MISS the exchanging mode refused it for having no signed-in user and
// forwardOAuth answered "missing OAuth token in connection args".
//
// Now the context carries the identity the OUTER request arrived with, and
// executeQuery puts it back on the inner one. What that buys, and what this
// test holds, is that the inner request is mutated exactly as a panel's query
// is: the pool keys on the subject and the principal rides the context, so the
// lookup runs as the person who triggered it.
func TestEntryPoint_AColdMetadataLookupRunsAsTheSignedInUser(t *testing.T) {
	token := jwtWithSubject("kc-sub-dave")

	// The outer request, as a panel's arrives.
	outer := makeQueryDataReq(t, exchange.CredentialsType, nil, `{"rawSql":"SELECT 1"}`)
	outer.SetHTTPHeader(backend.OAuthIdentityTokenHeaderName, "Bearer "+token)
	h := NewHydrolix()
	ctx, _ := h.MutateQueryData(context.Background(), outer)

	// The inner request, as MetadataProvider.executeQuery builds it: no
	// headers, because the macro had none to give it.
	inner := makeQueryDataReq(t, exchange.CredentialsType, nil, `{"rawSql":"DESCRIBE TABLE x"}`)
	require.Empty(t, inner.GetHTTPHeader(backend.OAuthIdentityTokenHeaderName),
		"premise: a macro's lookup starts with no identity of its own")
	if v, ok := identity.ForwardedTokenFrom(ctx); ok {
		inner.SetHTTPHeader(backend.OAuthIdentityTokenHeaderName, "Bearer "+v)
	}

	innerCtx, mutated := h.MutateQueryData(ctx, inner)

	assert.Equal(t, "kc-sub-dave", connArgsOf(t, mutated)["sub"],
		"the lookup keys the pool on the same person the panel does")
	p, ok := exchange.PrincipalFrom(innerCtx)
	require.True(t, ok, "without a principal the transport refuses the lookup")
	assert.Equal(t, "kc-sub-dave", p.Subject)
	assert.Equal(t, token, p.SubjectToken)
}

// The two carriers coexist: pkg/identity holds the raw forwarded token for the
// cache scope and for rebuilding an inner request, exchange.Principal holds
// the audience/subject/token the transport mints with. Neither shadows the
// other.
func TestEntryPoint_BothCarriersAreSetAndAgree(t *testing.T) {
	token := jwtWithSubject("kc-sub-erin")
	req := makeQueryDataReq(t, exchange.CredentialsType, nil, `{"rawSql":"SELECT 1"}`)
	req.SetHTTPHeader(backend.OAuthIdentityTokenHeaderName, "Bearer "+token)

	ctx, _ := NewHydrolix().MutateQueryData(context.Background(), req)

	carried, ok := identity.ForwardedTokenFrom(ctx)
	require.True(t, ok)
	p, ok := exchange.PrincipalFrom(ctx)
	require.True(t, ok)
	assert.Equal(t, carried, p.SubjectToken, "one identity, two carriers, no drift")
	assert.Equal(t, exchange.SubjectOf(carried), p.Subject)
}

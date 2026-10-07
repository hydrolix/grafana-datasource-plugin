package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/plugin/pkg/identity"
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
	assert.Equal(t, "kc-sub-alice", subjectOf(token))
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
	assert.Equal(t, "kc-sub-carol", subjectOf(token))
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

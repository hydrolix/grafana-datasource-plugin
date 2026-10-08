package plugin

import (
	"context"
	"github.com/hydrolix/plugin/pkg/plugin/exchange"
	"net/http"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jwtWithSubject builds an unsigned JWT carrying one claim. Unsigned is what
// this code path sees in a test, and the signature is not what it reads.

func forwardedHeaders(subject string) http.Header {
	h := http.Header{}
	h.Set(backend.OAuthIdentityTokenHeaderName, "Bearer "+jwtWithSubject(subject))
	return h
}

// answeringDS answers whichever schema query it is given, keyed on the refID
// the provider used, and counts calls so a test can see whether the cluster was
// asked at all.
func answeringDS(frame func() *data.Frame) *fakeMetadataDS {
	return &fakeMetadataDS{
		defaultDB: "mydb",
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frame(), req.Queries[0].RefID), nil
		},
	}
}

func describingDS() *fakeMetadataDS {
	return answeringDS(func() *data.Frame { return frameOf([]string{"ts"}, []string{"DateTime"}) })
}

func pkDS() *fakeMetadataDS {
	return answeringDS(func() *data.Frame { return frameOf([]string{"ts"}) })
}

func TestTwoUsersDoNotShareACachedSchema(t *testing.T) {
	// The defect this closes: in a forwarding mode the user's token reaches the
	// cluster on a cache MISS only, so one user's lookup would otherwise serve
	// every other user of the datasource that table's shape, unauthorized.
	ds := describingDS()
	p := NewMetadataProvider(ds)

	_, err := p.GetKeys(context.Background(), forwardedHeaders("user-alice"), "hydro.logs")
	require.NoError(t, err)
	require.Equal(t, 1, ds.callCount, "the first user's lookup reaches the cluster")

	_, err = p.GetKeys(context.Background(), forwardedHeaders("user-bob"), "hydro.logs")
	require.NoError(t, err)
	assert.Equal(t, 2, ds.callCount,
		"a second user must be authorized by the cluster rather than served from the first user's entry")
}

func TestTheSameUserIsStillServedFromTheCache(t *testing.T) {
	// The scope is the subject, not the token, so the cache still works — and a
	// token refresh mid-session does not throw it away.
	ds := describingDS()
	p := NewMetadataProvider(ds)

	for i := 0; i < 4; i++ {
		_, err := p.GetKeys(context.Background(), forwardedHeaders("user-alice"), "hydro.logs")
		require.NoError(t, err)
	}
	assert.Equal(t, 1, ds.callCount, "repeat lookups for one person hit the cache")
}

func TestAPrimaryKeyLookupIsScopedToo(t *testing.T) {
	ds := pkDS()
	p := NewMetadataProvider(ds)

	_, err := p.GetPK(context.Background(), forwardedHeaders("user-alice"), "mydb", "events")
	require.NoError(t, err)
	_, err = p.GetPK(context.Background(), forwardedHeaders("user-bob"), "mydb", "events")
	require.NoError(t, err)

	assert.Equal(t, 2, ds.callCount, "a primary key is schema too")
}

func TestModesThatForwardNoIdentityShareOneEntry(t *testing.T) {
	// A service account or a stored user account is one credential and therefore
	// one legitimate view: sharing is correct, and the key is the one the cache
	// has always used.
	ds := describingDS()
	p := NewMetadataProvider(ds)

	for i := 0; i < 3; i++ {
		_, err := p.GetKeys(context.Background(), http.Header{}, "hydro.logs")
		require.NoError(t, err)
	}
	assert.Equal(t, 1, ds.callCount)
	assert.Equal(t, "hydro.logs", scopedKey("", "hydro.logs"),
		"no forwarded identity keeps the original key")
}

func TestAnUnreadableForwardedTokenIsStillItsOwnIdentity(t *testing.T) {
	// An opaque token yields no subject. Keying on a digest of it re-caches on
	// every refresh, which is wasteful — and the right way to be wrong, because
	// the alternative shares one entry between different people.
	ds := describingDS()
	p := NewMetadataProvider(ds)

	opaque := func(v string) http.Header {
		h := http.Header{}
		h.Set(backend.OAuthIdentityTokenHeaderName, "Bearer "+v)
		return h
	}
	_, err := p.GetKeys(context.Background(), opaque("opaque-one"), "hydro.logs")
	require.NoError(t, err)
	_, err = p.GetKeys(context.Background(), opaque("opaque-two"), "hydro.logs")
	require.NoError(t, err)
	assert.Equal(t, 2, ds.callCount)

	_, err = p.GetKeys(context.Background(), opaque("opaque-one"), "hydro.logs")
	require.NoError(t, err)
	assert.Equal(t, 2, ds.callCount, "the same opaque token is still cached")
}

func TestSubjectOfReadsTheClaimWithoutVerifying(t *testing.T) {
	assert.Equal(t, "kc-sub-alice", exchange.SubjectOf(jwtWithSubject("kc-sub-alice")))
	for _, bad := range []string{"", "not-a-jwt", "a.b", "a.!!!.c"} {
		assert.Equal(t, "", exchange.SubjectOf(bad), bad)
	}
}

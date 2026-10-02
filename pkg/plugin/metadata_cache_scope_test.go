package plugin

import (
	"context"
	"net/http"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forwardedHeaders carries a signed-in user's token the way Grafana does.
func forwardedHeaders(subject string) http.Header {
	h := http.Header{}
	h.Set(backend.OAuthIdentityTokenHeaderName, "Bearer "+jwtWithSubject(subject))
	return h
}

// answeringDS answers whichever schema query it is given, keyed on the refID the
// provider used, and counts calls so a test can see whether the cluster was
// asked at all.
func answeringDS(frame func() *data.Frame) *fakeMetadataDS {
	return &fakeMetadataDS{
		defaultDB: "mydb",
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frame(), req.Queries[0].RefID), nil
		},
	}
}

// describingDS answers a DESCRIBE: one column and its type.
func describingDS() *fakeMetadataDS {
	return answeringDS(func() *data.Frame { return frameOf([]string{"ts"}, []string{"DateTime"}) })
}

// pkDS answers a primary-key lookup.
func pkDS() *fakeMetadataDS {
	return answeringDS(func() *data.Frame { return frameOf([]string{"ts"}) })
}

func TestTwoUsersDoNotShareACachedSchema(t *testing.T) {
	// The defect this closes: the forwarded token reaches the cluster on a cache
	// MISS only, so one user's lookup would otherwise serve every other user of
	// the datasource that table's shape for an hour, unauthorized.
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

	// And the same opaque token is still cached.
	_, err = p.GetKeys(context.Background(), opaque("opaque-one"), "hydro.logs")
	require.NoError(t, err)
	assert.Equal(t, 2, ds.callCount)
}

func TestNoCacheKeyWithASubjectReachesALogLine(t *testing.T) {
	// The keys now carry a subject, so the cache log lines name the table rather
	// than the key.
	scope := cacheScope(forwardedHeaders("user-alice"))
	require.Equal(t, "user-alice", scope)
	assert.NotEqual(t, scopedKey(scope, "hydro.logs"), "hydro.logs",
		"a forwarded identity must change the key")
	assert.Equal(t, "hydro.logs", scopedKey("", "hydro.logs"),
		"no forwarded identity keeps the original key")
}

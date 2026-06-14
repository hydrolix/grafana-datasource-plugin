package plugin

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMetadataDS is a minimal metadataDS test double. queryDataFn captures
// the request the provider built (so tests can assert header propagation)
// and returns canned frames.
type fakeMetadataDS struct {
	queryDataFn  func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error)
	instanceSet  backend.DataSourceInstanceSettings
	defaultDB    string
	callCount    int
	lastRequest  *backend.QueryDataRequest
}

func (f *fakeMetadataDS) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	f.callCount++
	f.lastRequest = req
	if f.queryDataFn == nil {
		return &backend.QueryDataResponse{Responses: map[string]backend.DataResponse{}}, nil
	}
	return f.queryDataFn(ctx, req)
}

func (f *fakeMetadataDS) InstanceSettings() backend.DataSourceInstanceSettings { return f.instanceSet }
func (f *fakeMetadataDS) DefaultDatabase() string                              { return f.defaultDB }

// nopMetadataDS panics if QueryData is invoked. Use it in tests that don't
// exercise the schema-query path.
type nopMetadataDS struct{}

func (nopMetadataDS) QueryData(context.Context, *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	panic("nopMetadataDS.QueryData called — test should not reach the schema-query path")
}
func (nopMetadataDS) InstanceSettings() backend.DataSourceInstanceSettings {
	return backend.DataSourceInstanceSettings{}
}
func (nopMetadataDS) DefaultDatabase() string { return "" }

// frameOf builds a one-row frame with one field per provided column value.
// Used to mock DESCRIBE / SELECT primary_key responses.
func frameOf(cols ...[]string) *data.Frame {
	frame := &data.Frame{}
	for i, col := range cols {
		values := make([]*string, len(col))
		for j := range col {
			v := col[j]
			values[j] = &v
		}
		frame.Fields = append(frame.Fields, data.NewField(string(rune('a'+i)), nil, values))
	}
	return frame
}

func respondWith(frame *data.Frame, refID string) *backend.QueryDataResponse {
	return &backend.QueryDataResponse{
		Responses: map[string]backend.DataResponse{
			refID: {Frames: data.Frames{frame}},
		},
	}
}

func TestGetStringSafe(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		s, err := GetStringSafe("hello")
		assert.NoError(t, err)
		assert.Equal(t, "hello", s)
	})
	t.Run("*string", func(t *testing.T) {
		v := "world"
		s, err := GetStringSafe(&v)
		assert.NoError(t, err)
		assert.Equal(t, "world", s)
	})
	t.Run("nil *string", func(t *testing.T) {
		var p *string
		s, err := GetStringSafe(p)
		assert.NoError(t, err)
		assert.Equal(t, "", s)
	})
	t.Run("unsupported type", func(t *testing.T) {
		_, err := GetStringSafe(42)
		assert.Error(t, err)
	})
}

func TestMetadataProvider_GetPK_CacheHit(t *testing.T) {
	ds := &fakeMetadataDS{}
	p := NewMetadataProvider(ds)
	p.pkCache.Set("db_tbl", "preseeded", ttlcache.DefaultTTL)

	pk, err := p.GetPK(context.Background(), nil, "db", "tbl")
	require.NoError(t, err)
	assert.Equal(t, "preseeded", pk)
	assert.Equal(t, 0, ds.callCount, "QueryData must not be called on cache hit")
}

func TestMetadataProvider_GetPK_CacheMissThenHit(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frameOf([]string{"id"}), "pk_query"), nil
		},
	}
	p := NewMetadataProvider(ds)

	pk, err := p.GetPK(context.Background(), nil, "db", "tbl")
	require.NoError(t, err)
	assert.Equal(t, "id", pk)
	assert.Equal(t, 1, ds.callCount)

	// Second call must hit the cache.
	pk, err = p.GetPK(context.Background(), nil, "db", "tbl")
	require.NoError(t, err)
	assert.Equal(t, "id", pk)
	assert.Equal(t, 1, ds.callCount, "second call must not re-query")
}

func TestMetadataProvider_GetPK_EmptyDatabaseUsesDefault(t *testing.T) {
	ds := &fakeMetadataDS{
		defaultDB: "events",
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frameOf([]string{"id"}), "pk_query"), nil
		},
	}
	p := NewMetadataProvider(ds)

	pk, err := p.GetPK(context.Background(), nil, "", "tbl")
	require.NoError(t, err)
	assert.Equal(t, "id", pk)

	// Cached under the resolved default database name.
	entry := p.pkCache.Get("events_tbl")
	require.NotNil(t, entry)
	assert.Equal(t, "id", entry.Value())
}

func TestMetadataProvider_GetPK_DefaultDatabaseMissingIsError(t *testing.T) {
	ds := &fakeMetadataDS{defaultDB: ""}
	p := NewMetadataProvider(ds)

	_, err := p.GetPK(context.Background(), nil, "", "tbl")
	assert.Error(t, err)
}

func TestMetadataProvider_QueryPK_EmptyFrameReturnsNotFound(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(&data.Frame{}, "pk_query"), nil
		},
	}
	p := NewMetadataProvider(ds)

	_, err := p.QueryPK(context.Background(), nil, "db", "tbl")
	assert.ErrorIs(t, err, ErrPrimaryKeyNotFound)
}

func TestMetadataProvider_GetKeys_CacheMissThenHit(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(
				frameOf(
					[]string{"col1", "col2"},
					[]string{"String", "UInt64"},
				),
				"key_query",
			), nil
		},
	}
	p := NewMetadataProvider(ds)

	keys, err := p.GetKeys(context.Background(), nil, "foo")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"col1": "String", "col2": "UInt64"}, keys)
	assert.Equal(t, 1, ds.callCount)

	// Second call must hit cache.
	keys, err = p.GetKeys(context.Background(), nil, "foo")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"col1": "String", "col2": "UInt64"}, keys)
	assert.Equal(t, 1, ds.callCount)
}

func TestMetadataProvider_QueryKeys_InsufficientFieldsErrors(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			// Only one field — provider needs name + type.
			return respondWith(frameOf([]string{"col1"}), "key_query"), nil
		},
	}
	p := NewMetadataProvider(ds)

	_, err := p.QueryKeys(context.Background(), nil, "foo")
	assert.ErrorIs(t, err, ErrAdHocKeysNotFound)
}

func TestMetadataProvider_ExecuteQuery_PropagatesHeadersViaSetHTTPHeader(t *testing.T) {
	// Asserts the SetHTTPHeader path: X-Grafana-Org-Id and Authorization
	// both round-trip through the synthetic request.
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return respondWith(frameOf([]string{"id"}), "pk_query"), nil
		},
	}
	p := NewMetadataProvider(ds)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer t")
	headers.Set(OrgIdHeaderKey, "5")

	_, err := p.QueryPK(context.Background(), headers, "db", "tbl")
	require.NoError(t, err)

	got := ds.lastRequest.GetHTTPHeaders()
	assert.Equal(t, "Bearer t", got.Get("Authorization"))
	assert.Equal(t, "5", got.Get(OrgIdHeaderKey), "X-Grafana-Org-Id must survive the SetHTTPHeader round-trip")
}

func TestMetadataProvider_QueryFailurePropagates(t *testing.T) {
	ds := &fakeMetadataDS{
		queryDataFn: func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			return nil, errors.New("upstream went away")
		},
	}
	p := NewMetadataProvider(ds)

	_, err := p.QueryPK(context.Background(), nil, "db", "tbl")
	assert.Error(t, err)
}

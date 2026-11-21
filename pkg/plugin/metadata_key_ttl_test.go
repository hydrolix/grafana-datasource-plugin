package plugin

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hydrolix/clickhouse-sql-parser/parser"
	"github.com/hydrolix/plugin/pkg/plugin/models"
	"github.com/jellydator/ttlcache/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Hydrolix table gains names while dashboards are open: a naming profile
// publishes `userAgent` as an alias of the existing column `UA`. The column-type
// map behind $__adHocFilter decides which filters apply, and a filter whose
// column the map does not list is dropped. So a stale map silently returns
// unfiltered rows for a filter on the new name.

// publishingTable is a DESCRIBE fake whose column list can grow mid-test.
type publishingTable struct {
	mu      sync.Mutex
	columns []string
}

func (t *publishingTable) publish(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.columns = append(t.columns, name)
}

func (t *publishingTable) ds() *fakeMetadataDS {
	return &fakeMetadataDS{
		queryDataFn: func(context.Context, *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			types := make([]string, len(t.columns))
			for i := range types {
				types[i] = "String"
			}
			return respondWith(frameOf(append([]string(nil), t.columns...), types), "key_query"), nil
		},
	}
}

// peekKeys reads the cached entry without touching it.
func peekKeys(p *MetadataProvider, key string) *ttlcache.Item[string, map[string]string] {
	return p.keyCache.Get(key, ttlcache.WithDisableTouchOnHit[string, map[string]string]())
}

func filterOnUserAgent(t *testing.T, p *MetadataProvider) string {
	t.Helper()
	query := &models.HdxQuery{
		Filters: []models.AdHocFilter{{Key: "userAgent", Operator: "=", Value: "WireReaderBot"}},
	}
	got, err := AdHocFilterMacro(context.Background(), query, []string{"hydro.logs"}, parser.Pos(0), p)
	require.NoError(t, err)
	return got
}

func TestAKeyMapIsReReadWithinAMinute(t *testing.T) {
	table := &publishingTable{columns: []string{"UA"}}
	p := NewMetadataProvider(table.ds())

	_, err := p.GetKeys(context.Background(), nil, "hydro.logs")
	require.NoError(t, err)

	item := peekKeys(p, "hydro.logs")
	require.NotNil(t, item)
	assert.Positive(t, item.TTL(), "a zero TTL never expires")
	assert.LessOrEqual(t, item.TTL(), time.Minute,
		"a newly published name must be filterable within a minute, without a datasource save")
}

func TestReadingAKeyMapDoesNotExtendIt(t *testing.T) {
	table := &publishingTable{columns: []string{"UA"}}
	ds := table.ds()
	p := NewMetadataProvider(ds)

	_, err := p.GetKeys(context.Background(), nil, "hydro.logs")
	require.NoError(t, err)
	expiresAt := peekKeys(p, "hydro.logs").ExpiresAt()

	time.Sleep(5 * time.Millisecond)
	_, err = p.GetKeys(context.Background(), nil, "hydro.logs")
	require.NoError(t, err)

	assert.Equal(t, 1, ds.callCount, "the second read is served from the cache")
	assert.Equal(t, expiresAt, peekKeys(p, "hydro.logs").ExpiresAt(),
		"a dashboard querying more often than the TTL must not keep a stale map alive")
}

func TestAFilterOnANewlyPublishedNameAppliesOnABusyDashboard(t *testing.T) {
	const ttl = 100 * time.Millisecond
	table := &publishingTable{columns: []string{"UA"}}
	p := newMetadataProvider(table.ds(), ttl)

	require.Equal(t, "1=1", filterOnUserAgent(t, p), "not published yet: the filter does not apply")

	table.publish("userAgent")

	// Panels keep querying with the filter set, faster than the TTL.
	deadline := time.Now().Add(5 * ttl)
	got := filterOnUserAgent(t, p)
	for got == "1=1" && time.Now().Before(deadline) {
		time.Sleep(ttl / 10)
		got = filterOnUserAgent(t, p)
	}
	assert.Contains(t, got, "userAgent", "the filter on the new name was still dropped after %s", 5*ttl)
}

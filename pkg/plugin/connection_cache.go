package plugin

import (
	"context"
	"time"

	"github.com/grafana/sqlds/v5"
	"github.com/jellydator/ttlcache/v3"
)

// TTLConnectionCache implements sqlds.ConnectionCache backed by
// jellydator/ttlcache/v3. Entries expire after the configured TTL and the
// underlying *sql.DB is closed on eviction. The bootstrap key
// (sqlds.defaultKey(uid) == "<uid>-default") is stored with ttlcache.NoTTL
// so the bootstrap entry survives for the cache's lifetime.
//
// sqlds.CachedConnection is a concrete value type (grafana/sqlds@v5.2.0): the
// cache stores and returns it by value, and Load returns the zero value on a
// miss. Its fields are unexported, so a stored entry's Close() cannot be
// instrumented from package plugin — closes are routed through the closeConn
// seam (default sqlds.CachedConnection.Close) so tests can observe them.
type TTLConnectionCache struct {
	inner        *ttlcache.Cache[string, sqlds.CachedConnection]
	bootstrapKey string
	// unsubscribe detaches the OnEviction callback. ttlcache returns it
	// from OnEviction; Dispose calls it so the explicit close loop is the
	// sole closer (otherwise DeleteAll would fire async OnEviction
	// callbacks that double-close).
	unsubscribe func()
	// closeConn closes an evicted/disposed entry. Defaults to
	// sqlds.CachedConnection.Close; tests override it to count invocations.
	closeConn func(sqlds.CachedConnection) error
}

var _ sqlds.ConnectionCache = (*TTLConnectionCache)(nil)

// NewTTLConnectionCache constructs a TTLConnectionCache scoped to a single
// datasource instance. uid SHOULD be the DataSourceInstanceSettings.UID; ttl
// is the per-entry default (the fork's choice is one hour). The cache's
// sweep goroutine is started before return.
func NewTTLConnectionCache(uid string, ttl time.Duration) sqlds.ConnectionCache {
	return newTTLConnectionCache(uid, ttl, sqlds.CachedConnection.Close)
}

// newTTLConnectionCache is the injectable constructor. closeConn is captured
// before the sweep goroutine starts (via the `go` statement's happens-before
// edge), so tests can supply a counting closer without racing the async
// OnEviction callback under `go test -race`.
func newTTLConnectionCache(uid string, ttl time.Duration, closeConn func(sqlds.CachedConnection) error) *TTLConnectionCache {
	cache := ttlcache.New[string, sqlds.CachedConnection](
		ttlcache.WithTTL[string, sqlds.CachedConnection](ttl),
	)
	c := &TTLConnectionCache{
		inner:        cache,
		bootstrapKey: uid + "-default",
		closeConn:    closeConn,
	}
	c.unsubscribe = cache.OnEviction(func(_ context.Context, _ ttlcache.EvictionReason, item *ttlcache.Item[string, sqlds.CachedConnection]) {
		_ = c.closeConn(item.Value())
	})
	go cache.Start()
	return c
}

func (c *TTLConnectionCache) Load(key string) (sqlds.CachedConnection, bool) {
	item := c.inner.Get(key)
	if item == nil {
		return sqlds.CachedConnection{}, false
	}
	return item.Value(), true
}

func (c *TTLConnectionCache) Store(key string, v sqlds.CachedConnection) {
	if key == c.bootstrapKey {
		c.inner.Set(key, v, ttlcache.NoTTL)
		return
	}
	c.inner.Set(key, v, ttlcache.DefaultTTL)
}

func (c *TTLConnectionCache) Range(f func(key string, v sqlds.CachedConnection) bool) {
	for key, item := range c.inner.Items() {
		if !f(key, item.Value()) {
			return
		}
	}
}

// Dispose halts the sweep goroutine, detaches the eviction callback (so the
// explicit close loop is the sole closer — no double-close from DeleteAll's
// async callbacks), closes every live entry synchronously, then clears the
// map. The synchronous close loop is the foreground-close contract sqlds
// expects: every connection is closed before Dispose returns.
func (c *TTLConnectionCache) Dispose() {
	c.inner.Stop()
	if c.unsubscribe != nil {
		c.unsubscribe()
		c.unsubscribe = nil
	}
	for _, item := range c.inner.Items() {
		_ = c.closeConn(item.Value())
	}
	c.inner.DeleteAll()
}

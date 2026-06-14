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
// The contract on Load requires that the returned CachedConnection be the
// exact reference passed to Store (sqlds-internal code type-asserts it back
// to a concrete struct). This implementation satisfies that contract by
// passing values straight through the underlying ttlcache.
type TTLConnectionCache struct {
	inner        *ttlcache.Cache[string, sqlds.CachedConnection]
	bootstrapKey string
	// unsubscribe detaches the OnEviction callback. ttlcache returns it
	// from OnEviction; Dispose calls it so the explicit close loop is the
	// sole closer (otherwise DeleteAll would fire async OnEviction
	// callbacks that double-close).
	unsubscribe func()
}

var _ sqlds.ConnectionCache = (*TTLConnectionCache)(nil)

// NewTTLConnectionCache constructs a TTLConnectionCache scoped to a single
// datasource instance. uid SHOULD be the DataSourceInstanceSettings.UID; ttl
// is the per-entry default (the fork's choice is one hour). The cache's
// sweep goroutine is started before return.
func NewTTLConnectionCache(uid string, ttl time.Duration) sqlds.ConnectionCache {
	cache := ttlcache.New[string, sqlds.CachedConnection](
		ttlcache.WithTTL[string, sqlds.CachedConnection](ttl),
	)
	unsubscribe := cache.OnEviction(func(_ context.Context, _ ttlcache.EvictionReason, item *ttlcache.Item[string, sqlds.CachedConnection]) {
		_ = item.Value().Close()
	})
	go cache.Start()
	return &TTLConnectionCache{
		inner:        cache,
		bootstrapKey: uid + "-default",
		unsubscribe:  unsubscribe,
	}
}

func (c *TTLConnectionCache) Load(key string) (sqlds.CachedConnection, bool) {
	item := c.inner.Get(key)
	if item == nil {
		return nil, false
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
		_ = item.Value().Close()
	}
	c.inner.DeleteAll()
}

package plugin

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/sqlds/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closeCounter is a test double for the cache's closeConn seam. Upstream's
// sqlds.CachedConnection is an opaque value struct whose Close() is a no-op for
// the zero value (nil db), so the cache's close behaviour is observed through
// this counter rather than through the stored value.
type closeCounter struct{ n atomic.Int32 }

func (c *closeCounter) close(sqlds.CachedConnection) error { c.n.Add(1); return nil }
func (c *closeCounter) count() int                         { return int(c.n.Load()) }

// newCache builds a cache with the production closer and registers Dispose for
// cleanup. Tests that assert close behaviour use newCountingCache instead.
func newCache(t *testing.T, uid string, ttl time.Duration) *TTLConnectionCache {
	t.Helper()
	c := newTTLConnectionCache(uid, ttl, sqlds.CachedConnection.Close)
	t.Cleanup(c.Dispose)
	return c
}

// newCountingCache injects a counting closer at construction so the async
// OnEviction callback observes it with a happens-before edge (race-free under
// `go test -race`). Dispose is NOT auto-registered — the close-behaviour tests
// drive it explicitly.
func newCountingCache(uid string, ttl time.Duration) (*TTLConnectionCache, *closeCounter) {
	cc := &closeCounter{}
	return newTTLConnectionCache(uid, ttl, cc.close), cc
}

func TestTTLCache_StoreLoadRoundTrip(t *testing.T) {
	c := newCache(t, "uid1", time.Hour)
	c.Store("uid1-foo", sqlds.CachedConnection{})

	_, ok := c.Load("uid1-foo")
	assert.True(t, ok, "stored key must be present")
}

func TestTTLCache_LoadMissReturnsZeroValueFalse(t *testing.T) {
	c := newCache(t, "uid1", time.Hour)
	got, ok := c.Load("absent")
	assert.False(t, ok)
	assert.Equal(t, sqlds.CachedConnection{}, got, "miss must return the zero CachedConnection")
}

func TestTTLCache_ConstructorReturnsUsableCache(t *testing.T) {
	c, ok := NewTTLConnectionCache("uid1", time.Hour).(*TTLConnectionCache)
	require.True(t, ok, "constructor must return *TTLConnectionCache")
	t.Cleanup(c.Dispose)

	c.Store("uid1-k", sqlds.CachedConnection{})
	_, present := c.Load("uid1-k")
	assert.True(t, present)
}

func TestTTLCache_BootstrapKeySurvivesPastTTL(t *testing.T) {
	c, cc := newCountingCache("uid1", 50*time.Millisecond)
	t.Cleanup(c.Dispose)
	c.Store("uid1-default", sqlds.CachedConnection{})

	time.Sleep(200 * time.Millisecond)

	_, ok := c.Load("uid1-default")
	assert.True(t, ok, "bootstrap key must survive past TTL")
	assert.Equal(t, 0, cc.count(), "bootstrap entry must not be closed while live")
}

func TestTTLCache_PerUserKeyEvictsAndCloses(t *testing.T) {
	c, cc := newCountingCache("uid1", 50*time.Millisecond)
	t.Cleanup(c.Dispose)
	c.Store("uid1-userkey", sqlds.CachedConnection{})

	// ttlcache defaults to touch-on-hit, so calling Load during the wait
	// would extend the TTL. A single Sleep past 5× the TTL is safe.
	time.Sleep(300 * time.Millisecond)

	_, ok := c.Load("uid1-userkey")
	assert.False(t, ok, "per-user key must expire after TTL")

	// OnEviction is dispatched asynchronously by ttlcache. Give the callback
	// goroutine a beat to land before asserting the close count.
	assert.Eventually(t, func() bool { return cc.count() == 1 },
		500*time.Millisecond, 20*time.Millisecond,
		"OnEviction must invoke closeConn exactly once")
}

func TestTTLCache_Range_VisitsAllAndShortCircuits(t *testing.T) {
	c := newCache(t, "uid1", time.Hour)
	c.Store("uid1-a", sqlds.CachedConnection{})
	c.Store("uid1-b", sqlds.CachedConnection{})
	c.Store("uid1-c", sqlds.CachedConnection{})

	t.Run("visits all when f always true", func(t *testing.T) {
		seen := 0
		c.Range(func(string, sqlds.CachedConnection) bool {
			seen++
			return true
		})
		assert.Equal(t, 3, seen)
	})

	t.Run("stops after f returns false", func(t *testing.T) {
		seen := 0
		c.Range(func(string, sqlds.CachedConnection) bool {
			seen++
			return false
		})
		assert.Equal(t, 1, seen)
	})
}

func TestTTLCache_Dispose_ClosesEverythingExactlyOnce(t *testing.T) {
	// Dispose is the system-under-test; drive it explicitly (no t.Cleanup).
	c, cc := newCountingCache("uid1", time.Hour)
	c.Store("uid1-default", sqlds.CachedConnection{})
	c.Store("uid1-a", sqlds.CachedConnection{})
	c.Store("uid1-b", sqlds.CachedConnection{})

	c.Dispose()

	assert.Equal(t, 3, cc.count(), "Dispose must close every live entry exactly once")

	// No async eviction callback double-closes after Dispose detaches it.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 3, cc.count(), "no entry may be closed a second time after Dispose")

	_, ok := c.Load("uid1-a")
	assert.False(t, ok, "cache must be empty after Dispose")
}

func TestTTLCache_Dispose_StopsSweepGoroutine(t *testing.T) {
	baseline := runtime.NumGoroutine()

	// Constructor starts the sweep goroutine via `go cache.Start()`.
	caches := make([]*TTLConnectionCache, 10)
	for i := range caches {
		caches[i] = newTTLConnectionCache("uid", time.Hour, sqlds.CachedConnection.Close)
	}

	// Give the goroutines time to spin up before measuring growth.
	time.Sleep(50 * time.Millisecond)
	withCaches := runtime.NumGoroutine()
	assert.Greater(t, withCaches, baseline, "expected goroutine count to grow with running sweeps")

	for _, c := range caches {
		c.Dispose()
	}

	// Allow the stopped sweeps to settle. The number doesn't have to hit
	// baseline exactly (the runtime may park goroutines during the test),
	// but it must drop substantially.
	time.Sleep(100 * time.Millisecond)
	afterDispose := runtime.NumGoroutine()
	assert.Less(t, afterDispose, withCaches, "Dispose must terminate sweep goroutines")
}

func TestTTLCache_ConcurrentLoadStoreRangeIsRaceFree(t *testing.T) {
	// Exercise the cache from many goroutines so `go test -race` can surface
	// any shared-state bug. Correctness assertions are minimal — this is a
	// smoke test for races, not for behaviour.
	c := newCache(t, "uid1", time.Hour)

	const goroutines = 16
	const opsPer = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < opsPer; i++ {
				c.Store("k", sqlds.CachedConnection{})
				_, _ = c.Load("k")
				c.Range(func(string, sqlds.CachedConnection) bool { return true })
			}
		}()
	}
	wg.Wait()
}

package plugin

import (
	"database/sql"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubCachedConn is a minimal sqlds.CachedConnection used by cache tests
// that don't need a real *sql.DB. It records Close calls so tests can
// assert the cache's eviction/dispose semantics.
type stubCachedConn struct {
	closes atomic.Int32
}

func (s *stubCachedConn) DB() *sql.DB                                   { return nil }
func (s *stubCachedConn) Settings() backend.DataSourceInstanceSettings  { return backend.DataSourceInstanceSettings{} }
func (s *stubCachedConn) Close() error                                  { s.closes.Add(1); return nil }
func (s *stubCachedConn) Closes() int                                   { return int(s.closes.Load()) }

func newCache(t *testing.T, uid string, ttl time.Duration) *TTLConnectionCache {
	t.Helper()
	cache, ok := NewTTLConnectionCache(uid, ttl).(*TTLConnectionCache)
	require.True(t, ok, "constructor must return *TTLConnectionCache")
	t.Cleanup(cache.Dispose)
	return cache
}

func TestTTLCache_StoreLoadRoundTrip(t *testing.T) {
	c := newCache(t, "uid1", time.Hour)
	conn := &stubCachedConn{}
	c.Store("uid1-foo", conn)

	got, ok := c.Load("uid1-foo")
	assert.True(t, ok)
	// Reference equality — the cache must not wrap or decorate.
	assert.Same(t, conn, got)
}

func TestTTLCache_LoadMissReturnsNilFalse(t *testing.T) {
	c := newCache(t, "uid1", time.Hour)
	got, ok := c.Load("absent")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestTTLCache_NoWrappingContract(t *testing.T) {
	// Custom interface value to catch any decoration in the impl.
	c := newCache(t, "uid1", time.Hour)
	var stored sqlds.CachedConnection = &stubCachedConn{}
	c.Store("k", stored)

	got, ok := c.Load("k")
	require.True(t, ok)
	assert.True(t, stored == got, "Load must return the exact interface value passed to Store")
}

func TestTTLCache_BootstrapKeySurvivesPastTTL(t *testing.T) {
	c := newCache(t, "uid1", 50*time.Millisecond)
	conn := &stubCachedConn{}
	c.Store("uid1-default", conn)

	time.Sleep(200 * time.Millisecond)

	got, ok := c.Load("uid1-default")
	assert.True(t, ok, "bootstrap key must survive past TTL")
	assert.Same(t, conn, got)
	assert.Equal(t, 0, conn.Closes(), "bootstrap Close must not be invoked while live")
}

func TestTTLCache_PerUserKeyEvictsAndCloses(t *testing.T) {
	c := newCache(t, "uid1", 50*time.Millisecond)
	conn := &stubCachedConn{}
	c.Store("uid1-userkey", conn)

	// ttlcache defaults to touch-on-hit, so calling Load during the wait
	// would extend the TTL. A single Sleep past 5× the TTL is safe.
	time.Sleep(300 * time.Millisecond)

	got, ok := c.Load("uid1-userkey")
	assert.False(t, ok, "per-user key must expire after TTL")
	assert.Nil(t, got)

	// OnEviction is dispatched asynchronously by ttlcache. Give the
	// callback goroutine a beat to land before asserting close count.
	assert.Eventually(t, func() bool { return conn.Closes() == 1 },
		500*time.Millisecond, 20*time.Millisecond,
		"OnEviction must close exactly once")
}

func TestTTLCache_Range_VisitsAllAndShortCircuits(t *testing.T) {
	c := newCache(t, "uid1", time.Hour)
	c.Store("uid1-a", &stubCachedConn{})
	c.Store("uid1-b", &stubCachedConn{})
	c.Store("uid1-c", &stubCachedConn{})

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
	// Drop the t.Cleanup default by constructing without it; Dispose is
	// the system-under-test here.
	cache, ok := NewTTLConnectionCache("uid1", time.Hour).(*TTLConnectionCache)
	require.True(t, ok)

	conns := []*stubCachedConn{{}, {}, {}}
	cache.Store("uid1-default", conns[0])
	cache.Store("uid1-a", conns[1])
	cache.Store("uid1-b", conns[2])

	cache.Dispose()

	for i, conn := range conns {
		assert.Equal(t, 1, conn.Closes(), "conn[%d] must be closed exactly once on Dispose", i)
	}
	got, ok := cache.Load("uid1-a")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestTTLCache_Dispose_StopsSweepGoroutine(t *testing.T) {
	baseline := runtime.NumGoroutine()

	// Constructor starts the sweep goroutine via `go cache.Start()`.
	caches := make([]*TTLConnectionCache, 10)
	for i := range caches {
		c, ok := NewTTLConnectionCache("uid", time.Hour).(*TTLConnectionCache)
		require.True(t, ok)
		caches[i] = c
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
	// Exercise the cache from many goroutines so `go test -race` can
	// surface any shared-state bug. Correctness assertions are minimal —
	// this is a smoke test for races, not for behaviour.
	c := newCache(t, "uid1", time.Hour)

	const goroutines = 16
	const opsPer = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPer; i++ {
				c.Store("k", &stubCachedConn{})
				_, _ = c.Load("k")
				c.Range(func(string, sqlds.CachedConnection) bool { return true })
			}
		}(g)
	}
	wg.Wait()
}

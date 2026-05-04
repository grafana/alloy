package relabel

import (
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
)

// relabelCache is the per-component cache of relabeled labels keyed by
// the input labels' hash. Implementations are safe for concurrent use
// and may own background goroutines; callers must Close when done.
type relabelCache interface {
	// Get returns the cached relabel result for the given hash. The
	// boolean is false if no entry is cached (or, in TTL mode, if the
	// entry has expired).
	Get(hash uint64) (labels.Labels, bool)
	// Add inserts or refreshes the cached relabel result for the given
	// hash.
	Add(hash uint64, lbls labels.Labels)
	// Remove drops the cached entry for the given hash, if present.
	Remove(hash uint64)
	// Len returns the number of entries currently in the cache.
	Len() int
	// Close releases any background resources held by the cache.
	Close()
}

// newRelabelCache constructs a cache from args. TTL caches return with
// their scan goroutine already running; the caller owns Close.
func newRelabelCache(args Arguments, evictions prometheus_client.Counter) (relabelCache, error) {
	if args.CacheTTL > 0 {
		c := newTTLRelabelCache(args.CacheTTL, evictions)
		c.start()
		return c, nil
	}
	c, err := lru.New[uint64, labels.Labels](args.CacheSize)
	if err != nil {
		return nil, err
	}
	return &lruRelabelCache{c: c}, nil
}

type lruRelabelCache struct {
	c *lru.Cache[uint64, labels.Labels]
}

func (c *lruRelabelCache) Get(hash uint64) (labels.Labels, bool) {
	return c.c.Get(hash)
}

func (c *lruRelabelCache) Add(hash uint64, lbls labels.Labels) {
	c.c.Add(hash, lbls)
}

func (c *lruRelabelCache) Remove(hash uint64) {
	c.c.Remove(hash)
}

func (c *lruRelabelCache) Len() int { return c.c.Len() }
func (c *lruRelabelCache) Close()   {}

// ttlEntry's lbls is immutable after insertion (same input hash always
// yields the same relabel output within a cache lifetime); expires is
// atomic so Get can slide the TTL window without taking the write lock.
type ttlEntry struct {
	lbls    labels.Labels
	expires atomic.Int64 // Unix seconds
}

// ttlRelabelCache is a TTL-bounded cache with no hard size limit. Each
// Get slides the entry's expiry forward, so active series stay cached
// while inactive entries are reaped after cache_ttl.
//
// Go maps don't release bucket memory on delete, so the underlying map
// holds at the peak working set for the cache's lifetime; restart or
// config reload to reclaim it.
type ttlRelabelCache struct {
	mu      sync.RWMutex
	entries map[uint64]*ttlEntry

	ttl          time.Duration
	scanInterval time.Duration

	evictions prometheus_client.Counter

	startOnce sync.Once
	closeOnce sync.Once
	closeCh   chan struct{}
	scanWG    sync.WaitGroup
}

// scanIntervalFor caps post-expiry lag at ~25% of the TTL.
func scanIntervalFor(ttl time.Duration) time.Duration {
	return ttl / 4
}

func newTTLRelabelCache(ttl time.Duration, evictions prometheus_client.Counter) *ttlRelabelCache {
	return &ttlRelabelCache{
		entries:      make(map[uint64]*ttlEntry),
		ttl:          ttl,
		scanInterval: scanIntervalFor(ttl),
		evictions:    evictions,
		closeCh:      make(chan struct{}),
	}
}

func (c *ttlRelabelCache) Get(hash uint64) (labels.Labels, bool) {
	c.mu.RLock()
	entry, ok := c.entries[hash]
	c.mu.RUnlock()
	if !ok {
		return labels.EmptyLabels(), false
	}
	// Slide the TTL window forward without upgrading to the write lock.
	entry.expires.Store(time.Now().Add(c.ttl).Unix())
	return entry.lbls, true
}

func (c *ttlRelabelCache) Add(hash uint64, lbls labels.Labels) {
	expires := time.Now().Add(c.ttl).Unix()
	c.mu.Lock()
	if existing, ok := c.entries[hash]; ok {
		// Concurrent miss: keep lbls immutable and just refresh expiry.
		existing.expires.Store(expires)
		c.mu.Unlock()
		return
	}
	e := &ttlEntry{lbls: lbls}
	e.expires.Store(expires)
	c.entries[hash] = e
	c.mu.Unlock()
}

func (c *ttlRelabelCache) Remove(hash uint64) {
	c.mu.Lock()
	delete(c.entries, hash)
	c.mu.Unlock()
}

func (c *ttlRelabelCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// start spawns the background scan goroutine. Idempotent.
func (c *ttlRelabelCache) start() {
	c.startOnce.Do(func() {
		c.scanWG.Add(1)
		go func() {
			defer c.scanWG.Done()
			t := time.NewTicker(c.scanInterval)
			defer t.Stop()
			for {
				select {
				case <-c.closeCh:
					return
				case now := <-t.C:
					c.scan(now)
				}
			}
		}()
	})
}

// Close terminates the background scan goroutine and blocks until it
// exits. Idempotent.
func (c *ttlRelabelCache) Close() {
	c.closeOnce.Do(func() { close(c.closeCh) })
	c.scanWG.Wait()
}

// scan prunes expired entries.
func (c *ttlRelabelCache) scan(now time.Time) {
	nowSec := now.Unix()
	// Phase 1: collect expired keys under RLock. If there's no work,
	// skip the write lock entirely.
	c.mu.RLock()
	var expired []uint64
	for h, e := range c.entries {
		if nowSec > e.expires.Load() {
			expired = append(expired, h)
		}
	}
	c.mu.RUnlock()

	if len(expired) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Phase 2: prune. Re-check expiry per key in case a concurrent Get
	// slid the entry forward (or Add refreshed it) between the phases.
	for _, h := range expired {
		if e, ok := c.entries[h]; ok && nowSec > e.expires.Load() {
			delete(c.entries, h)
			c.evictions.Inc()
		}
	}
}

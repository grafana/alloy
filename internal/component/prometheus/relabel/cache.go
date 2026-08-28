package relabel

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"go.uber.org/atomic"
)

// relabelCache is the per-component cache of relabeled labels keyed by
// the input labels' hash. Implementations are safe for concurrent use
// and may own background goroutines; callers must close when done.
type relabelCache interface {
	// get returns the cached relabel result for the given hash. The
	// boolean is false if no entry is cached.
	get(hash uint64) (labels.Labels, bool)
	// add inserts or refreshes the cached relabel result for the given
	// hash.
	add(hash uint64, lbls labels.Labels)
	// remove drops the cached entry for the given hash, if present.
	remove(hash uint64)
	// len returns the number of entries currently in the cache.
	len() int
	// close releases any background resources held by the cache.
	close()
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

func (c *lruRelabelCache) get(hash uint64) (labels.Labels, bool) {
	return c.c.Get(hash)
}

func (c *lruRelabelCache) add(hash uint64, lbls labels.Labels) {
	c.c.Add(hash, lbls)
}

func (c *lruRelabelCache) remove(hash uint64) {
	c.c.Remove(hash)
}

func (c *lruRelabelCache) len() int { return c.c.Len() }
func (c *lruRelabelCache) close()   {}

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
// Go maps don't release bucket memory on delete, so a transient
// cardinality spike would otherwise pin the map at its peak footprint
// indefinitely. The scanner rebuilds the map when live entries drop far
// enough below the high-water mark for the reclaim to be worth the copy.
type ttlRelabelCache struct {
	mu      sync.RWMutex
	entries map[uint64]*ttlEntry
	// peak is the high-water mark of map size since the last rebuild;
	// scan compares len(entries) against it to decide when to shrink.
	peak int

	ttl          time.Duration
	scanInterval time.Duration

	evictions prometheus_client.Counter

	startOnce sync.Once
	closeOnce sync.Once
	closeCh   chan struct{}
	scanWG    sync.WaitGroup
}

// shouldShrink applies a two-tier policy. Reclaim scales roughly linearly
// with peak (empirically ~18MB at 1M / 50% drop, ~150MB at 10M / 33%
// drop), so larger peaks justify shrinking at smaller drops. Below 1M
// the absolute savings (≤2MB at 10x drop) aren't worth the rebuild.
func shouldShrink(peak, live int) bool {
	switch {
	case peak >= 10_000_000:
		return live*3 <= peak*2 // 33%+ drop, reclaims ~150MB+
	case peak >= 1_000_000:
		return live*2 <= peak // 50%+ drop, reclaims ~18MB+
	default:
		return false
	}
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

func (c *ttlRelabelCache) get(hash uint64) (labels.Labels, bool) {
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

func (c *ttlRelabelCache) add(hash uint64, lbls labels.Labels) {
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
	if n := len(c.entries); n > c.peak {
		c.peak = n
	}
	c.mu.Unlock()
}

func (c *ttlRelabelCache) remove(hash uint64) {
	c.mu.Lock()
	delete(c.entries, hash)
	c.mu.Unlock()
}

func (c *ttlRelabelCache) len() int {
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

// close terminates the background scan goroutine and blocks until it
// exits. Idempotent.
func (c *ttlRelabelCache) close() {
	c.closeOnce.Do(func() { close(c.closeCh) })
	c.scanWG.Wait()
}

// scan prunes expired entries and rebuilds the map when it has drained
// far enough below its peak to make the reclaim worthwhile.
func (c *ttlRelabelCache) scan(now time.Time) {
	nowSec := now.Unix()
	// Phase 1: collect expired keys under RLock. Also peek at the
	// shrink condition so a drain-only cycle (entries removed via stale
	// markers, nothing expired) still triggers the write lock.
	c.mu.RLock()
	var expired []uint64
	for h, e := range c.entries {
		if nowSec > e.expires.Load() {
			expired = append(expired, h)
		}
	}
	maybeShrink := shouldShrink(c.peak, len(c.entries)-len(expired))
	c.mu.RUnlock()

	if len(expired) == 0 && !maybeShrink {
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

	// Phase 3: shrink if the live set has drained far enough below peak
	// that the bucket reclaim outweighs the rebuild cost. Resetting peak
	// to the new size provides natural hysteresis — we can't rebuild
	// again until the map grows back up and drains.
	//
	// The old and new maps coexist until this function returns and the
	// old one becomes unreachable, so peak footprint briefly grows by
	// the new map's bucket size (roughly len/peak of the existing
	// allocation). Acceptable: we'd rather hold the spike for one scan
	// interval than carry the bloated map indefinitely.
	if shouldShrink(c.peak, len(c.entries)) {
		rebuilt := make(map[uint64]*ttlEntry, len(c.entries))
		for h, e := range c.entries {
			rebuilt[h] = e
		}
		c.entries = rebuilt
		c.peak = len(c.entries)
	}
}

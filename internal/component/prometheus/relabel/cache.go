package relabel

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
)

// relabelCache abstracts the per-component cache of relabeled labels keyed
// by the input labels' hash. Implementations cover two modes selectable
// via Arguments: bounded LRU (default) and time-bounded ("size dictated
// by working set"). All methods are safe for concurrent use.
//
// Implementations own any background work (e.g. periodic eviction).
// `newRelabelCache` returns a ready-to-use cache; callers must invoke
// Close when the cache is no longer needed. Tests that want to drive
// eviction manually can use the per-impl bare constructors (e.g.
// `newTTLRelabelCache`) which don't spawn the background goroutine.
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

// newRelabelCache constructs a cache impl based on Arguments and
// readies it for use (TTL caches have their background scan goroutine
// running on return). Validation has already ensured exactly one of
// CacheSize and CacheTTL is non-zero, so the lru.New error here is
// effectively unreachable in production but propagated for safety.
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

type ttlEntry struct {
	lbls    labels.Labels
	expires time.Time
}

// ttlRelabelCache is a TTL-bounded cache without a hard size limit. The
// active set sizes itself to the working set of series flowing through
// the component. Entries expire after a fixed TTL relative to the most
// recent insertion.
//
// Note: Go maps don't return bucket memory on delete, so the underlying
// map's footprint stays at the peak working set for the cache's
// lifetime. Operators who need to reclaim that memory should restart
// the component (config reload also recreates the cache from scratch).
type ttlRelabelCache struct {
	mu      sync.RWMutex
	entries map[uint64]ttlEntry

	ttl          time.Duration
	scanInterval time.Duration

	evictions prometheus_client.Counter

	startOnce sync.Once
	closeOnce sync.Once
	closeCh   chan struct{}
	scanWG    sync.WaitGroup
}

// scanIntervalFor scales the background scan cadence to the configured
// TTL: scanning at TTL/4 caps the post-expiry lag at ~25% of the TTL,
// which keeps the cache sized to roughly the working set without
// scanning so often that the bookkeeping dominates real work.
func scanIntervalFor(ttl time.Duration) time.Duration {
	return ttl / 4
}

func newTTLRelabelCache(ttl time.Duration, evictions prometheus_client.Counter) *ttlRelabelCache {
	return &ttlRelabelCache{
		entries:      make(map[uint64]ttlEntry),
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
	// Lazy expiration: an entry past its expiry counts as a miss even if
	// the periodic scan hasn't pruned it yet.
	if time.Now().After(entry.expires) {
		return labels.EmptyLabels(), false
	}
	return entry.lbls, true
}

func (c *ttlRelabelCache) Add(hash uint64, lbls labels.Labels) {
	c.mu.Lock()
	c.entries[hash] = ttlEntry{lbls: lbls, expires: time.Now().Add(c.ttl)}
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

// Close terminates the background scan goroutine (if any) and blocks
// until it exits. Idempotent. Safe to call even if start was never
// called. The blocking-wait matters for testing/synctest's deadlock
// detector, which fires if any goroutine in the bubble is still
// running when the test's main goroutine returns.
func (c *ttlRelabelCache) Close() {
	c.closeOnce.Do(func() { close(c.closeCh) })
	c.scanWG.Wait()
}

// scan prunes expired entries.
func (c *ttlRelabelCache) scan(now time.Time) {
	// Phase 1: collect expired keys under RLock. If there's no work,
	// skip the write lock entirely.
	c.mu.RLock()
	var expired []uint64
	for h, e := range c.entries {
		if now.After(e.expires) {
			expired = append(expired, h)
		}
	}
	c.mu.RUnlock()

	if len(expired) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Phase 2: prune. Re-check expiry per key in case a concurrent
	// Add refreshed the entry between the phases.
	for _, h := range expired {
		if e, ok := c.entries[h]; ok && now.After(e.expires) {
			delete(c.entries, h)
			c.evictions.Inc()
		}
	}
}

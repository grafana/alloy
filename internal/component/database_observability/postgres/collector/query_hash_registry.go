package collector

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// QueryHashInfo holds the per-queryid data the metrics collector needs at
// scrape time. The fields are intentionally narrow: anything more goes on the
// emitted Loki entry, not on the join metric (cardinality control).
type QueryHashInfo struct {
	Fingerprint  string
	DatabaseName string
	LastSeen     time.Time
}

// QueryHashRegistry is a small LRU+TTL cache mapping the native PostgreSQL
// queryid (string form) to the semantic fingerprint Alloy computed from that
// query's text. The query_details collector populates it on each scrape; the
// query_samples collector and the query_hash_info Prometheus collector read
// from it.
//
// The size cap matches the pg_stat_statements.max default ceiling; TTL ensures
// stale queryids don't keep being exported after the database evicts them.
type QueryHashRegistry struct {
	mu    sync.RWMutex
	cache *expirable.LRU[string, QueryHashInfo]
}

func NewQueryHashRegistry(size int, ttl time.Duration) *QueryHashRegistry {
	return &QueryHashRegistry{
		cache: expirable.NewLRU[string, QueryHashInfo](size, nil, ttl),
	}
}

func (r *QueryHashRegistry) Set(queryID, fingerprint, databaseName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache.Add(queryID, QueryHashInfo{
		Fingerprint:  fingerprint,
		DatabaseName: databaseName,
		LastSeen:     time.Now(),
	})
}

func (r *QueryHashRegistry) Get(queryID string) (QueryHashInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cache.Get(queryID)
}

// Snapshot returns a shallow copy of all live entries. Used by the metrics
// collector at scrape time; not on the hot path of any collector.
func (r *QueryHashRegistry) Snapshot() map[string]QueryHashInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[string]QueryHashInfo, r.cache.Len())
	for _, k := range r.cache.Keys() {
		if v, ok := r.cache.Peek(k); ok {
			out[k] = v
		}
	}
	return out
}

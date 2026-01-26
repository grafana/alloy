package collector

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// QueryHashInfo contains information about a query hash
type QueryHashInfo struct {
	QueryHash    string
	DatabaseName string
	LastSeen     time.Time
}

// QueryHashRegistry maintains a mapping of queryid to queryhash
type QueryHashRegistry struct {
	mu    sync.RWMutex
	cache *expirable.LRU[string, QueryHashInfo]
}

// NewQueryHashRegistry creates a new QueryHashRegistry
func NewQueryHashRegistry(size int, ttl time.Duration) *QueryHashRegistry {
	return &QueryHashRegistry{
		cache: expirable.NewLRU[string, QueryHashInfo](size, nil, ttl),
	}
}

// Set stores a queryid to queryhash mapping
func (r *QueryHashRegistry) Set(queryID, queryHash, databaseName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache.Add(queryID, QueryHashInfo{
		QueryHash:    queryHash,
		DatabaseName: databaseName,
		LastSeen:     time.Now(),
	})
}

// Get retrieves the QueryHashInfo for a given queryID
func (r *QueryHashRegistry) Get(queryID string) (QueryHashInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.cache.Get(queryID)
	return info, ok
}

// GetAll returns all current queryid to queryhash mappings
func (r *QueryHashRegistry) GetAll() map[string]QueryHashInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]QueryHashInfo)
	for _, key := range r.cache.Keys() {
		if info, ok := r.cache.Peek(key); ok {
			result[key] = info
		}
	}
	return result
}

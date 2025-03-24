package cache

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type InMemoryCache[valueType any] struct {
	lru       *lru.Cache[string, *valueType]
	cacheSize int
	cacheMut  sync.RWMutex
}

// NewInMemoryCacheWithConfig creates a new thread-safe LRU cache for index entries and ensures the total cache
// size approximately does not exceed maxBytes.
func NewInMemoryCacheWithConfig[valueType any](config InMemoryCacheConfig) (*InMemoryCache[valueType], error) {
	c := &InMemoryCache[valueType]{
		cacheSize: config.CacheSize,
	}

	// Initialize LRU cache
	cache, err := lru.New[string, *valueType](c.cacheSize)
	if err != nil {
		return nil, err
	}
	c.lru = cache

	return c, nil
}

func (c *InMemoryCache[valueType]) Get(key string) (*valueType, error) {
	c.cacheMut.RLock()
	defer c.cacheMut.RUnlock()

	fm, found := c.lru.Get(key)
	if !found {
		return nil, errNotFound
	}

	return fm, nil
}

func (c *InMemoryCache[valueType]) GetMultiple(keys []string) (map[string]*valueType, error) {
	c.cacheMut.RLock()
	defer c.cacheMut.RUnlock()

	values := make(map[string]*valueType, len(keys))

	for _, key := range keys {
		found := false
		values[key], found = c.lru.Get(key)
		if !found {
			return nil, errNotFound
		}
	}

	return values, nil
}

func (c *InMemoryCache[valueType]) Remove(key string) {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()

	c.lru.Remove(key)
}

func (c *InMemoryCache[valueType]) Set(key string, value *valueType, ttl time.Duration) error {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()

	c.lru.Add(key, value)

	return nil
}

func (c *InMemoryCache[valueType]) SetMultiple(values map[string]*valueType, ttl time.Duration) error {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()

	for key, value := range values {
		c.lru.Add(key, value)
	}

	return nil
}

func (c *InMemoryCache[valueType]) Clear(newSize int) error {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()
	lru, err := lru.New[string, *valueType](newSize)
	if err != nil {
		return err
	}

	c.lru = lru
	return nil
}

func (c *InMemoryCache[valueType]) GetCacheSize() int {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()
	return c.lru.Len()
}

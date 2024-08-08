package cache

import (
	"time"

	"github.com/grafana/dskit/cache"
	"github.com/grafana/dskit/flagext"
	"github.com/pkg/errors"
)

const (
	// InMemory is the value for the in-memory cache backend.
	InMemory = "inmemory"

	// Memcached is the value for the Memcached cache backend.
	Memcached = cache.BackendMemcached

	// Redis is the value for the Redis cache backend.
	Redis = cache.BackendRedis

	// Default is the value for the default cache backend.
	Default = InMemory
)

var (
	SupportedCaches = []string{InMemory, Memcached, Redis}

	errUnsupportedCache = errors.New("unsupported cache backend")
	errNotFound         = errors.New("not found in cache")
)

type CacheConfig struct {
	Backend   string              `alloy:"backend,attr"`
	Memcached MemcachedConfig     `alloy:"memcached,block,optional"`
	Redis     RedisConf           `alloy:"redis,block,optional"`
	InMemory  InMemoryCacheConfig `alloy:"inmemory,block,optional"`
}

//TODO Those field are copied from dskit/cache for now (only the one mandatory)
// We need to have a better way to manage conf
// For now I used those because we cannot embed 'yaml' tags into alloy configuration
// Ideally we should be using the dskit/cache conf directly, but it means it should not
// be into the alloy configuration ?

type RedisConf struct {
	// Endpoint specifies the endpoint of Redis server.
	Endpoint flagext.StringSliceCSV `alloy:"endpoint,attr"`

	// Use the specified Username to authenticate the current connection
	// with one of the connections defined in the ACL list when connecting
	// to a Redis 6.0 instance, or greater, that is using the Redis ACL system.
	Username string `alloy:"username,attr"`

	// Optional password. Must match the password specified in the
	// requirepass server configuration option (if connecting to a Redis 5.0 instance, or lower),
	// or the User Password when connecting to a Redis 6.0 instance, or greater,
	// that is using the Redis ACL system.
	Password string `alloy:"password,attr,optional"`

	// DB Database to be selected after connecting to the server.
	DB int `alloy:"db,attr"`

	// MaxAsyncConcurrency specifies the maximum number of SetAsync goroutines.
	MaxAsyncConcurrency int `yaml:"max_async_concurrency" category:"advanced"`

	// MaxAsyncBufferSize specifies the queue buffer size for SetAsync operations.
	MaxAsyncBufferSize int `yaml:"max_async_buffer_size" category:"advanced"`
}

type MemcachedConfig struct {
	// Addresses specifies the list of memcached addresses. The addresses get
	// resolved with the DNS provider.
	Addresses flagext.StringSliceCSV `alloy:"addresses,attr"`

	// WriteBufferSizeBytes specifies the size of the write buffer (in bytes). The buffer
	// is allocated for each connection.
	WriteBufferSizeBytes int `alloy:"write_buffer_size_bytes,attr"`

	// ReadBufferSizeBytes specifies the size of the read buffer (in bytes). The buffer
	// is allocated for each connection.
	ReadBufferSizeBytes int `alloy:"read_buffer_size_bytes,attr"`

	// MaxAsyncConcurrency specifies the maximum number of SetAsync goroutines.
	MaxAsyncConcurrency int `yaml:"max_async_concurrency" category:"advanced"`

	// MaxAsyncBufferSize specifies the queue buffer size for SetAsync operations.
	MaxAsyncBufferSize int `yaml:"max_async_buffer_size" category:"advanced"`
}

type InMemoryCacheConfig struct {
	CacheSize int `alloy:"cache_size,attr"`
}

type Cache[valueType any] interface {
	Get(key string) (*valueType, error)
	GetMultiple(keys []string) (map[string]*valueType, error)
	Set(key string, value *valueType, ttl time.Duration) error
	SetMultiple(values map[string]*valueType, ttl time.Duration) error
	Remove(key string)
	Clear(newSize int) error
	GetCacheSize() int
}

// NewCache creates a new cache based on the given configuration
func NewCache[valueType any](cfg CacheConfig) (Cache[valueType], error) {
	switch cfg.Backend {
	case InMemory:
		return NewInMemoryCacheWithConfig[valueType](InMemoryCacheConfig{
			CacheSize: cfg.InMemory.CacheSize,
		})
	case Memcached:
		return newMemcachedCache[valueType](cfg.Memcached)
	case Redis:
		return newRedisCache[valueType](cfg.Redis)
	default:
		return nil, errUnsupportedCache
	}
}

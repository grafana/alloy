# Proposal: Prometheus relabel external cache

- Author(s): Wilfried ROSET (@wilfriedroset), Pierre BAILHACHE (@pbailhache)
- Last updated: 2024-09-02
- Original issue: <https://github.com/grafana/alloy/issues/1600>

## Abstract

This proposal introduces a way to configure the component `prometheus.relabel` so that it could use an external cache such as `Redis` or `Memcached` instead of `in-memory`.

## Problem

The `prometheus.relabel` component rewrites the label set of each metric passed along to the exported receiver by applying one or more relabeling rules. To do so it uses a relabeling cache stored in memory. For each `prometheus.relabel` component Alloy will create a dedicated relabel cache. This is not a huge issue per se due to the possibility of registering multiple rules into on component to mutualize the cache.

However if you're horizontally scaling Alloy deployment with load-balancing, you will end up with one cache per Alloy instance and those local caches will have overlaps, increasing the footprint of each instance.
Moreover, the cache is tied to the pod, which means that a new Alloy process starts with an empty cache.

For a couple of horizontally scaled Alloy pods it's acceptable but if you plan to have lots of instances processing data horizontally it's not sustainable.

## Proposal

Allow to use `Redis` or `Memcached` instead of the `in-memory` one as cache.

Using [dskit](https://github.com/grafana/dskit/blob/main/cache/cache.go) code to manage connection and client configuration.

We could create an interface to avoid changing the code logic and to abstract the kind of cache used to the component.

## Pros and cons

**Pros:**

- No logic change so impact is expected to be null for users
- Possibility to use an external cache if needed, even having multiple caches for different relabeling components.
- Will be easy to use in other parts of the codebase

**Cons:**

- Config is a bit more complex compared to previous one

## Alternative solutions

The alternative is to do nothing as we deem this improvement unnecessary.

## Compatibility

This proposal offer to deprecate the old way to configure the in-memory cache and drop it in the next major release (e.g: 2.0). Doing so allow to migrate the settings to the correct block.

## Implementation

We should add a re-usable cache interface compatible with multiple backend, it should be usable with different value types

```golang
type Cache[valueType any] interface {
 Get(key string) (*valueType, error)
 Set(key string, value *valueType, ttl time.Duration) error
 Remove(key string)
 GetMultiple(keys []string) ([]*valueType, error)
 SetMultiple(values map[string]*valueType, ttl time.Duration) error
 Clear(newSize int) error
 GetCacheSize() int
}
```

Each backend cache should be implemented in a separate file. For now we should support in_memory, redis and memcached.

We should add a way to select the cache and its connections options through the components arguments

For example, based on what's done in [Mimir index cache](https://github.com/grafana/mimir/blob/main/pkg/storage/tsdb/index_cache.go#L47):

```golang
type Arguments struct {
    // Where the relabelled metrics should be forwarded to.
    ForwardTo []storage.Appendable `alloy:"forward_to,attr"`

    // The relabelling rules to apply to each metric before it's forwarded.
    MetricRelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`

    // DEPRECATED Use type = inmemory and cache_size field.
    InMemoryCacheSizeDeprecated int `alloy:"max_cache_size,attr,optional"`

    // Cache backend configuration.
    CacheConfig cache.CacheConfig `alloy:"cache,block,optional"`
}

type CacheConfig struct {
   cache.BackendConfig `yaml:",inline"`
   InMemory            InMemoryCacheConfig `yaml:"inmemory"`
}

type InMemoryCacheConfig struct {
   CacheSize int `yaml:"cache_size"`
}

------
type BackendConfig struct {
    Backend   string                `yaml:"backend"`
    Memcached MemcachedClientConfig `yaml:"memcached"`
    Redis     RedisClientConfig     `yaml:"redis"`
}
```

Configuration should be `in_memory` by default.
`max_cache_size` should still be taken into account but only if `backend = in_memory`. It also should be deprecated and we should redirect to the `InMemoryRelabelCacheConfig.CacheSize` field.

Here is some examples:

- legacy config unchanged

```river
prometheus.relabel "legacy_config" {
  forward_to = [...]
  max_cache_size = 10000000
  rule {
   ...
  }

}
```

- redis config

```river
prometheus.relabel "redis_config" {
  forward_to = [...]
  cache {
    backend = "redis"
    redis {
        endpoint = "redis.url"
        username = "user"
        password = "password"
        ...
    }
  }
  ...
}
```

- new in memory config

```river
prometheus.relabel "inmemory_config" {
  forward_to = [...]
  cache {
    backend = "inmemory"
    inmemory {
        cache_size = 10000000
    }
  }
  ...
}
```

- memcached config

```river
prometheus.relabel "memcached_config" {
  forward_to = [...]
  cache {
    backend = "memcached"
    memcached {
        addresses = "address1, address2"
        timeout = 10
        ...
    }
  }
  ...
}
```

## Related open issues

N/A

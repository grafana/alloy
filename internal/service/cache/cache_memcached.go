package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"io"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/cache"
)

type MemcachedCache[valueType any] struct {
	client *cache.MemcachedClient
}

func newMemcachedCache[valueType any](cfg MemcachedConfig) (*MemcachedCache[valueType], error) {
	client, err := cache.NewMemcachedClientWithConfig(
		//TODO NewLogFmtLogger ? Maybe something else
		log.NewLogfmtLogger(os.Stdout),
		"memcached-cache",
		cache.MemcachedClientConfig{
			Addresses:            cfg.Addresses,
			WriteBufferSizeBytes: cfg.WriteBufferSizeBytes,
			ReadBufferSizeBytes:  cfg.ReadBufferSizeBytes,
			MaxAsyncConcurrency:  cfg.MaxAsyncConcurrency,
			MaxAsyncBufferSize:   cfg.MaxAsyncBufferSize,
		},
		//TODO add prometheus registerer here
		nil,
	)

	if err != nil {
		return nil, err
	}

	return &MemcachedCache[valueType]{
		client: client,
	}, nil
}

func (c *MemcachedCache[valueType]) Get(key string) (*valueType, error) {
	ctx := context.Background()
	var out valueType

	data := c.client.GetMulti(ctx, []string{key})
	if data[key] == nil {
		//TODO check if data == nil means only not found ?
		// what happens when network errors ?
		return nil, errNotFound
	}

	decoder := gob.NewDecoder(bytes.NewReader(data[key]))
	if err := decoder.Decode(&out); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, err
		}
	}

	return &out, nil
}

func (c *MemcachedCache[valueType]) GetMultiple(keys []string) (map[string]*valueType, error) {
	ctx := context.Background()

	data := c.client.GetMulti(ctx, keys)
	if data == nil {
		//TODO check if data == nil means only not found ?
		// what happens when network errors ?
		return nil, errNotFound
	}

	result := make(map[string]*valueType, len(keys))

	for key, rawValue := range data {
		decoder := gob.NewDecoder(bytes.NewReader(rawValue))
		if err := decoder.Decode(result[key]); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				return nil, err
			}
		}
	}

	return result, nil
}

func (c *MemcachedCache[valueType]) Remove(key string) {
	ctx := context.Background()
	//TODO manage error
	_ = c.client.Delete(ctx, key)

}

func (c *MemcachedCache[valueType]) Set(key string, value *valueType, ttl time.Duration) error {
	if value == nil {
		c.client.SetAsync(key, nil, ttl)
		return nil
	}

	var indexBuffer bytes.Buffer

	encoder := gob.NewEncoder(&indexBuffer)
	if err := encoder.Encode(*value); err != nil {
		return err
	}
	c.client.SetAsync(key, indexBuffer.Bytes(), ttl)
	return nil
}

func (c *MemcachedCache[valueType]) SetMultiple(values map[string]*valueType, ttl time.Duration) error {
	var (
		firstErr error
		failed   int
	)

	for key, value := range values {
		var indexBuffer bytes.Buffer
		encoder := gob.NewEncoder(&indexBuffer)

		if err := encoder.Encode(*value); err != nil {
			return err
		}

		if err := c.client.SetAsync(key, indexBuffer.Bytes(), ttl); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (c *MemcachedCache[valueType]) Clear(newSize int) error {
	// do nothing here
	return nil
}

func (c *MemcachedCache[valueType]) GetCacheSize() int {
	// do nothing here
	return 0
}

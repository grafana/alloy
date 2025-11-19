// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// readatbuf providers wrappers adding caching to types that implement the `ReaderAt` interface.

package readatbuf // import "go.opentelemetry.io/ebpf-profiler/libpf/readatbuf"

import (
	"errors"
	"fmt"
	"io"

	lru "github.com/elastic/go-freelru"

	"go.opentelemetry.io/ebpf-profiler/libpf/hash"
)

// page represents a cached region from the underlying reader.
type page struct {
	// data contains the data cached from a previous read.
	data []byte
	// eof determines whether we encountered an EOF when reading the page originally.
	eof bool
}

// Statistics contains statistics about cache efficiency.
type Statistics struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
}

// Reader implements buffering for random access reads via the `ReaderAt` interface.
type Reader struct {
	inner        io.ReaderAt
	cache        *lru.LRU[uint, page]
	pageSize     uint
	stats        Statistics
	sparePageBuf []byte
}

func HashUInt(v uint) uint32 {
	return uint32(hash.Uint64(uint64(v)))
}

// New creates a new buffered reader supporting random access. The pageSize argument decides the
// size of each region (page) tracked in the cache. cacheSize defines the maximum number of pages
// to cache.
func New(inner io.ReaderAt, pageSize, cacheSize uint) (reader *Reader, err error) {
	if pageSize == 0 {
		return nil, errors.New("pageSize cannot be zero")
	}
	if cacheSize == 0 {
		return nil, errors.New("cacheSize cannot be zero")
	}

	reader = &Reader{
		inner:    inner,
		pageSize: pageSize,
	}

	reader.cache, err = lru.New[uint, page](uint32(cacheSize), HashUInt)
	if err != nil {
		return nil, fmt.Errorf("failed to create internal cache: %w", err)
	}

	reader.cache.SetOnEvict(func(_ uint, page page) {
		reader.stats.Evictions++
		// For EOF pages, the slice might have been truncated. However, all slices were originally
		// allocated with page size. Thus, we can expand them back to their original size. Perhaps
		// counter-intuitively, Go's slice bounds-checking doesn't limit by the length, but by the
		// capacity.
		reader.sparePageBuf = page.data[:pageSize]
	})

	return
}

// InvalidateCache flushes the internal cache. Resets the statistics.
func (reader *Reader) InvalidateCache() {
	reader.cache.Purge()
	reader.stats = Statistics{}
}

// Statistics returns statistics about cache efficiency.
func (reader *Reader) Statistics() Statistics {
	return reader.stats
}

// ReadAt implements the `ReaderAt` interface.
func (reader *Reader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset value %d given", off)
	}

	// If reading large amounts of data, skip the cache and use inner ReadAt
	// directly. This avoids a single read to trash the whole cache.
	// When using underlying os.File this also reduces the number of syscalls
	// made as the caching logic would split this to ReadAt call per page.
	if uint(len(p)) > reader.pageSize*3/2 {
		return reader.inner.ReadAt(p, off)
	}

	writeOffset := uint(0)
	remaining := uint(len(p))
	skipOffset := uint(off) % reader.pageSize
	pageIdx := uint(off) / reader.pageSize

	for remaining > 0 {
		data, eof, err := reader.getOrReadPage(pageIdx)
		if err != nil {
			return int(writeOffset), err
		}
		if skipOffset > uint(len(data)) {
			return 0, io.EOF
		}

		copyLen := min(remaining, uint(len(data))-skipOffset)
		copy(p[writeOffset:][:copyLen], data[skipOffset:][:copyLen])

		skipOffset = 0
		pageIdx++
		writeOffset += copyLen
		remaining -= copyLen

		if eof {
			if remaining == 0 {
				// While there was an EOF in the chunk read, the user buffer was small enough to
				// not have caused it.
				break
			}

			// The read is incomplete.
			return int(writeOffset), io.EOF
		}
	}

	return int(writeOffset), nil
}

func (reader *Reader) getOrReadPage(pageIdx uint) (data []byte, eof bool, err error) {
	if cachedPage, exists := reader.cache.Get(pageIdx); exists {
		// Data is cached: serve from there.
		reader.stats.Hits++
		return cachedPage.data, cachedPage.eof, nil
	}

	reader.stats.Misses++

	var buffer []byte
	if reader.sparePageBuf != nil {
		// If present, reuse the spare page from previous evictions.
		buffer = reader.sparePageBuf
		reader.sparePageBuf = nil
	} else {
		// Otherwise, allocate a fresh one.
		buffer = make([]byte, reader.pageSize)
	}

	// Read from the underlying reader.
	n, err := reader.inner.ReadAt(buffer, int64(pageIdx*reader.pageSize))
	if err != nil {
		// We speculatively read more than the original caller asked us to, so running into
		// EOF is actually expected for us.
		if err == io.EOF {
			buffer = buffer[:n]
			eof = true
		} else {
			return nil, false, err
		}
	}

	if !eof && uint(n) < reader.pageSize {
		return nil, false, errors.New("failed to read whole page")
	}

	reader.cache.Add(pageIdx, page{data: buffer, eof: eof})
	return buffer, eof, nil
}

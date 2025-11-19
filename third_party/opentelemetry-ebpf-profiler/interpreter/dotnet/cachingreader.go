// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dotnet // import "go.opentelemetry.io/ebpf-profiler/interpreter/dotnet"

import (
	"io"
)

// CachingReader allows reading data from the remote process using io.ReadByte and
// io.Reader interfaces. It provides also simple cache.
type cachingReader struct {
	// r is the ReaderAt from which we are reading the data from
	r io.ReaderAt
	// buf contains all data read from the target process
	buf []byte
	// addr is the target offset to continue reading from
	addr int64
	// i is the index to the buf[] byte which is to be returned next in ReadByte()
	i int
}

// ReadByte implements io.ByteReader interface to do cached single byte reads.
func (cr *cachingReader) ReadByte() (byte, error) {
	// Readahead to buffer if needed
	if cr.i >= len(cr.buf) {
		cr.i = 0
		_, err := cr.r.ReadAt(cr.buf, cr.addr)
		if err != nil {
			return 0, err
		}
		cr.addr += int64(len(cr.buf))
	}
	// Return byte from buffer
	b := cr.buf[cr.i]
	cr.i++
	return b, nil
}

// Skip consumes numBytes without copying them
func (cr *cachingReader) Skip(numBytes int) {
	if cr.i+numBytes < len(cr.buf) {
		cr.i += numBytes
		return
	}
	numBytes -= len(cr.buf) - cr.i
	cr.i = len(cr.buf)
	cr.addr += int64(numBytes)
}

// Read implements io.Reader interface to read from the target.
func (cr *cachingReader) Read(buf []byte) (int, error) {
	offs := 0
	if cr.i < len(cr.buf) {
		// Read from the cache
		cache := cr.buf[cr.i:]
		if len(cache) > len(buf) {
			cache = cache[:len(buf)]
		}
		copy(buf, cache)
		offs = len(cache)
		cr.i += len(cache)
		if len(buf) == len(cache) {
			return offs, nil
		}
	}

	// Satisfy rest of the read directly
	n, err := cr.r.ReadAt(buf[offs:], cr.addr)
	return offs + n, err
}

// Reader returns a cachingReader to read and record data from given start.
func newCachingReader(r io.ReaderAt, addr int64, cacheSize int) *cachingReader {
	return &cachingReader{
		r:    r,
		addr: addr,
		i:    cacheSize,
		buf:  make([]byte, cacheSize),
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot // import "go.opentelemetry.io/ebpf-profiler/interpreter/hotspot"

import (
	"io"
)

// RecordingReader allows reading data from the remote process using io.ReadByte interface.
// It provides basic buffering by reading memory in pieces of 'chunk' bytes and it also
// records all read memory in a backing buffer to be later stored as a whole.
type RecordingReader struct {
	// reader is the ReaderAt from which we are reading the data from
	reader io.ReaderAt
	// buf contains all data read from the target process
	buf []byte
	// offs is the offset to continue reading from
	offs int64
	// i is the index to the buf[] byte which is to be returned next in ReadByte()
	i int
	// chunk is the number of bytes to read from target process when mora data is needed
	chunk int
}

// ReadByte implements io.ByteReader interface to read memory single byte at a time.
func (rr *RecordingReader) ReadByte() (byte, error) {
	// Readahead to buffer if needed
	if rr.i >= len(rr.buf) {
		buf := make([]byte, len(rr.buf)+rr.chunk)
		copy(buf, rr.buf)
		_, err := rr.reader.ReadAt(buf[len(rr.buf):], rr.offs)
		if err != nil {
			return 0, err
		}
		rr.offs += int64(rr.chunk)
		rr.buf = buf
	}
	// Return byte from buffer
	b := rr.buf[rr.i]
	rr.i++
	return b, nil
}

// GetBuffer returns all the data so far as a single slice.
func (rr *RecordingReader) GetBuffer() []byte {
	return rr.buf[0:rr.i]
}

// newRecordingReader returns a RecordingReader to read and record data from given start.
func newRecordingReader(reader io.ReaderAt, offs int64, chunkSize uint) *RecordingReader {
	return &RecordingReader{
		reader: reader,
		offs:   offs,
		chunk:  int(chunkSize),
	}
}

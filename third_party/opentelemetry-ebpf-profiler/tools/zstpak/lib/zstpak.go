// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// zstpak implements reading and writing for an efficiently seekable compressed file format. The
// efficient random access is achieved by compressing the data in small chunks and keeping an index
// of the chunks in a footer. The footer can then be inspected to determine in which chunk the
// required data for any offset is located.
//
// # File format
//
// >>> <compressed data>
// >>> for chunk in number_of_chunks:
// >>>   compressed_data_offset: u64 LE   # offset in compressed data
// >>> number_of_chunks: u64 LE
// >>> decompressed_size: u64 LE
// >>> chunk_size: u64 LE
// >>> magic: [8]char
//
// Using relative offsets and variable size ints in the footer could shave off a few more bytes,
// but was omitted for simplicity.

package zstpak // import "go.opentelemetry.io/ebpf-profiler/tools/zstpak/lib"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

// footerSize is the size of the static portion of the footer (without the index data).
const footerSize = 32

// magic defines the file magic that uniquely identifies zstpak files.
const magic = "ZSTPAK00"

// footer contains the meta-information stored at the end of zstpak files.
type footer struct {
	chunkSize        uint64
	uncompressedSize uint64
	index            []uint64
}

func readFooter(input io.ReaderAt, fileSize uint64) (*footer, error) {
	var buf [footerSize]byte

	if fileSize < footerSize {
		return nil, errors.New("file is too small to be a valid zstpak file")
	}
	if _, err := input.ReadAt(buf[:], int64(fileSize-footerSize)); err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}
	if !bytes.Equal(buf[24:], []byte(magic)) {
		return nil, errors.New("file doesn't appear to be in zstpak format (bad magic)")
	}

	chunkSize := binary.LittleEndian.Uint64(buf[16:])
	uncompressedSize := binary.LittleEndian.Uint64(buf[8:])
	numberOfChunks := binary.LittleEndian.Uint64(buf[0:])

	// Read raw index from file.
	if fileSize < footerSize+numberOfChunks*8 {
		return nil, errors.New("file too small to hold index table")
	}
	rawIndex := make([]byte, numberOfChunks*8)
	indexOffset := fileSize - footerSize - numberOfChunks*8
	if _, err := input.ReadAt(rawIndex, int64(indexOffset)); err != nil {
		return nil, fmt.Errorf("failed to read index from file: %w", err)
	}

	// Convert into array of uint64.
	index := make([]uint64, 0, numberOfChunks)
	for i := range numberOfChunks {
		entry := binary.LittleEndian.Uint64(rawIndex[i*8:])
		if i > 0 && entry < index[i-1] {
			return nil, errors.New("index entries aren't monotonically increasing")
		}
		index = append(index, entry)
	}

	return &footer{
		chunkSize:        chunkSize,
		uncompressedSize: uncompressedSize,
		index:            index,
	}, nil
}

func (ftr *footer) write(out io.Writer) error {
	for _, offset := range ftr.index {
		if err := binary.Write(out, binary.LittleEndian, offset); err != nil {
			return fmt.Errorf("failed to write index entry: %w", err)
		}
	}

	if err := binary.Write(out, binary.LittleEndian, uint64(len(ftr.index))); err != nil {
		return fmt.Errorf("failed to write number of entries: %w", err)
	}
	if err := binary.Write(out, binary.LittleEndian, ftr.uncompressedSize); err != nil {
		return fmt.Errorf("failed to write uncompressed size: %w", err)
	}
	if err := binary.Write(out, binary.LittleEndian, ftr.chunkSize); err != nil {
		return fmt.Errorf("failed to write chunk size: %w", err)
	}
	if _, err := out.Write([]byte(magic)); err != nil {
		return fmt.Errorf("failed to write magic: %w", err)
	}

	return nil
}

// CompressInto reads data from an input reader, writing it out in compressed form. The chunk size
// determines how often to create new chunks. Higher numbers increase compression rates, but come
// at the cost of making random access less efficient.
func CompressInto(in io.Reader, out io.Writer, chunkSize uint64) error {
	readBuf := make([]byte, chunkSize)
	compressBuf := make([]byte, chunkSize)

	// Compress chunks, memorizing their start offsets.
	index := []uint64{0}
	writeOffset := uint64(0)
	uncompressedSize := uint64(0)

	enc, err := zstd.NewWriter(nil)
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}
	for {
		n, err := io.ReadFull(in, readBuf)
		if err != nil {
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				// Last chunk: truncate our buffer and continue. Next read will
				// return EOF and thus break the loop.
				readBuf = readBuf[:n]
			} else {
				return err
			}
		}

		compressed := enc.EncodeAll(readBuf, compressBuf[:0])

		uncompressedSize += uint64(n)
		writeOffset += uint64(len(compressed))
		index = append(index, writeOffset)

		if _, err = out.Write(compressed); err != nil {
			return fmt.Errorf("failed to write compressed data: %w", err)
		}
	}

	// Write footer.
	ftr := footer{
		uncompressedSize: uncompressedSize,
		chunkSize:        chunkSize,
		index:            index,
	}
	return ftr.write(out)
}

// Reader allows random access reads within zstpak files. Created via the `Open` method.
type Reader struct {
	file   *os.File
	footer *footer
}

// Open a zstpak file for random access reading.
func Open(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	hdr, err := readFooter(file, uint64(fileInfo.Size()))
	if err != nil {
		return nil, err
	}

	return &Reader{
		file:   file,
		footer: hdr,
	}, nil
}

// UncompressedSize returns the size of the packed file if it was fully decompressed.
func (reader *Reader) UncompressedSize() uint64 {
	return reader.footer.uncompressedSize
}

// ChunkSize returns the size of the compressed chunks in this file.
func (reader *Reader) ChunkSize() uint64 {
	return reader.footer.chunkSize
}

// Close implements the `Closer` interface.
func (reader *Reader) Close() error {
	return reader.file.Close()
}

// ReadAt implements the `ReaderAt` interface.
func (reader *Reader) ReadAt(p []byte, off int64) (n int, err error) {
	writeOffset := 0
	remaining := len(p)
	chunkIdx := int(off) / int(reader.footer.chunkSize)
	skipOffset := int(off) % int(reader.footer.chunkSize)

	for remaining > 0 {
		if chunkIdx+1 >= len(reader.footer.index) {
			return writeOffset, io.EOF
		}

		// Read compressed chunk from disk.
		compressedChunkStart := reader.footer.index[chunkIdx]
		compressedChunkLen := reader.footer.index[chunkIdx+1] - compressedChunkStart
		decompressed, err := reader.getDecompressedChunk(compressedChunkStart, compressedChunkLen)
		if err != nil {
			return writeOffset, err
		}

		// Copy data to output buffer.
		if skipOffset > len(decompressed) {
			return 0, errors.New("corrupted chunk data")
		}
		copyLen := min(remaining, len(decompressed)-skipOffset)
		copy(p[writeOffset:][:copyLen], decompressed[skipOffset:][:copyLen])

		// Only apply skipping in first iteration.
		skipOffset = 0

		// Adjust offset and capacity.
		writeOffset += copyLen
		remaining -= copyLen
		chunkIdx++
	}

	return writeOffset, nil
}

func (reader *Reader) getDecompressedChunk(start, length uint64) ([]byte, error) {
	compressedChunk := make([]byte, length)
	if _, err := reader.file.ReadAt(compressedChunk, int64(start)); err != nil {
		return nil, fmt.Errorf("failed to read chunk data: %w", err)
	}

	dec, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	decompressed, err := dec.DecodeAll(compressedChunk, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress chunk: %w", err)
	}

	return decompressed, nil
}

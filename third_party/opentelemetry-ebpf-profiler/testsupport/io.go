// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testsupport // import "go.opentelemetry.io/ebpf-profiler/testsupport"

import (
	"bytes"
	"io"
	"math/rand/v2"
	"testing"
)

// ValidateReadAtWrapperTransparency validates that a `ReadAt` implementation provides a
// transparent view into the given reference buffer.
func ValidateReadAtWrapperTransparency(
	t *testing.T, iterations uint, reference []byte, testee io.ReaderAt) {
	bufferSize := uint64(len(reference))

	// Samples random slices to validate within the file.
	r := rand.New(rand.NewPCG(0, 0)) //nolint:gosec
	for range iterations {
		// Intentionally allow slices that over-read the file to test this case.
		length := r.Uint64() % bufferSize
		start := r.Uint64() % bufferSize

		readBuf := make([]byte, length)
		n, err := testee.ReadAt(readBuf, int64(start))

		truncReadLen := min(bufferSize-start, length)
		if truncReadLen != length {
			// If we asked to read more than the file has, we expect a truncated read.
			if err != io.EOF {
				t.Fatalf("expected an EOF error")
			}
			if uint64(n) != truncReadLen {
				t.Fatalf("expected truncation to %d, but got %d", truncReadLen, n)
			}
		} else {
			// Otherwise, we expect a full read.
			if uint64(n) != length {
				t.Fatalf("read length mismatch (%v vs %v)", n, length)
			}
			if err != nil {
				t.Fatalf("failed to read: %v", err)
			}
		}

		got := readBuf[:truncReadLen]
		expected := reference[start:][:truncReadLen]
		if !bytes.Equal(got, expected) {
			t.Fatalf("data mismatch: got %v, expected %v", got, expected)
		}
	}
}

// GenerateTestInputFile generates a test input file, repeating a number sequence over and over.
func GenerateTestInputFile(seqLen uint8, outputSize uint) []byte {
	out := make([]byte, 0, outputSize)
	for i := range outputSize {
		out = append(out, byte(i%uint(seqLen)))
	}

	return out
}

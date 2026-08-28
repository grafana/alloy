package tail

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// signature represents the first N bytes of a file, used to detect atomic writes
// where a file is replaced but may have the same initial content.
type signature struct {
	d []byte
}

// signatureSize is the target size for a complete signature.
const signatureSize = 1024

// signatureThresholds defines the byte offsets at which we should recompute the signature
// as the file grows. This allows us to progressively build a more complete signature.
var signatureThresholds = []int{64, 128, 256, 512, signatureSize}

// newSignatureFromFile reads up to signatureSize bytes from the beginning of the file
// to create a signature. If the file is smaller, the signature will be incomplete.
func newSignatureFromFile(f *os.File) (*signature, error) {
	buf := make([]byte, signatureSize)
	n, err := f.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to compute signature for file: %w", err)
	}
	return &signature{
		d: buf[:n],
	}, nil
}

// completed returns true if the signature has reached the target size.
func (s *signature) completed() bool {
	return len(s.d) == signatureSize
}

// equal compares two signatures. For incomplete signatures, it compares only
// the overlapping bytes. For complete signatures, both must be the same length and content.
// If signature has a length of 0 equal will always return false.
func (s *signature) equal(other *signature) bool {
	len1 := len(s.d)
	if len1 == 0 {
		return false
	}

	len2 := len(other.d)
	if !s.completed() {
		if len1 > len2 {
			return false
		}
		return bytes.Equal(s.d[:len1], other.d[:len1])
	}

	return len1 == len2 && bytes.Equal(s.d, other.d)
}

// shouldRecompute returns true if we have read past a signature threshold and
// the current signature is smaller than that threshold. This allows us to
// progressively update the signature as the file grows.
func (s *signature) shouldRecompute(at int64) bool {
	if s.completed() {
		return false
	}

	currentSize := len(s.d)
	for _, threshold := range signatureThresholds {
		if at >= int64(threshold) && currentSize < threshold {
			return true
		}
	}

	return false
}

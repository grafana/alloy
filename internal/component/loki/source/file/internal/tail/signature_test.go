package tail

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignature(t *testing.T) {
	verifyFilePosition := func(f *os.File, expected int64) {
		offset, err := f.Seek(0, io.SeekCurrent)
		require.NoError(t, err)
		require.Equal(t, expected, offset)
	}

	t.Run("newSignatureFromFile with empty file", func(t *testing.T) {
		f := createEmptyFile(t, "empty")
		defer f.Close()
		sig, err := newSignatureFromFile(f)
		require.NoError(t, err)
		require.Equal(t, 0, len(sig.d))
		require.False(t, sig.completed())
		verifyFilePosition(f, 0)
	})

	t.Run("newSignatureFromFile with small file", func(t *testing.T) {
		content := []byte("hello world")
		f := createFileWithContent(t, "small", string(content))
		defer f.Close()

		sig, err := newSignatureFromFile(f)
		require.NoError(t, err)
		require.Equal(t, len(content), len(sig.d))
		require.Equal(t, content, sig.d)
		require.False(t, sig.completed())
		verifyFilePosition(f, 0)
	})

	t.Run("newSignatureFromFile with exactly 512 bytes", func(t *testing.T) {
		content := make([]byte, signatureSize)
		for i := range content {
			content[i] = byte(i % 256)
		}
		f := createFileWithContent(t, "exact512", string(content))
		defer f.Close()

		sig, err := newSignatureFromFile(f)
		require.NoError(t, err)
		require.Equal(t, signatureSize, len(sig.d))
		require.Equal(t, content, sig.d)
		require.True(t, sig.completed())
		verifyFilePosition(f, 0)
	})

	t.Run("newSignatureFromFile with large file", func(t *testing.T) {
		content := make([]byte, signatureSize*2)
		for i := range content {
			content[i] = byte(i % 256)
		}
		f := createFileWithContent(t, "large", string(content))
		defer f.Close()

		sig, err := newSignatureFromFile(f)
		require.NoError(t, err)
		require.Equal(t, signatureSize, len(sig.d))
		require.Equal(t, content[:signatureSize], sig.d)
		require.True(t, sig.completed())
		verifyFilePosition(f, 0)
	})

	t.Run("completed returns false for incomplete signature", func(t *testing.T) {
		sig := &signature{d: []byte("small")}
		require.False(t, sig.completed())
	})

	t.Run("completed returns true for complete signature", func(t *testing.T) {
		sig := &signature{d: make([]byte, signatureSize)}
		require.True(t, sig.completed())
	})

	t.Run("equal with identical complete signatures", func(t *testing.T) {
		data := make([]byte, signatureSize)
		for i := range data {
			data[i] = byte(i % 256)
		}
		sig1 := &signature{d: data}
		sig2 := &signature{d: append([]byte(nil), data...)}
		require.True(t, sig1.equal(sig2))
		require.True(t, sig2.equal(sig1))
	})

	t.Run("equal with different complete signatures", func(t *testing.T) {
		data1 := make([]byte, signatureSize)
		data2 := make([]byte, signatureSize)
		copy(data2, data1)
		data2[0] = 0xFF
		sig1 := &signature{d: data1}
		sig2 := &signature{d: data2}
		require.False(t, sig1.equal(sig2))
		require.False(t, sig2.equal(sig1))
	})

	t.Run("equal with identical incomplete signatures", func(t *testing.T) {
		data := []byte("hello")
		sig1 := &signature{d: data}
		sig2 := &signature{d: append([]byte(nil), data...)}
		require.True(t, sig1.equal(sig2))
		require.True(t, sig2.equal(sig1))
	})

	t.Run("equal with different incomplete signatures", func(t *testing.T) {
		sig1 := &signature{d: []byte("hello")}
		sig2 := &signature{d: []byte("world")}
		require.False(t, sig1.equal(sig2))
		require.False(t, sig2.equal(sig1))
	})

	t.Run("equal with incomplete signature longer than other", func(t *testing.T) {
		sig1 := &signature{d: []byte("hello world")}
		sig2 := &signature{d: []byte("hello")}
		require.False(t, sig1.equal(sig2))
	})

	t.Run("equal with incomplete shorter than complete with same prefix", func(t *testing.T) {
		prefix := []byte("hello")
		incomplete := &signature{d: prefix}
		complete := &signature{d: append(prefix, make([]byte, signatureSize-len(prefix))...)}
		require.True(t, incomplete.equal(complete))
	})

	t.Run("equal with incomplete shorter than complete with different prefix", func(t *testing.T) {
		incomplete := &signature{d: []byte("hello")}
		completeData := make([]byte, signatureSize)
		copy(completeData, []byte("world"))
		complete := &signature{d: completeData}
		require.False(t, incomplete.equal(complete))
	})

	t.Run("equal with different length incomplete signatures", func(t *testing.T) {
		sig1 := &signature{d: []byte("hello")}
		sig2 := &signature{d: []byte("hi")}
		require.False(t, sig1.equal(sig2))
	})

	t.Run("shouldRecompute returns false for completed signature", func(t *testing.T) {
		sig := &signature{d: make([]byte, signatureSize)}
		require.False(t, sig.shouldRecompute(2000))
		require.False(t, sig.shouldRecompute(100))
	})

	t.Run("shouldRecompute returns false when position is before any threshold", func(t *testing.T) {
		sig := &signature{d: []byte("small")}
		require.False(t, sig.shouldRecompute(10))
		require.False(t, sig.shouldRecompute(63))
	})

	t.Run("shouldRecompute returns false when signature already at threshold", func(t *testing.T) {
		sig := &signature{d: make([]byte, 64)}
		require.False(t, sig.shouldRecompute(64))
	})

	t.Run("shouldRecompute returns true when position crosses threshold and signature is smaller", func(t *testing.T) {
		sig := &signature{d: []byte("small")}
		require.True(t, sig.shouldRecompute(64))
		require.True(t, sig.shouldRecompute(65))
		require.True(t, sig.shouldRecompute(128))
	})

	t.Run("shouldRecompute returns true for threshold 256 when signature is 128 bytes", func(t *testing.T) {
		sig := &signature{d: make([]byte, 128)}
		require.False(t, sig.shouldRecompute(128))
		require.True(t, sig.shouldRecompute(256))
	})

	t.Run("shouldRecompute returns true for threshold 1024 when signature is 512 bytes", func(t *testing.T) {
		sig := &signature{d: make([]byte, 512)}
		require.False(t, sig.shouldRecompute(512))
		require.True(t, sig.shouldRecompute(1024))
	})
}

package tail

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRingBuffer(t *testing.T) {
	t.Run("new buffer is empty", func(t *testing.T) {
		rb := newRingBuffer(8)
		require.Equal(t, 0, rb.Len())
		require.Nil(t, rb.Bytes())
	})

	t.Run("append and bytes", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hello"))
		require.Equal(t, 5, rb.Len())
		require.True(t, bytes.Equal([]byte("hello"), rb.Bytes()))
	})

	t.Run("advance", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hello"))
		rb.Advance(2)
		require.Equal(t, 3, rb.Len())
		require.True(t, bytes.Equal([]byte("llo"), rb.Bytes()))
	})

	t.Run("advance all", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hello"))
		rb.Advance(5)
		require.Equal(t, 0, rb.Len())
		require.Nil(t, rb.Bytes())
	})

	t.Run("advance more than cap", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hi"))
		rb.Advance(10)
		require.Equal(t, 0, rb.Len())
		require.Nil(t, rb.Bytes())
	})

	t.Run("append after advance", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hello"))
		rb.Advance(2)
		rb.Append([]byte("world"))
		require.Equal(t, 8, rb.Len())
		require.True(t, bytes.Equal([]byte("lloworld"), rb.Bytes()))
	})

	t.Run("grow when full", func(t *testing.T) {
		rb := newRingBuffer(4)
		rb.Append([]byte("abcd"))
		require.Equal(t, 4, rb.Len())
		rb.Append([]byte("e"))
		require.Equal(t, 5, rb.Len())
		require.True(t, bytes.Equal([]byte("abcde"), rb.Bytes()))
	})

	t.Run("bytes when wrapped", func(t *testing.T) {
		rb := newRingBuffer(4)
		rb.Append([]byte("ab"))
		rb.Advance(2)              // head=2, tail=2
		rb.Append([]byte("cdef")) // wraps: tail goes to 2 (writes at 2,3,0,1)
		require.Equal(t, 4, rb.Len())
		require.True(t, bytes.Equal([]byte("cdef"), rb.Bytes()))
	})

	t.Run("advance when at end of buffer", func(t *testing.T) {
		// Regression: advance exactly to end (head=len) must not wrap head to 0.
		rb := newRingBuffer(4)
		rb.Append([]byte("abcd"))
		rb.Advance(4)
		require.Equal(t, 0, rb.Len())
		rb.Append([]byte("x"))
		require.Equal(t, 1, rb.Len())
		require.True(t, bytes.Equal([]byte("x"), rb.Bytes()))
	})

	t.Run("reset", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hello"))
		rb.Reset()
		require.Equal(t, 0, rb.Len())
		require.Nil(t, rb.Bytes())
		rb.Append([]byte("world"))
		require.Equal(t, 5, rb.Len())
		require.True(t, bytes.Equal([]byte("world"), rb.Bytes()))
	})

	t.Run("append empty is no-op", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hi"))
		rb.Append(nil)
		rb.Append([]byte{})
		require.Equal(t, 2, rb.Len())
		require.True(t, bytes.Equal([]byte("hi"), rb.Bytes()))
	})

	t.Run("advance zero or negative is no-op", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Append([]byte("hello"))
		rb.Advance(0)
		require.Equal(t, 5, rb.Len())
		rb.Advance(-1)
		require.Equal(t, 5, rb.Len())
	})
}

package filequeue

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileQueue(t *testing.T) {
	dir := t.TempDir()
	q, err := newFileQueue(dir, "test")
	require.NoError(t, err)
	handle, err := q.Add([]byte("test"))
	require.NoError(t, err)
	require.True(t, handle != "")
	data := make([]byte, 0)

	data, name, found, more := q.Next(data)
	require.True(t, found)
	require.False(t, more)
	require.True(t, string(data) == "test")

	q.Delete(name)

	data, name, found, more = q.Next(data)
	require.False(t, found)
	require.False(t, more)
	require.True(t, len(data) == 0)
	require.True(t, name == "")
}

func TestFileQueueMultiple(t *testing.T) {
	dir := t.TempDir()
	q, err := newFileQueue(dir, "test")
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		handle, err := q.Add([]byte(fmt.Sprintf("%d test", i)))
		require.NoError(t, err)
		require.True(t, handle != "")
	}

	for i := 0; i < 3; i++ {
		data := make([]byte, 0)
		data, name, _, _ := q.Next(data)
		require.True(t, string(data) == fmt.Sprintf("%d test", i))
		q.Delete(name)
	}
}

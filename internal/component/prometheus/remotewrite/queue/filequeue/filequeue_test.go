package filequeue

import (
	"context"
	"github.com/go-kit/log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileQueue(t *testing.T) {
	dir := t.TempDir()
	log := log.NewNopLogger()
	q, err := NewQueue(dir, log)
	require.NoError(t, err)
	handle, err := q.Add([]byte("test"))
	require.NoError(t, err)
	require.True(t, handle != "")

	ctx := context.Background()
	buf := make([]byte, 0)
	buf, name, err := q.Next(ctx, buf)
	require.NoError(t, err)
	require.True(t, name != "")
	require.True(t, string(buf) == "test")

	q.Delete(name)

	ctx, cncl := context.WithTimeout(ctx, 1*time.Second)
	defer cncl()
	buf, name, err = q.Next(ctx, buf)
	require.Error(t, err)
	require.True(t, len(buf) == 0)
	require.True(t, name == "")
}

//go:build linux

package tail

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/stretchr/testify/require"
)

// TestFileRecreatedAtPath reads a deleted and recreated file at the same path
// from the beginning rather than resuming at the previous read offset.
func TestFileRecreatedAtPath(t *testing.T) {
	const original = "first-generation\n"
	const replacement = "second-generation\n"

	name := createFile(t, "recreated", original)

	f, err := NewFile(logging.NewSlogNop(), &Config{
		Filename: name,
		WatcherConfig: WatcherConfig{
			MinPollFrequency: 200 * time.Millisecond,
			MaxPollFrequency: 200 * time.Millisecond,
		},
	})
	require.NoError(t, err)
	defer f.Close()

	verifyResult(t, f, &Line{Text: "first-generation", Offset: 17}, nil)
	verifyResult(t, f, nil, io.EOF)

	go func() {
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, os.Remove(name))
		require.Eventually(t, func() bool {
			return !pathHeld(name)
		}, 5*time.Second, 5*time.Millisecond)
		require.NoError(t, os.WriteFile(name, []byte(replacement), 0o600))
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, f.Wait(ctx))
	verifyResult(t, f, &Line{Text: "second-generation", Offset: 18}, nil)
	verifyResult(t, f, nil, io.EOF)
}

// pathHeld reports whether any file descriptor in this process still refers
// to path, including a handle kept open after the directory entry was removed.
func pathHeld(path string) bool {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return true
	}
	deleted := path + " (deleted)"
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err != nil {
			continue
		}
		if target == path || target == deleted {
			return true
		}
	}
	return false
}

//go:build linux

package tail

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/stretchr/testify/require"
)

// TestFileRecreatedWithReusedInode deletes a file that has been read to the
// end and creates a new one at the same path after the tailer has closed its
// handle. On ext4 that creation often recycles the inode, so os.SameFile
// alone cannot tell the new file from the old one.
func TestFileRecreatedWithReusedInode(t *testing.T) {
	const original = "same-prefix\nold-tail\n"
	oldMod := time.Unix(1_700_000_000, 0)

	tests := []struct {
		name      string
		recreated string
		mtime     time.Time
		want      []string
	}{
		{
			// Size is below the offset already consumed. The mtime is restored
			// so detection cannot rely on the modification time.
			name:      "shrinks below the consumed offset",
			recreated: "same-prefix\n",
			mtime:     oldMod,
			want:      []string{"same-prefix"},
		},
		{
			// The new file is already larger than the old offset and starts
			// with the same bytes, so a content signature would resume at the
			// previous offset and drop the prefix. The mtime is what shows
			// the inode was reused.
			name:      "grows past the consumed offset with a new mtime",
			recreated: "same-prefix\nold-tail\nbrand-new\n",
			mtime:     oldMod.Add(time.Second),
			want:      []string{"same-prefix", "old-tail", "brand-new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := createFile(t, "reused", original)
			require.NoError(t, os.Chtimes(name, oldMod, oldMod))
			oldIno := inodeOf(t, name)

			f, err := NewFile(logging.NewSlogNop(), &Config{
				Filename: name,
				WatcherConfig: WatcherConfig{
					MinPollFrequency: 200 * time.Millisecond,
					MaxPollFrequency: 200 * time.Millisecond,
				},
			})
			require.NoError(t, err)
			defer f.Close()

			require.Equal(t, []string{"same-prefix", "old-tail"}, readLinesUntilEOF(t, f))

			ino := make(chan uint64, 1)
			go recreateAfterClose(t, name, tt.recreated, tt.mtime, ino)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			require.NoError(t, f.Wait(ctx))
			require.Equal(t, oldIno, <-ino, "filesystem did not reuse the inode; os.SameFile branch was not exercised")
			require.Equal(t, tt.want, readLinesUntilEOF(t, f))
		})
	}
}

// recreateAfterClose removes name once tailing is in progress, waits until
// the tailer closes the deleted file, then creates the replacement. The
// pause after close leaves reopen blocked in its poll so the new mtime is
// set before the replacement is observed.
func recreateAfterClose(t *testing.T, name, content string, mtime time.Time, ino chan<- uint64) {
	t.Helper()
	fail := func(err error) {
		t.Errorf("%v", err)
		ino <- 0
	}

	time.Sleep(50 * time.Millisecond)
	if err := os.Remove(name); err != nil {
		fail(err)
		return
	}

	deadline := time.Now().Add(5 * time.Second)
	for pathHeld(name) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if pathHeld(name) {
		fail(fmt.Errorf("tailed file %s was not closed", name))
		return
	}
	// reopen polls every 200ms. Wait until that poll is in progress so
	// WriteFile and Chtimes both finish before the new file is stat'd.
	time.Sleep(40 * time.Millisecond)

	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		fail(err)
		return
	}
	if err := os.Chtimes(name, mtime, mtime); err != nil {
		fail(err)
		return
	}

	got, err := inodeOfPath(name)
	if err != nil {
		fail(err)
		return
	}
	ino <- got
}

func readLinesUntilEOF(t *testing.T, f *File) []string {
	t.Helper()
	var lines []string
	for {
		line, err := f.Next()
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			return lines
		}
		lines = append(lines, line.Text)
	}
}

func inodeOf(t *testing.T, path string) uint64 {
	t.Helper()
	ino, err := inodeOfPath(path)
	require.NoError(t, err)
	return ino
}

func inodeOfPath(path string) (uint64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, os.ErrInvalid
	}
	return st.Ino, nil
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

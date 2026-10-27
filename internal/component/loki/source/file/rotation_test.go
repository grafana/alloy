package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/componenttest"
)

const (
	rotationWriters       = 3
	rotationLinesPerFile  = 50
	rotationGenerations   = 4
	rotationWriteInterval = 5 * time.Millisecond
)

// rotateFunc rotates the file a writer writes to. It returns the file and path
// to write the next generation to.
type rotateFunc func(t *testing.T, w *rotationWriter) (*os.File, string)

// TestFileRotation runs the component while several writers write to their own
// files and rotate them under each common strategy. Every line must arrive
// exactly once, labeled with the file it was written to. The tail package
// tests cover each rotation event on its own; this covers them under
// concurrent writes.
func TestFileRotation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the strategies rename and delete files while they are tailed, which behaves differently on Windows")
	}

	strategies := []struct {
		name   string
		rotate rotateFunc
		// maxLost is how many lines may be lost. Duplicated, mislabeled or
		// unexpected lines fail the test regardless.
		maxLost int
		// skip, when set, is why the strategy is skipped.
		skip string
		// syncPeriod overrides the file-match interval. Zero keeps the short
		// default used to notice new files quickly.
		syncPeriod time.Duration
	}{
		{
			// Move the file aside and start a new one at the same path.
			name: "rename",
			// FIXME(https://github.com/grafana/alloy/issues/7372): lines written
			// just before the rename are sometimes lost, two or three per run.
			// Set this back to 0 once fixed.
			maxLost: 5,
			rotate: func(t *testing.T, w *rotationWriter) (*os.File, string) {
				require.NoError(t, w.file.Close())
				require.NoError(t, os.Rename(w.path, fmt.Sprintf("%s.%d", w.path, w.generation)))
				return openForAppend(t, w.path), w.path
			},
		},
		{
			// Delete the file and recreate it once the tailer has closed it,
			// as a log rotator would. That is when the new file can reuse
			// the inode. File matching stays quiet so the tailer is not
			// stopped while the path is absent. Every line must arrive
			// (issue 7510).
			name:       "delete",
			syncPeriod: time.Minute,
			rotate: func(t *testing.T, w *rotationWriter) (*os.File, string) {
				require.NoError(t, w.file.Close())
				require.NoError(t, os.Remove(w.path))
				// Recreate only after the tailer closes the deleted file.
				// That is the moment a new file can reuse the inode.
				require.Eventually(t, func() bool {
					return !processHoldsPath(w.path)
				}, 2*time.Second, 5*time.Millisecond)
				return openForAppend(t, w.path), w.path
			},
		},
		{
			// Copy the content aside and truncate the file in place.
			// Truncating loses any line the component has not read yet, which
			// is inherent to the strategy, so wait until it has read them all.
			name: "copytruncate",
			rotate: func(t *testing.T, w *rotationWriter) (*os.File, string) {
				dst := openForAppend(t, fmt.Sprintf("%s.%d", w.path, w.generation))
				_, err := w.file.Seek(0, io.SeekStart)
				require.NoError(t, err)
				_, err = io.Copy(dst, w.file)
				require.NoError(t, err)
				require.NoError(t, dst.Close())

				w.waitForWritten(t)
				require.NoError(t, w.file.Truncate(0))
				_, err = w.file.Seek(0, io.SeekStart)
				require.NoError(t, err)
				return w.file, w.path
			},
		},
		{
			// Leave the file and write the next generation to a new file that
			// the target's glob also matches.
			name: "new",
			rotate: func(t *testing.T, w *rotationWriter) (*os.File, string) {
				require.NoError(t, w.file.Close())
				path := filepath.Join(filepath.Dir(w.path), fmt.Sprintf("%d-%d.log", w.id, w.generation+1))
				return openForAppend(t, path), path
			},
		},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			if s.skip != "" {
				t.Skip(s.skip)
			}
			runRotationTest(t, s.rotate, s.maxLost, s.syncPeriod)
		})
	}
}

func runRotationTest(t *testing.T, rotate rotateFunc, maxLost int, syncPeriod time.Duration) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(componenttest.TestContext(t))
	defer cancel()

	// A long sync period would miss files created after startup. Create the
	// first generation up front so the initial schedule tails them.
	if syncPeriod > 0 {
		for i := range rotationWriters {
			f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%d.log", i)), os.O_CREATE|os.O_WRONLY, 0o644)
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}
	} else {
		syncPeriod = 100 * time.Millisecond
	}

	ctrl, err := componenttest.NewControllerFromID(nil, "loki.source.file")
	require.NoError(t, err)

	receiver := loki.NewLogsReceiver()
	received := newRotationCollector()
	go received.collect(ctx, receiver)

	runErr := make(chan error, 1)
	go func() {
		runErr <- ctrl.Run(ctx, Arguments{
			Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
				"__path__": filepath.Join(dir, "*.log"),
			})},
			ForwardTo: []loki.LogsReceiver{receiver},
			// The new-file strategy needs a short period to notice new paths.
			// Delete passes a long period so the tailer stays up while the
			// path is missing.
			FileMatch: FileMatch{Enabled: true, SyncPeriod: syncPeriod},
			FileWatch: FileWatch{MinPollFrequency: 10 * time.Millisecond, MaxPollFrequency: 50 * time.Millisecond},
		})
	}()
	require.NoError(t, ctrl.WaitRunning(time.Minute))

	// Each writer runs as a parallel subtest, so it can fail the test from its
	// own goroutine. The group returns once all of them have finished.
	writers := make([]*rotationWriter, rotationWriters)
	t.Run("writers", func(t *testing.T) {
		for i := range writers {
			path := filepath.Join(dir, fmt.Sprintf("%d.log", i))
			writers[i] = &rotationWriter{id: i, path: path, file: openForAppend(t, path), generation: 1, received: received}
			t.Run(fmt.Sprintf("writer %d", i), func(t *testing.T) {
				t.Parallel()
				writers[i].run(t, rotate)
			})
		}
	})
	if t.Failed() {
		return
	}

	want := make(map[string]string)
	for _, w := range writers {
		require.NoError(t, w.file.Close())
		for line, path := range w.written {
			want[line] = path
		}
	}
	require.Len(t, want, rotationWriters*rotationLinesPerFile*rotationGenerations)

	// Wait for every line that must arrive, then give late lines and
	// duplicates a moment to show up.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.GreaterOrEqual(c, received.total(), len(want)-maxLost)
	}, 30*time.Second, 50*time.Millisecond)
	time.Sleep(500 * time.Millisecond)

	got := received.snapshot()
	var lost []string
	for line, path := range want {
		paths, ok := got[line]
		if !ok {
			lost = append(lost, line)
			continue
		}
		if assert.Len(t, paths, 1, "line %q should arrive exactly once", line) {
			assert.Equal(t, path, paths[0], "line %q should be labeled with the file it was written to", line)
		}
	}
	assert.LessOrEqual(t, len(lost), maxLost, "lost lines: %v", lost)
	if len(lost) > 0 {
		t.Logf("lost %d of %d lines, within the allowed %d: %v", len(lost), len(want), maxLost, lost)
	}
	for line := range got {
		_, ok := want[line]
		assert.True(t, ok, "line %q was not written by any writer", line)
	}

	cancel()
	require.NoError(t, <-runErr)
}

// rotationWriter writes numbered lines to a file and rotates it every
// rotationLinesPerFile lines.
type rotationWriter struct {
	id         int
	path       string
	file       *os.File
	generation int
	received   *rotationCollector

	// written maps each line to the file it was written to, and current lists
	// the lines written since the last rotation.
	written map[string]string
	current []string
}

func (w *rotationWriter) run(t *testing.T, rotate rotateFunc) {
	w.written = make(map[string]string)
	for {
		for n := range rotationLinesPerFile {
			line := fmt.Sprintf("writer=%d generation=%d line=%d", w.id, w.generation, n)
			_, err := fmt.Fprintln(w.file, line)
			require.NoError(t, err)
			w.written[line] = w.path
			w.current = append(w.current, line)

			// Rotating before the component has found the file would only
			// test discovery, so wait for the first line once.
			if w.generation == 1 && n == 0 {
				w.waitForWritten(t)
			}
			time.Sleep(rotationWriteInterval)
		}

		if w.generation == rotationGenerations {
			return
		}
		w.file, w.path = rotate(t, w)
		w.generation++
		w.current = nil
	}
}

// waitForWritten waits until the component has read every line written since
// the last rotation.
func (w *rotationWriter) waitForWritten(t *testing.T) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, line := range w.current {
			assert.True(c, w.received.has(line), "line %q not received yet", line)
		}
	}, 10*time.Second, 10*time.Millisecond)
}

// rotationCollector records every received line with the filename label of
// each copy of it.
type rotationCollector struct {
	mut   sync.Mutex
	lines map[string][]string
	count int
}

func newRotationCollector() *rotationCollector {
	return &rotationCollector{lines: make(map[string][]string)}
}

func (c *rotationCollector) collect(ctx context.Context, receiver loki.LogsReceiver) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-receiver.Chan():
			c.mut.Lock()
			c.lines[e.Line] = append(c.lines[e.Line], string(e.Labels["filename"]))
			c.count++
			c.mut.Unlock()
		}
	}
}

func (c *rotationCollector) has(line string) bool {
	c.mut.Lock()
	defer c.mut.Unlock()
	_, ok := c.lines[line]
	return ok
}

func (c *rotationCollector) total() int {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.count
}

func (c *rotationCollector) snapshot() map[string][]string {
	c.mut.Lock()
	defer c.mut.Unlock()
	out := make(map[string][]string, len(c.lines))
	for line, paths := range c.lines {
		out[line] = append([]string(nil), paths...)
	}
	return out
}

// processHoldsPath reports whether this process still has path open.
// A deleted path shows up with a " (deleted)" suffix. On systems without
// /proc, it reports false so callers do not wait forever.
func processHoldsPath(path string) bool {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return false
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

func openForAppend(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	require.NoError(t, err)
	return f
}

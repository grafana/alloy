package gather

import (
	"bytes"
	"context"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

func TestPprofWindowGathererCPUInUse(t *testing.T) {
	// Simulate CPU profiling already active elsewhere (e.g. the pprof extension).
	var outer bytes.Buffer
	require.NoError(t, pprof.StartCPUProfile(&outer))
	defer pprof.StopCPUProfile()

	// Start must not fail; it records the CPU error and still collects the rest.
	finish, err := PprofWindow{}.Start(context.Background(), Options{Duration: 50 * time.Millisecond})
	require.NoError(t, err)
	require.NotNil(t, finish)

	files, ferr := finish(context.Background())
	require.Error(t, ferr) // the CPU-in-use error is surfaced

	m := gatherToMap(t, files)
	require.NotContains(t, m, "pprof/cpu.pprof") // CPU dropped
	require.Contains(t, m, "pprof/mutex.pprof")  // mutex still collected
	require.Contains(t, m, "pprof/block.pprof")  // block still collected
}

func gatherToMap(t *testing.T, files []File) map[string][]byte {
	t.Helper()
	m := make(map[string][]byte, len(files))
	for _, f := range files {
		m[f.Path] = f.Content
	}
	return m
}

func TestPprofSnapshotGatherer(t *testing.T) {
	files, err := PprofSnapshot{}.Gather(context.Background(), Options{})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "pprof/heap.pprof")
	require.Contains(t, m, "pprof/goroutine.pprof")
	require.NotContains(t, m, "pprof/cpu.pprof")
	require.NotContains(t, m, "pprof/mutex.pprof")
	require.NotContains(t, m, "pprof/block.pprof")

	require.NotEmpty(t, m["pprof/heap.pprof"])
	require.NotEmpty(t, m["pprof/goroutine.pprof"])

	_, err = profile.ParseData(m["pprof/heap.pprof"])
	require.NoError(t, err)
	_, err = profile.ParseData(m["pprof/goroutine.pprof"])
	require.NoError(t, err)
}

func TestPprofWindowGatherer(t *testing.T) {
	finish, err := PprofWindow{}.Start(context.Background(), Options{Duration: 50 * time.Millisecond})
	require.NoError(t, err)
	require.NotNil(t, finish)

	// Simulate the shared window that the orchestrator waits for.
	time.Sleep(50 * time.Millisecond)

	files, err := finish(context.Background())
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "pprof/cpu.pprof")
	require.Contains(t, m, "pprof/mutex.pprof")
	require.Contains(t, m, "pprof/block.pprof")

	// The profiles may hold zero samples, but they are always valid.
	_, err = profile.ParseData(m["pprof/cpu.pprof"])
	require.NoError(t, err)
	_, err = profile.ParseData(m["pprof/mutex.pprof"])
	require.NoError(t, err)
	_, err = profile.ParseData(m["pprof/block.pprof"])
	require.NoError(t, err)
}

func TestPprofWindowGathererZeroWindow(t *testing.T) {
	// A zero window skips the CPU profile but still emits mutex and block.
	finish, err := PprofWindow{}.Start(context.Background(), Options{Duration: 0})
	require.NoError(t, err)
	require.NotNil(t, finish)

	files, err := finish(context.Background())
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.NotContains(t, m, "pprof/cpu.pprof")
	require.Contains(t, m, "pprof/mutex.pprof")
	require.Contains(t, m, "pprof/block.pprof")

	_, err = profile.ParseData(m["pprof/mutex.pprof"])
	require.NoError(t, err)
	_, err = profile.ParseData(m["pprof/block.pprof"])
	require.NoError(t, err)
}

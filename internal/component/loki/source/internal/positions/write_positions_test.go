package positions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util/atomicfile"
)

func testEntries() map[Entry]string {
	return map[Entry]string{
		{Path: "/var/log/one.log", Labels: "{}"}: "17",
		{Path: "/var/log/two.log", Labels: "{}"}: "42",
	}
}

func TestWritePositionFileRoundTrip(t *testing.T) {
	target := filepath.Join(t.TempDir(), "positions.yml")

	require.NoError(t, writePositionFile(target, testEntries()))

	got, err := readPositionsFile(Config{PositionsFile: target}, logging.NewSlogNop())
	require.NoError(t, err)
	require.Equal(t, testEntries(), got)
}

// The positions file is rewritten on every sync period, so the temporary file
// it is written through has to reuse one name. A name that is unique per write
// leaves the kernel with a directory entry per write, which under cgroup v2 is
// charged to the container and counts towards its memory limit.
// See https://github.com/grafana/alloy/issues/6938.
//
// Putting a directory where the temporary file belongs pins the path: an
// implementation that picked a random name would not touch it, and the write
// would succeed.
func TestWritePositionFileUsesOneTempPath(t *testing.T) {
	target := filepath.Join(t.TempDir(), "positions.yml")
	require.NoError(t, writePositionFile(target, testEntries()))

	require.NoError(t, os.Mkdir(atomicfile.TempPath(target), 0700))

	err := writePositionFile(target, map[Entry]string{{Path: "/var/log/three.log", Labels: "{}"}: "99"})
	require.Error(t, err)
	require.ErrorContains(t, err, atomicfile.TempPath(target))

	got, err := readPositionsFile(Config{PositionsFile: target}, logging.NewSlogNop())
	require.NoError(t, err)
	require.Equal(t, testEntries(), got, "a failed write must leave the recorded offsets alone")
}

// Repeated writes, as the sync period produces, must not accumulate files in
// the positions directory.
func TestWritePositionFileRepeatedWrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "positions.yml")

	for i := 0; i < 5; i++ {
		require.NoError(t, writePositionFile(target, testEntries()))
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "positions.yml", entries[0].Name())

	got, err := readPositionsFile(Config{PositionsFile: target}, logging.NewSlogNop())
	require.NoError(t, err)
	require.Equal(t, testEntries(), got)
}

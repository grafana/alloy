package positions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestLegacyConversion(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := writeLegacy(t, tmpDir)
	positionsPath := filepath.Join(tmpDir, "positions")
	ConvertLegacyPositionsFile(legacy, positionsPath, log.NewNopLogger())
	ps, err := readPositionsFile(positionsPath)
	require.NoError(t, err)
	require.Len(t, ps, 1)
	for k, v := range ps {
		require.True(t, k.Path == "/tmp/random.log")
		require.True(t, v == "17623")
	}
}

func TestLegacyConversionWithNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := writeLegacy(t, tmpDir)
	// Write a new file.
	positionsPath := filepath.Join(tmpDir, "positions")
	err := writePositionFile(positionsPath, map[Entry]string{
		{Path: "/tmp/newrandom.log", Labels: ""}: "100",
	})
	require.NoError(t, err)

	// In this state nothing should be overwritten.
	ConvertLegacyPositionsFile(legacy, positionsPath, log.NewNopLogger())
	ps, err := readPositionsFile(positionsPath)
	require.NoError(t, err)
	require.Len(t, ps, 1)
	for k, v := range ps {
		require.True(t, k.Path == "/tmp/newrandom.log")
		require.True(t, v == "100")
	}
}

func TestLegacyConversionWithNoLegacyFile(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := filepath.Join(tmpDir, "legacy")
	positionsPath := filepath.Join(tmpDir, "positions")
	// Write a new file.
	err := writePositionFile(positionsPath, map[Entry]string{
		{Path: "/tmp/newrandom.log", Labels: ""}: "100",
	})
	require.NoError(t, err)

	ConvertLegacyPositionsFile(legacy, positionsPath, log.NewNopLogger())
	ps, err := readPositionsFile(positionsPath)
	require.NoError(t, err)
	require.Len(t, ps, 1)
	for k, v := range ps {
		require.True(t, k.Path == "/tmp/newrandom.log")
		require.True(t, v == "100")
	}
}

func TestConvertLegacyPositionsFileJournal(t *testing.T) {
	tmpDir := t.TempDir()

	journalCursor := "s=96c0493b15cd4824b73d031da667369f;i=25557;b=64c7ff8e6dbc4a21b77425da44ce57f1;m=d9ddfd2be7;t=63d7d20071963;x=336b5294210ef9ec"
	legacyFile := filepath.Join(tmpDir, "legacy.yaml")
	legacyPositions := LegacyFile{
		Positions: map[string]string{
			"/some/path/test.log": "100",
			CursorKey("oldjob"):   journalCursor,
		},
	}

	f, err := os.Create(legacyFile)
	require.NoError(t, err)
	require.NoError(t, yaml.NewEncoder(f).Encode(legacyPositions))
	require.NoError(t, f.Close())

	newFile := filepath.Join(tmpDir, "new.yaml")
	ConvertLegacyPositionsFileJournal(legacyFile, "oldjob", newFile, "loki.source.journal.test", log.NewNopLogger())

	f, err = os.Open(newFile)
	require.NoError(t, err)

	var posFile File
	require.NoError(t, yaml.NewDecoder(f).Decode(&posFile))
	require.NoError(t, f.Close())

	pos := posFile.Positions[Entry{Path: CursorKey("loki.source.journal.test"), Labels: "{}"}]
	require.Equal(t, journalCursor, pos)
}

func writeLegacy(t *testing.T, tmpDir string) string {
	legacy := filepath.Join(tmpDir, "legacy")
	legacyPositions := LegacyFile{
		Positions: make(map[string]string),
	}
	// Filename and byte offset
	legacyPositions.Positions["/tmp/random.log"] = "17623"
	buf, err := yaml.Marshal(legacyPositions)
	require.NoError(t, err)
	err = os.WriteFile(legacy, buf, 0644)
	require.NoError(t, err)
	return legacy
}

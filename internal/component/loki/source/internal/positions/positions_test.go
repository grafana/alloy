package positions

// This code is copied from Promtail. The positions package allows logging
// components to keep track of read file offsets on disk and continue from the
// same place in case of a restart.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func tempFilename(t *testing.T) string {
	t.Helper()

	temp, err := os.CreateTemp(t.TempDir(), "positions")
	if err != nil {
		t.Fatal("tempFilename:", err)
	}
	err = temp.Close()
	if err != nil {
		t.Fatal("tempFilename:", err)
	}

	name := temp.Name()
	err = os.Remove(name)
	if err != nil {
		t.Fatal("tempFilename:", err)
	}

	return name
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

func TestLegacyConversion(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := writeLegacy(t, tmpDir)
	positionsPath := filepath.Join(tmpDir, "positions")
	ConvertLegacyPositionsFile(legacy, positionsPath, log.NewNopLogger())
	ps, err := readPositionsFile(Config{
		PositionsFile: positionsPath,
	}, log.NewNopLogger())
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
	ps, err := readPositionsFile(Config{
		PositionsFile: positionsPath,
	}, log.NewNopLogger())
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
	ps, err := readPositionsFile(Config{
		PositionsFile: positionsPath,
	}, log.NewNopLogger())
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

func TestReadPositions(t *testing.T) {
	t.Run("current structure", func(t *testing.T) {
		temp := tempFilename(t)
		defer func() {
			_ = os.Remove(temp)
		}()

		yaml := []byte(`
positions:
  ? path: /tmp/random.log
    labels: '{job="tmp"}'
  : "17623"
`)
		err := os.WriteFile(temp, yaml, 0644)
		if err != nil {
			t.Fatal(err)
		}

		pos, err := readPositionsFile(Config{
			PositionsFile: temp,
		}, log.NewNopLogger())

		require.NoError(t, err)
		require.Equal(t, "17623", pos[Entry{
			Path:   "/tmp/random.log",
			Labels: `{job="tmp"}`,
		}])
	})

	t.Run("unknown properties", func(t *testing.T) {
		temp := tempFilename(t)
		defer func() {
			_ = os.Remove(temp)
		}()

		yaml := []byte(`
Version: v2
positions:
  ? path: /tmp/random.log
    labels: '{job="tmp"}'
  : "17623"
`)
		err := os.WriteFile(temp, yaml, 0644)
		if err != nil {
			t.Fatal(err)
		}

		pos, err := readPositionsFile(Config{
			PositionsFile: temp,
		}, log.NewNopLogger())

		require.NoError(t, err)
		require.Equal(t, "17623", pos[Entry{
			Path:   "/tmp/random.log",
			Labels: `{job="tmp"}`,
		}])
	})
}

func TestReadPositionsEmptyFile(t *testing.T) {
	temp := tempFilename(t)
	defer func() {
		_ = os.Remove(temp)
	}()

	yaml := []byte(``)
	err := os.WriteFile(temp, yaml, 0644)
	if err != nil {
		t.Fatal(err)
	}

	pos, err := readPositionsFile(Config{
		PositionsFile: temp,
	}, log.NewNopLogger())

	require.NoError(t, err)
	require.NotNil(t, pos)
}

func TestReadPositionsFromDir(t *testing.T) {
	temp := tempFilename(t)
	err := os.Mkdir(temp, 0644)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_ = os.Remove(temp)
	}()

	_, err = readPositionsFile(Config{
		PositionsFile: temp,
	}, log.NewNopLogger())

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), temp)) // error must contain filename
}

func TestReadPositionsFromBadYaml(t *testing.T) {
	temp := tempFilename(t)
	defer func() {
		_ = os.Remove(temp)
	}()

	badYaml := []byte(`
positions:
  ? path: /tmp/random.log
    labels: "{}"
  : "176
`)
	err := os.WriteFile(temp, badYaml, 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = readPositionsFile(Config{
		PositionsFile: temp,
	}, log.NewNopLogger())

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), temp)) // error must contain filename
}

func TestReadPositionsFromBadYamlIgnoreCorruption(t *testing.T) {
	temp := tempFilename(t)
	defer func() {
		_ = os.Remove(temp)
	}()

	badYaml := []byte(`
positions:
  ? path: /tmp/random.log
    labels: "{}"
  : "176
`)
	err := os.WriteFile(temp, badYaml, 0644)
	if err != nil {
		t.Fatal(err)
	}

	out, err := readPositionsFile(Config{
		PositionsFile:     temp,
		IgnoreInvalidYaml: true,
	}, log.NewNopLogger())

	require.NoError(t, err)
	require.Equal(t, map[Entry]string{}, out)
}

func Test_ReadOnly(t *testing.T) {
	temp := tempFilename(t)
	defer func() {
		_ = os.Remove(temp)
	}()
	yaml := []byte(`
positions:
  ? path: /tmp/random.log
    labels: '{job="tmp"}'
  : "17623"
`)
	err := os.WriteFile(temp, yaml, 0644)
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(log.NewNopLogger(), Config{
		SyncPeriod:    20 * time.Second,
		PositionsFile: temp,
		ReadOnly:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	p.Put("/foo/bar/f", "", 12132132)
	p.PutString("/foo/f", "", "100")
	pos, err := p.Get("/tmp/random.log", `{job="tmp"}`)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, int64(17623), pos)
	p.(*positions).save()
	out, err := readPositionsFile(Config{
		PositionsFile:     temp,
		IgnoreInvalidYaml: true,
		ReadOnly:          true,
	}, log.NewNopLogger())

	require.NoError(t, err)
	require.Equal(t, map[Entry]string{
		{Path: "/tmp/random.log", Labels: `{job="tmp"}`}: "17623",
	}, out)
}

func TestWriteEmptyLabels(t *testing.T) {
	temp := tempFilename(t)
	defer func() {
		_ = os.Remove(temp)
	}()
	yaml := []byte(`
positions:
  ? path: /tmp/initial.log
    labels: '{job="tmp"}'
  : "10030"
`)
	err := os.WriteFile(temp, yaml, 0644)
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(log.NewNopLogger(), Config{
		SyncPeriod:    20 * time.Second,
		PositionsFile: temp,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	p.Put("/tmp/foo/nolabels.log", "", 10040)
	p.Put("/tmp/foo/emptylabels.log", "{}", 10050)
	p.PutString("/tmp/bar/nolabels.log", "", "10060")
	p.PutString("/tmp/bar/emptylabels.log", "{}", "10070")
	pos, err := p.Get("/tmp/initial.log", `{job="tmp"}`)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, int64(10030), pos)
	p.(*positions).save()
	out, err := readPositionsFile(Config{
		PositionsFile:     temp,
		IgnoreInvalidYaml: true,
		ReadOnly:          false,
	}, log.NewNopLogger())

	require.NoError(t, err)
	require.Equal(t, map[Entry]string{
		{Path: "/tmp/initial.log", Labels: `{job="tmp"}`}: "10030",
		{Path: "/tmp/bar/emptylabels.log", Labels: `{}`}:  "10070",
		{Path: "/tmp/bar/nolabels.log", Labels: ""}:       "10060",
		{Path: "/tmp/foo/emptylabels.log", Labels: `{}`}:  "10050",
		{Path: "/tmp/foo/nolabels.log", Labels: ""}:       "10040",
	}, out)
}

func TestReadEmptyLabels(t *testing.T) {
	temp := tempFilename(t)
	defer func() {
		_ = os.Remove(temp)
	}()

	yaml := []byte(`
positions:
  ? path: /tmp/nolabels.log
    labels: ''
  : "10020"
  ? path: /tmp/emptylabels.log
    labels: '{}'
  : "10030"
  ? path: /tmp/missinglabels.log
  : "10040"
`)
	err := os.WriteFile(temp, yaml, 0644)
	if err != nil {
		t.Fatal(err)
	}

	pos, err := readPositionsFile(Config{
		PositionsFile: temp,
	}, log.NewNopLogger())

	require.NoError(t, err)
	require.Equal(t, "10020", pos[Entry{
		Path:   "/tmp/nolabels.log",
		Labels: ``,
	}])
	require.Equal(t, "10030", pos[Entry{
		Path:   "/tmp/emptylabels.log",
		Labels: `{}`,
	}])
	require.Equal(t, "10040", pos[Entry{
		Path:   "/tmp/missinglabels.log",
		Labels: ``,
	}])
}

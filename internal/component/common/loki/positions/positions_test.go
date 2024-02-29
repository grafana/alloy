package positions

// This code is copied from Promtail. The positions package allows logging
// components to keep track of read file offsets on disk and continue from the
// same place in case of a restart.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	util_log "github.com/grafana/loki/pkg/util/log"
)

func tempFilename(t *testing.T) string {
	t.Helper()

	temp, err := os.CreateTemp("", "positions")
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

func TestReadPositionsOK(t *testing.T) {
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
	p, err := New(util_log.Logger, Config{
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
	p, err := New(util_log.Logger, Config{
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

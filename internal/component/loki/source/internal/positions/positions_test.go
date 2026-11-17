package positions

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
)

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
			Path: temp,
		})

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
some_new_property: new_unexpected_value
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
			Path: temp,
		})

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
		Path: temp,
	})

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
		Path: temp,
	})

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
		Path: temp,
	})

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), temp)) // error must contain filename
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
	p, err := New(logging.NewSlogNop(), Config{
		Path: temp,
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
	p.save()
	out, err := readPositionsFile(Config{
		Path: temp,
	})

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
		Path: temp,
	})

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

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

	"github.com/grafana/alloy/syntax"
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		cfg      string
		expected Config
		wantErr  bool
	}{
		{
			name: "defaults",
			cfg:  ``,
			expected: Config{
				KeyMode:    KeyModeIncludeLabels,
				SyncPeriod: 10 * time.Second,
			},
		},
		{
			name: "custom values",
			cfg: `
				key_mode = "exclude_labels"
				sync_period = "30s"
			`,
			expected: Config{
				KeyMode:    KeyModeExcludeLabels,
				SyncPeriod: 30 * time.Second,
			},
		},
		{
			name: "invalid key mode",
			cfg: `
				key_mode = "nope"
			`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := syntax.Unmarshal([]byte(tt.cfg), &cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, cfg)
		})
	}
}

func TestPositionFile(t *testing.T) {
	t.Run("include labels tracks independently", func(t *testing.T) {
		p := &PositionsFile{
			cfg:       Config{KeyMode: KeyModeIncludeLabels, SyncPeriod: time.Second},
			positions: map[Entry]string{},
		}

		p.Put("/tmp/app.log", `{job="a"}`, 10)
		p.Put("/tmp/app.log", `{job="b"}`, 20)

		posA, err := p.Get("/tmp/app.log", `{job="a"}`)
		require.NoError(t, err)
		require.Equal(t, int64(10), posA)

		posB, err := p.Get("/tmp/app.log", `{job="b"}`)
		require.NoError(t, err)
		require.Equal(t, int64(20), posB)
	})

	t.Run("exclude labels tracks by path only", func(t *testing.T) {
		p := &PositionsFile{
			cfg:       Config{KeyMode: KeyModeExcludeLabels, SyncPeriod: time.Second},
			positions: map[Entry]string{},
		}

		p.Put("/tmp/app.log", `{job="a"}`, 10)
		p.Put("/tmp/app.log", `{job="b"}`, 20)

		posA, err := p.Get("/tmp/app.log", "")
		require.NoError(t, err)
		require.Equal(t, int64(20), posA)

		posB, err := p.Get("/tmp/app.log", "")
		require.NoError(t, err)
		require.Equal(t, int64(20), posB)
	})

	t.Run("switch include to exclude keeps readable position", func(t *testing.T) {
		p := &PositionsFile{
			cfg:       Config{KeyMode: KeyModeIncludeLabels, SyncPeriod: time.Second},
			positions: map[Entry]string{},
		}

		p.Put("/tmp/app.log", `{job="a"}`, 10)
		pos, err := p.Get("/tmp/app.log", `{job="a"}`)
		require.NoError(t, err)
		require.Equal(t, int64(10), pos)

		p.Update(Config{KeyMode: KeyModeExcludeLabels, SyncPeriod: time.Second})
		pos, err = p.Get("/tmp/app.log", `{job="a"}`)
		require.NoError(t, err)
		require.Equal(t, int64(10), pos)
	})

	t.Run("switch exclude to include keeps readable position", func(t *testing.T) {
		p := &PositionsFile{
			cfg:       Config{KeyMode: KeyModeExcludeLabels, SyncPeriod: time.Second},
			positions: map[Entry]string{},
		}

		p.Put("/tmp/app.log", "", 10)
		pos, err := p.Get("/tmp/app.log", "")
		require.NoError(t, err)
		require.Equal(t, int64(10), pos)

		p.Update(Config{KeyMode: KeyModeIncludeLabels, SyncPeriod: time.Second})
		pos, err = p.Get("/tmp/app.log", `{job="a"}`)
		require.NoError(t, err)
		require.Equal(t, int64(10), pos)
	})

	t.Run("remove respects active key mode", func(t *testing.T) {
		t.Run("exclude labels", func(t *testing.T) {
			p := &PositionsFile{
				cfg:       Config{KeyMode: KeyModeExcludeLabels, SyncPeriod: time.Second},
				positions: map[Entry]string{},
			}
			p.Put("/tmp/app.log", `{job="a"}`, 10)
			p.Remove("/tmp/app.log", `{job="b"}`)

			pos, err := p.Get("/tmp/app.log", `{job="a"}`)
			require.NoError(t, err)
			require.Equal(t, int64(0), pos)
		})

		t.Run("include labels", func(t *testing.T) {
			p := &PositionsFile{
				cfg:       Config{KeyMode: KeyModeIncludeLabels, SyncPeriod: time.Second},
				positions: map[Entry]string{},
			}
			p.Put("/tmp/app.log", `{job="a"}`, 10)
			p.Put("/tmp/app.log", `{job="b"}`, 20)
			p.Remove("/tmp/app.log", `{job="a"}`)

			posA, err := p.Get("/tmp/app.log", `{job="a"}`)
			require.NoError(t, err)
			require.Equal(t, int64(0), posA)

			posB, err := p.Get("/tmp/app.log", `{job="b"}`)
			require.NoError(t, err)
			require.Equal(t, int64(20), posB)
		})
	})
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

		pos, err := readPositionsFile(temp)

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

		pos, err := readPositionsFile(temp)

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

	pos, err := readPositionsFile(temp)

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

	_, err = readPositionsFile(temp)

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

	_, err = readPositionsFile(temp)

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
	p, err := New(log.NewNopLogger(), temp, Config{
		SyncPeriod: 20 * time.Second,
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
	p.(*PositionsFile).save()
	out, err := readPositionsFile(temp)

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

	pos, err := readPositionsFile(temp)

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
	require.NoError(t, err)
	require.NoError(t, temp.Close())

	name := temp.Name()
	require.NoError(t, os.Remove(name))
	return name
}

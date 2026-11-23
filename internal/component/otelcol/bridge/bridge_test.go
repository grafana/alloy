package bridge_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/processor/batchprocessor"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/bridge"
	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/token/builder"
)

func TestDecode_MatchesYAML(t *testing.T) {
	var args batch.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		timeout       = "1m"
		metadata_keys = ["tenant"]
		output {}
	`), &args))

	actual, err := args.Convert()
	require.NoError(t, err)

	// Settings the user doesn't write keep Alloy's documented defaults.
	expected := loadYAML(t, batchprocessor.NewFactory(), `
timeout: 1m
metadata_keys: [tenant]
send_batch_size: 2000
send_batch_max_size: 3000
`)
	require.Equal(t, expected, actual)
	require.Equal(t, batchprocessor.Config{
		Timeout:                  time.Minute,
		SendBatchSize:            2000,
		SendBatchMaxSize:         3000,
		MetadataKeys:             []string{"tenant"},
		MetadataCardinalityLimit: 1000,
	}, args.Config)
}

func TestDecode_Errors(t *testing.T) {
	tests := []struct {
		name   string
		config string
		err    string
	}{
		{
			name:   "unknown attribute is reported by Alloy with a position",
			config: "not_a_field = 1\noutput {}",
			err:    `1:1: unrecognized attribute name "not_a_field"`,
		},
		{
			name:   "wrong value type is reported by Alloy with a position",
			config: "metadata_keys = 1\noutput {}",
			err:    `1:17: 1 should be array, got number`,
		},
		{
			// Alloy would convert a number to a string, but upstream doesn't. The
			// error comes from confmap, so it has a key path but no position.
			name:   "upstream decoding is stricter than Alloy",
			config: "metadata_keys = [1]\noutput {}",
			err:    `'metadata_keys[0]' expected type 'string', got unconvertible type 'int'`,
		},
		{
			// Upstream validation is stricter than Alloy's old hand-written version.
			name:   "upstream validation",
			config: "metadata_keys = [\"tenant\", \"TENANT\"]\noutput {}",
			err:    `duplicate entry in metadata_keys: "tenant" (case-insensitive)`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args batch.Arguments
			err := syntax.Unmarshal([]byte(tc.config), &args)
			require.ErrorContains(t, err, tc.err)
		})
	}
}

// TestConformance converts every configuration in each upstream component's
// own testdata/config.yaml to Alloy syntax, decodes it back, and checks the
// result is what the Collector loads from the YAML. It runs for every bridged
// processor, so a dependency bump is checked against upstream's test cases.
func TestConformance(t *testing.T) {
	for _, p := range bridge.Processors() {
		t.Run(p.Name, func(t *testing.T) {
			path := upstreamTestdata(t, p.Factory, "config.yaml")
			if path == "" {
				t.Skip("upstream module has no testdata/config.yaml")
			}
			conf, err := confmaptest.LoadConf(path)
			require.NoError(t, err)

			// Upstream testdata is either keyed by component ID (resource,
			// resource/2, ...) or is a single flat configuration.
			cases := map[string]*confmap.Conf{}
			for key := range conf.ToStringMap() {
				var id otelcomponent.ID
				if err := id.UnmarshalText([]byte(key)); err == nil && id.Type() == p.Factory.Type() {
					sub, err := conf.Sub(key)
					require.NoError(t, err)
					cases[key] = sub
				}
			}
			if len(cases) == 0 {
				cases[p.Factory.Type().String()] = conf
			}

			for key, sub := range cases {
				t.Run(key, func(t *testing.T) {

					expected := p.Factory.CreateDefaultConfig()
					require.NoError(t, sub.Unmarshal(expected))
					expectValid := expected.(interface{ Validate() error }).Validate() == nil

					// Upstream config -> Alloy syntax.
					f := builder.NewFile()
					f.Body().AppendFrom(p.NewArguments(expected, &otelcol.ConsumerArguments{}))
					alloySrc := f.Bytes()

					// Alloy syntax -> upstream config.
					args := p.NewArguments(p.Factory.CreateDefaultConfig(), nil)
					err = syntax.Unmarshal(alloySrc, args)
					if !expectValid {
						require.Error(t, err, "upstream rejects this config, so Alloy should too:\n%s", alloySrc)
						return
					}
					require.NoError(t, err, "Alloy syntax:\n%s", alloySrc)

					actual, err := args.(interface {
						Convert() (otelcomponent.Config, error)
					}).Convert()
					require.NoError(t, err)
					require.Equal(t, expected, actual, "Alloy syntax:\n%s", alloySrc)
				})
			}
		})
	}
}

func loadYAML(t *testing.T, factory otelcomponent.Factory, in string) otelcomponent.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(in), 0o600))
	conf, err := confmaptest.LoadConf(path)
	require.NoError(t, err)

	cfg := factory.CreateDefaultConfig()
	require.NoError(t, conf.Unmarshal(cfg))
	return cfg
}

// upstreamTestdata returns the path of a file in the testdata directory of
// the module that provides factory, or "" if it doesn't exist.
func upstreamTestdata(t *testing.T, factory otelcomponent.Factory, name string) string {
	t.Helper()

	info, ok := debug.ReadBuildInfo()
	require.True(t, ok)

	pkgPath := reflect.TypeOf(factory.CreateDefaultConfig()).Elem().PkgPath()
	for _, dep := range info.Deps {
		if dep.Path != pkgPath {
			continue
		}
		if dep.Replace != nil {
			dep = dep.Replace
		}
		out, err := exec.Command("go", "env", "GOMODCACHE").Output()
		require.NoError(t, err)
		modCache := strings.TrimSpace(string(out))
		path := filepath.Join(modCache, escapeModulePath(dep.Path)+"@"+dep.Version, "testdata", name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// escapeModulePath escapes a module path the way the module cache does:
// upper-case letters become "!" followed by the lower-case letter.
func escapeModulePath(path string) string {
	var sb strings.Builder
	for _, r := range path {
		if unicode.IsUpper(r) {
			sb.WriteByte('!')
			r = unicode.ToLower(r)
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

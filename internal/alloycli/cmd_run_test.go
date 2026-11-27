package alloycli

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestConfigSource(t *testing.T) {
	const alloyConfig = "logging {\n\tlevel = \"debug\"\n}\n"

	encode := base64.StdEncoding.EncodeToString
	wrapped := func(s string) string {
		// Mimic base64(1), which wraps lines and ends with a newline.
		e := encode([]byte(s))
		return e[:10] + "\n" + e[10:] + "\n"
	}

	configFile := filepath.Join(t.TempDir(), "config.alloy")
	require.NoError(t, os.WriteFile(configFile, []byte(alloyConfig), 0o600))

	tt := []struct {
		name string
		// flags and args are applied in order, env is set for the test.
		flags       []string
		args        []string
		env         string
		wantSources map[string]string
		wantPath    string
		// wantConfigErr is checked when the source is resolved, wantReadErr when
		// it is read.
		wantConfigErr string
		wantReadErr   string
	}{
		{
			name:        "path argument",
			args:        []string{configFile},
			wantSources: map[string]string{configFile: alloyConfig},
			wantPath:    configFile,
		},
		{
			name:        "base64 from env",
			flags:       []string{"--config.base64.from-env"},
			env:         encode([]byte(alloyConfig)),
			wantSources: map[string]string{inlineConfigName: alloyConfig},
		},
		{
			name:        "base64 from env ignores whitespace",
			flags:       []string{"--config.base64.from-env"},
			env:         wrapped(alloyConfig),
			wantSources: map[string]string{inlineConfigName: alloyConfig},
		},
		{
			name:          "base64 from env with config format",
			flags:         []string{"--config.format=prometheus", "--config.base64.from-env"},
			env:           encode([]byte(alloyConfig)),
			wantConfigErr: "--config.format=prometheus can't be used with --config.base64.from-env",
		},
		{
			name:          "no source",
			wantConfigErr: "no config source provided",
		},
		{
			name:          "empty path argument",
			args:          []string{""},
			wantConfigErr: "path argument not provided",
		},
		{
			name:          "path argument and base64 from env",
			flags:         []string{"--config.base64.from-env"},
			args:          []string{configFile},
			env:           encode([]byte(alloyConfig)),
			wantConfigErr: "only one config source can be used at a time, but got the path argument and --config.base64.from-env",
		},
		{
			name:        "env variable unset",
			flags:       []string{"--config.base64.from-env"},
			wantReadErr: configBase64EnvVar + " is not set or empty",
		},
		{
			name:        "invalid base64",
			flags:       []string{"--config.base64.from-env"},
			env:         "super-secret-token*",
			wantReadErr: "decoding base64: illegal base64 data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(configBase64EnvVar, tc.env)

			r := newAlloyRun(nil)
			fset := pflag.NewFlagSet("run", pflag.ContinueOnError)
			mountRunFlags(r, fset)
			mountConfigSourceFlags(r, fset)
			require.NoError(t, fset.Parse(tc.flags))

			src, err := r.configSource(tc.args)
			if tc.wantConfigErr != "" {
				require.ErrorContains(t, err, tc.wantConfigErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantPath, src.path)

			sources, err := src.read()
			if tc.wantReadErr != "" {
				require.ErrorContains(t, err, tc.wantReadErr)
				if tc.env != "" {
					// The (possibly secret) configuration must not leak into errors.
					require.NotContains(t, err.Error(), tc.env)
				}
				return
			}
			require.NoError(t, err)

			got := make(map[string]string, len(sources))
			for name, contents := range sources {
				got[name] = string(contents)
			}
			require.Equal(t, tc.wantSources, got)
		})
	}
}

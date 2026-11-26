package alloycli

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
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
		env         map[string]string
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
			name:        "base64 flag",
			flags:       []string{"--config.base64=" + encode([]byte(alloyConfig))},
			wantSources: map[string]string{inlineConfigName: alloyConfig},
		},
		{
			name:        "base64 flag ignores whitespace",
			flags:       []string{"--config.base64=" + wrapped(alloyConfig)},
			wantSources: map[string]string{inlineConfigName: alloyConfig},
		},
		{
			name:        "base64 from env",
			flags:       []string{"--config.base64.from-env"},
			env:         map[string]string{configBase64EnvVar: encode([]byte(alloyConfig))},
			wantSources: map[string]string{inlineConfigName: alloyConfig},
		},
		{
			name:          "base64 flag with config format",
			flags:         []string{"--config.format=prometheus", "--config.base64=" + encode([]byte(alloyConfig))},
			wantConfigErr: "--config.format=prometheus can't be used with --config.base64",
		},
		{
			name:          "base64 from env with config format",
			flags:         []string{"--config.format=prometheus", "--config.base64.from-env"},
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
			name:          "empty base64 flag",
			flags:         []string{"--config.base64="},
			wantConfigErr: "--config.base64 must not be empty",
		},
		{
			name:          "path argument and base64 flag",
			flags:         []string{"--config.base64=" + encode([]byte(alloyConfig))},
			args:          []string{configFile},
			wantConfigErr: "only one config source can be used at a time, but got the path argument and --config.base64",
		},
		{
			name:          "path argument and base64 from env",
			flags:         []string{"--config.base64.from-env"},
			args:          []string{configFile},
			env:           map[string]string{configBase64EnvVar: encode([]byte(alloyConfig))},
			wantConfigErr: "only one config source can be used at a time, but got the path argument and --config.base64.from-env",
		},
		{
			name:          "base64 flag and base64 from env",
			flags:         []string{"--config.base64=" + encode([]byte(alloyConfig)), "--config.base64.from-env"},
			env:           map[string]string{configBase64EnvVar: encode([]byte(alloyConfig))},
			wantConfigErr: "only one config source can be used at a time, but got --config.base64 and --config.base64.from-env",
		},
		{
			name:        "env variable unset",
			flags:       []string{"--config.base64.from-env"},
			wantReadErr: configBase64EnvVar + " is not set or empty",
		},
		{
			name:        "invalid base64",
			flags:       []string{"--config.base64=not*base64"},
			wantReadErr: "decoding base64: illegal base64 data",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			r := newAlloyRun(nil)
			cmd := &cobra.Command{}
			mountRunFlags(r, cmd.Flags())
			mountConfigSourceFlags(r, cmd.Flags())
			require.NoError(t, cmd.Flags().Parse(tc.flags))

			src, err := r.configSource(cmd, tc.args)
			if tc.wantConfigErr != "" {
				require.ErrorContains(t, err, tc.wantConfigErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantPath, src.path)

			sources, err := src.read()
			if tc.wantReadErr != "" {
				require.ErrorContains(t, err, tc.wantReadErr)
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

func TestConfigSourceErrorDoesNotLeakConfig(t *testing.T) {
	const secret = "super-secret-token"

	r := newAlloyRun(nil)
	cmd := &cobra.Command{}
	mountRunFlags(r, cmd.Flags())
	mountConfigSourceFlags(r, cmd.Flags())
	require.NoError(t, cmd.Flags().Parse([]string{"--config.base64=" + secret + "*"}))

	src, err := r.configSource(cmd, nil)
	require.NoError(t, err)

	_, err = src.read()
	require.Error(t, err)
	require.NotContains(t, err.Error(), secret)
}

func TestSupportBundleRuntimeFlags(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`password = "fake-test-password"`))

	tt := []struct {
		name  string
		flags []string
		want  []string
	}{
		{
			name: "no config flags",
			want: []string{"config.base64=", "config.base64.from-env=false"},
		},
		{
			name:  "base64 flag is redacted",
			flags: []string{"--config.base64=" + encoded},
			want:  []string{"config.base64=(secret)", "config.base64.from-env=false"},
		},
		{
			name:  "base64 from env",
			flags: []string{"--config.base64.from-env"},
			want:  []string{"config.base64=", "config.base64.from-env=true"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			r := newAlloyRun(nil)
			cmd := &cobra.Command{}
			mountRunFlags(r, cmd.Flags())
			mountConfigSourceFlags(r, cmd.Flags())
			require.NoError(t, cmd.Flags().Parse(tc.flags))

			got := supportBundleRuntimeFlags(cmd.Flags())
			for _, w := range tc.want {
				require.Contains(t, got, w)
			}
			for _, f := range got {
				require.NotContains(t, f, encoded)
			}
		})
	}
}

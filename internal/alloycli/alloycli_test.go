package alloycli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/alloycli/clifeature"
	_ "github.com/grafana/alloy/internal/converter/install"
)

const testPrometheusConfig = `scrape_configs:
  - job_name: test
    static_configs:
      - targets: ["localhost:9090"]
remote_write:
  - url: http://localhost:9009/api/v1/push
`

const testConvertedConfig = `prometheus.scrape "test" {
	targets = [{
		__address__ = "localhost:9090",
	}]
	forward_to = [prometheus.remote_write.default.receiver]
	job_name   = "test"
}

prometheus.remote_write "default" {
	endpoint {
		url = "http://localhost:9009/api/v1/push"

		queue_config { }

		metadata_config { }
	}
}
`

// <bin> is replaced with the name of the test binary, which cobra uses as the
// name of the root command.
const testHelpWithoutConverter = `Grafana Alloy

Usage:
  <bin> [global options] <subcommand> [flags]
  <bin> [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  fmt         Format a configuration file
  gql         [EXPERIMENTAL] Run a GraphQL query against a running Alloy instance
  help        Help about any command
  run         Run Grafana Alloy with Default Engine
  validate    Validate a configuration file

Flags:
  -h, --help      help for <bin>
  -v, --version   version for <bin>

Additional help topics:
  <bin> tools      Utilities for various components

Use "<bin> [command] --help" for more information about a command.
`

const testValidateHelpWithoutConverter = `Validate a configuration file

Usage:
  <bin> validate [flags] file

Flags:
      --feature.community-components.enabled   Enable community components.
  -h, --help                                   help for validate
      --stability.level <stability_level>      Minimum stability level of features to enable. Supported values: "generally-available", "public-preview", "experimental" (default "generally-available")
`

const testValidateHelpWithConverter = `Validate a configuration file

Usage:
  <bin> validate [flags] file

Flags:
      --config.bypass-conversion-errors        Enable bypassing errors when converting
      --config.extra-args string               Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.
      --config.format string                   The format of the source file. Supported formats: "otelcol", "prometheus", "promtail", "static". (default "alloy")
      --feature.community-components.enabled   Enable community components.
  -h, --help                                   help for validate
      --stability.level <stability_level>      Minimum stability level of features to enable. Supported values: "generally-available", "public-preview", "experimental" (default "generally-available")
`

func TestCommands(t *testing.T) {
	dir := t.TempDir()
	promConfig := filepath.Join(dir, "prometheus.yaml")
	require.NoError(t, os.WriteFile(promConfig, []byte(testPrometheusConfig), 0o644))
	convertOut := filepath.Join(dir, "out.alloy")

	var (
		noConverter   = clifeature.Set{}
		withConverter = clifeature.All()
	)

	tests := []struct {
		name     string
		features clifeature.Set
		args     []string
		// Expected output written to the command's stdout.
		wantOut string
		// Expected error, if any.
		wantErr string
		// Expected contents of convertOut, if the command writes it.
		wantConverted string
	}{
		{
			name:     "help without converter",
			features: noConverter,
			args:     []string{"--help"},
			wantOut:  testHelpWithoutConverter,
		},
		{
			name:     "validate help without converter",
			features: noConverter,
			args:     []string{"validate", "--help"},
			wantOut:  testValidateHelpWithoutConverter,
		},
		{
			name:     "validate help with converter",
			features: withConverter,
			args:     []string{"validate", "--help"},
			wantOut:  testValidateHelpWithConverter,
		},
		{
			name:     "validate without converter",
			features: noConverter,
			args:     []string{"validate", "--config.format=prometheus", promConfig},
			wantErr:  "config conversion is not included in this build of Alloy",
		},
		{
			name:     "validate with converter",
			features: withConverter,
			args:     []string{"validate", "--config.format=prometheus", promConfig},
		},
		{
			name:     "convert without converter",
			features: noConverter,
			// Reading from stdin would block, so this also checks that the
			// command fails before reading its input.
			args:    []string{"convert", "--source-format=prometheus", "-"},
			wantErr: "config conversion is not included in this build of Alloy",
		},
		{
			name:          "convert with converter",
			features:      withConverter,
			args:          []string{"convert", "--source-format=prometheus", "--output=" + convertOut, promConfig},
			wantConverted: testConvertedConfig,
		},
	}

	bin := filepath.Base(os.Args[0])
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCommand(tc.features)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, strings.ReplaceAll(tc.wantOut, "<bin>", bin), out.String())

			if tc.wantConverted != "" {
				converted, err := os.ReadFile(convertOut)
				require.NoError(t, err)
				require.Equal(t, tc.wantConverted, string(converted))
			}
		})
	}
}

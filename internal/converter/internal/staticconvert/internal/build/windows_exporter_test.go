package build

import (
	"testing"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/static/integrations/windows_exporter"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/stretchr/testify/require"
)

func Test_split(t *testing.T) {
	tests := []struct {
		testName       string
		input          string
		expectedOutput []string
	}{
		{
			testName:       "multiple values",
			input:          "example1,example2",
			expectedOutput: []string{"example1", "example2"},
		},
		{
			testName:       "single value",
			input:          "example1",
			expectedOutput: []string{"example1"},
		},
		{
			testName:       "empty string",
			input:          "",
			expectedOutput: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			got := split(tt.input)
			require.Equal(t, tt.expectedOutput, got)
		})
	}
}

func Test_translateCollectors(t *testing.T) {
	// filetime becomes file, thermalzone is dropped, and a duplicate is removed.
	got := translateCollectors([]string{"cpu", "filetime", "thermalzone", "file", "os"})
	require.Equal(t, []string{"cpu", "file", "os"}, got)
}

func Test_mergePatterns(t *testing.T) {
	got := mergePatterns([]string{`C:\a\*`, `C:\b\*`}, []string{`C:\b\*`, `C:\c\*`})
	require.Equal(t, []string{`C:\a\*`, `C:\b\*`, `C:\c\*`}, got)
	// The result is never nil, to match the default of the component.
	require.NotNil(t, mergePatterns(nil, nil))
}

func Test_toWindowsExporter_translatesRemovedSettings(t *testing.T) {
	config := &windows_exporter.Config{
		EnabledCollectors: "cpu,filetime,thermalzone",
		File:              windows_exporter.FileConfig{FilePatterns: []string{`C:\a\*`}},
		Filetime:          windows_exporter.FiletimeConfig{FilePatterns: []string{`C:\a\*`, `C:\b\*`}},
	}

	args := toWindowsExporter(config)
	require.Equal(t, []string{"cpu", "file"}, args.EnabledCollectors)
	require.Equal(t, []string{`C:\a\*`, `C:\b\*`}, args.File.FilePatterns)
}

func Test_appendWindowsExporter_counterVersionWarning(t *testing.T) {
	build := func(counterVersion uint8) (string, diag.Diagnostics) {
		var diags diag.Diagnostics
		f := builder.NewFile()
		b := NewConfigBuilder(f, &diags, nil, &GlobalContext{IntegrationsLabelPrefix: "integrations"})
		b.appendWindowsExporter(&windows_exporter.Config{
			EnabledCollectors: "cpu,filetime",
			Process:           windows_exporter.ProcessConfig{CounterVersion: counterVersion},
		}, nil)
		return string(f.Bytes()), diags
	}

	out, diags := build(2)
	require.Contains(t, out, "prometheus.exporter.windows")
	require.Contains(t, out, `"file"`)
	require.NotContains(t, out, "filetime")
	require.NotContains(t, out, "counter_version")
	require.Len(t, diags, 1)
	require.Equal(t, diag.SeverityLevelWarn, diags[0].Severity)
	require.Contains(t, diags[0].Summary, "counter_version")

	_, diags = build(0)
	require.Empty(t, diags)
}

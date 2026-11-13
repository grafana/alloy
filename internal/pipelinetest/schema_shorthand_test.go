package pipelinetest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMapMatchSchemaForms(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want MapMatchSchema
	}{
		{
			name: "full form",
			yaml: "mode: partial\nvalues: {job: api}",
			want: MapMatchSchema{Mode: "partial", Values: map[string]string{"job": "api"}},
		},
		{
			name: "full form without mode",
			yaml: "values: {job: api}",
			want: MapMatchSchema{Values: map[string]string{"job": "api"}},
		},
		{
			name: "short form",
			yaml: "{job: api, level: error}",
			want: MapMatchSchema{Values: map[string]string{"job": "api", "level": "error"}},
		},
		{
			// A label called values holding a string is a short form.
			name: "short form with a label named values",
			yaml: "{values: x, mode: y}",
			want: MapMatchSchema{Values: map[string]string{"values": "x", "mode": "y"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got MapMatchSchema
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &got))
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAssertionTypeInference(t *testing.T) {
	one := 1
	require.Equal(t, "count", assertionType("", &one))
	require.Equal(t, "contains", assertionType("", nil))
	require.Equal(t, "contains", assertionType("contains", &one))
}

func TestSelectorAssertions(t *testing.T) {
	var schema AssertionSchema
	require.NoError(t, yaml.Unmarshal([]byte(`
loki:
  - match: {selector: '{service_name="api", level=~"warn|error"}'}
prometheus:
  - count: 2
    match: {selector: 'up{job!="ignored"}'}
`), &schema))

	checks, err := buildChecks(schema, nil)
	require.NoError(t, err)
	require.Len(t, checks, 2)
	require.Equal(t, `loki[0] contains: {service_name="api", level=~"warn|error"}`, checks[0].Name)
	require.Equal(t, `prometheus[0] count: count = 2, up{job!="ignored"}`, checks[1].Name)
}

func TestSelectorRejectsInvalidSyntax(t *testing.T) {
	_, err := buildChecks(AssertionSchema{
		Prometheus: []PrometheusAssertionSchema{{Match: PrometheusMatchSchema{Selector: "up{job="}}},
	}, nil)
	require.ErrorContains(t, err, `parse selector "up{job="`)
}

func TestLoadTestFileRejectsUnknownFields(t *testing.T) {
	path := t.TempDir() + "/a.pipelinetest.yaml"
	writeFile(t, path, "kind: pipelinetest\nversion: 1\nconfig: 'x'\nasert: {}\n")
	_, err := LoadTestFile(path)
	require.ErrorContains(t, err, "field asert not found")
}

func TestLoadTestFileChecksKindAndVersion(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		err     string
	}{
		"no kind":       {"config: 'x'\n", "not a pipeline test"},
		"other kind":    {"kind: Deployment\nversion: 1\n", "not a pipeline test"},
		"not a mapping": {"- a\n- b\n", "not a pipeline test"},
		"no version":    {"kind: pipelinetest\nconfig: 'x'\n", "version is missing, set version: 1"},
		// A newer format reports its version, not its new fields.
		"newer version": {"kind: pipelinetest\nversion: 2\nnew_field: x\n", "version 2 not supported, this Alloy supports version 1"},
	} {
		t.Run(name, func(t *testing.T) {
			path := t.TempDir() + "/a.pipelinetest.yaml"
			writeFile(t, path, tc.content)
			_, err := LoadTestFile(path)
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestTargetLiteralsKeepValuesAndLines(t *testing.T) {
	out, err := targetLiterals([]TargetInputSchema{{
		Replace: "discovery.kubernetes.pods",
		Targets: []map[string]string{{"__address__": "a:1", "note": "two  spaces\nand a newline"}},
	}})
	require.NoError(t, err)
	lit := out["discovery.kubernetes.pods"]
	require.NotContains(t, lit, "\n")
	require.Contains(t, lit, `"two  spaces\nand a newline"`)
}

func TestAssertionSinkMustBeConfigured(t *testing.T) {
	_, err := buildChecks(AssertionSchema{
		Loki: []LokiAssertionSchema{{Sink: "loki.write.audit", Count: new(int)}},
	}, []string{"loki.write.default"})
	require.ErrorContains(t, err, "sink loki.write.audit is not one of config.sinks")
}

func TestLokiEntryLineRepeat(t *testing.T) {
	e, err := buildLokiEntry(LokiEntrySchema{Line: "ab", LineRepeat: 3})
	require.NoError(t, err)
	require.Equal(t, "ababab", e.Line)

	_, err = buildLokiEntry(LokiEntrySchema{LineRepeat: 3})
	require.ErrorContains(t, err, "line_repeat needs a line")

	_, err = buildLokiEntry(LokiEntrySchema{Line: "ab", LineRepeat: -1})
	require.ErrorContains(t, err, "line_repeat must be positive")
}

package pipelinetest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSchema_ParsesWindowsFields(t *testing.T) {
	var schema TestSchema
	require.NoError(t, yaml.Unmarshal([]byte(`
os: windows
timeout: 5m
setup:
  files:
    - path: "{{TEMP_DIR}}/probe.txt"
      content: alloy
  commands:
    - run: [schtasks, /Create]
      cleanup: [schtasks, /Delete]
config: ""
assert:
  logs:
    - contains: deprecated
  prometheus:
    - type: count
      count: 0
      match:
        labels:
          mode: partial
          values:
            __name__: windows_scheduled_task_state
          regex:
            task: /Microsoft/.*
`), &schema))

	require.Equal(t, "windows", schema.OS)
	require.Equal(t, "5m", schema.Timeout)
	require.Len(t, schema.Setup.Files, 1)
	require.Len(t, schema.Setup.Commands, 1)

	assertions, err := buildAssertions(schema.Assert)
	require.NoError(t, err)
	require.Len(t, assertions, 2)
}

func TestBuildAssertions_RejectsBadInput(t *testing.T) {
	_, err := buildAssertions(AssertionSchema{Logs: []LogAssertionSchema{{}}})
	require.Error(t, err)

	count := 0
	_, err = buildAssertions(AssertionSchema{Prometheus: []PrometheusAssertionSchema{{
		Type:  "count",
		Count: &count,
		Match: PrometheusMatchSchema{Labels: MapMatchSchema{Regex: map[string]string{"task": "("}}},
	}}})
	require.Error(t, err)
}

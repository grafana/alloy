package pipelinetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSchema_ParsesWindowsFields(t *testing.T) {
	var schema TestSchema
	require.NoError(t, yaml.Unmarshal([]byte(`
os: windows
timeout: 5m
stable_for: 10s
setup:
  files:
    - path: "{{TEMP_DIR}}/probe.txt"
      content: alloy
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
	require.Equal(t, "10s", schema.StableFor)

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

func TestValidateOS(t *testing.T) {
	require.NoError(t, validateOS(""))
	require.NoError(t, validateOS("windows"))
	require.Error(t, validateOS("Windows"))
	require.Error(t, validateOS("win"))
}

func TestBuildAssertions_RejectsRegexForLoki(t *testing.T) {
	_, err := buildAssertions(AssertionSchema{Loki: []LokiAssertionSchema{{
		Type:  "contains",
		Match: LokiMatchSchema{Labels: MapMatchSchema{Regex: map[string]string{"app": ".*"}}},
	}}})
	require.Error(t, err)
}

func TestBuildAssertions_ValidatesModeForRegexOnlyMatch(t *testing.T) {
	count := 0
	_, err := buildAssertions(AssertionSchema{Prometheus: []PrometheusAssertionSchema{{
		Type:  "count",
		Count: &count,
		Match: PrometheusMatchSchema{Labels: MapMatchSchema{Mode: "bogus", Regex: map[string]string{"task": ".*"}}},
	}}})
	require.Error(t, err)
}

func TestLoggingBlock(t *testing.T) {
	require.Contains(t, logging(`prometheus.scrape "x" {}`), `destination = "stderr"`)
	require.Empty(t, logging("logging {\n  level = \"debug\"\n}\n"))
	require.Empty(t, logging("  logging{\n  }\n"))
}

func TestCapTimeout(t *testing.T) {
	// No deadline or no timeout means no cap.
	got, err := capTimeout(time.Minute, time.Time{})
	require.NoError(t, err)
	require.Equal(t, time.Minute, got)
	got, err = capTimeout(0, time.Now().Add(time.Second))
	require.NoError(t, err)
	require.Zero(t, got)

	got, err = capTimeout(time.Hour, time.Now().Add(5*time.Minute))
	require.NoError(t, err)
	require.Less(t, got, 5*time.Minute)
	require.Greater(t, got, 4*time.Minute)

	_, err = capTimeout(time.Minute, time.Now().Add(time.Second))
	require.Error(t, err)
}

func TestRunTest_RejectsStableWindowLongerThanTimeout(t *testing.T) {
	err := RunTest(t.Context(), TestSchema{Timeout: "1s", StableFor: "5s"}, TestConfig{DataPath: t.TempDir(), WorkDir: t.TempDir()})
	require.ErrorContains(t, err, "stable window")
}

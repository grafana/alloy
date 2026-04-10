package stages

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testOutputAlloy = `
stage.json {
    expressions = { "out" = "message" }
}
stage.output {
    source = "out"
}
`

var testOutputLogLine = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN",
	"nested" : {"child":"value"},
	"message" : "this is a log line"
}
`
var testOutputLogLineWithMissingKey = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN",
	"nested" : {"child":"value"}
}
`

func TestPipeline_Output(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	pl, err := NewPipeline(logger.Slog(), loadConfig(testOutputAlloy), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)

	out := processEntries(pl, newEntry(nil, nil, testOutputLogLine, time.Now()))[0]
	assert.Equal(t, "this is a log line", out.Line)
}

func TestPipelineWithMissingKey_Output(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logging.New(&buf, logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
	require.NoError(t, err)
	pl, err := NewPipeline(logger.Slog(), loadConfig(testOutputAlloy), prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	require.NoError(t, err)

	_ = processEntries(pl, newEntry(nil, nil, testOutputLogLineWithMissingKey, time.Now()))
	expectedLog := "level=debug source=/home/kalle/projects/grafana/alloy/internal/component/loki/process/stages/output.go:46 msg=\"extracted output could not be converted to a string\" stage=output err=\"can't convert <nil> to string\" type=<nil>"
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

func TestOutputValidation(t *testing.T) {
	emptyConfig := OutputConfig{Source: ""}
	_, err := newOutputStage(logging.NewSlogNop(), emptyConfig)
	require.Equal(t, err, ErrOutputSourceRequired)
}

func TestOutputStage_Process(t *testing.T) {
	cfg := OutputConfig{
		Source: "out",
	}
	extractedValues := map[string]any{
		"something": "notimportant",
		"out":       "outmessage",
	}
	wantOutput := "outmessage"

	st, err := newOutputStage(logging.NewSlogNop(), cfg)
	require.NoError(t, err)
	out := processEntries(st, newEntry(extractedValues, nil, "replaceme", time.Time{}))[0]

	assert.Equal(t, wantOutput, out.Line)
}

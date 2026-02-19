package stages

import (
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
)

// Not all these are tested but are here to make sure the different types marshal without error
var testLimitWaitAlloy = `
stage.json {
		expressions = { "app" = "", "msg" = "" }
}
stage.limit {
		rate  = 1
		burst = 1
		drop  = false
}`

// Not all these are tested but are here to make sure the different types marshal without error
var testLimitDropAlloy = `
stage.json {
		expressions = { "app" = "", "msg" = "" }
}
stage.limit {
		rate  = 1
		burst = 1
		drop  = true
}`

var testLimitByLabelAlloy = `
stage.json {
		expressions = { "app" = "", "msg" = "" }
}
stage.limit {
		rate  = 1
		burst = 1
		drop  = true
		
		by_label_name = "app"
}`

var testNonAppLogLine = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"msg" : "Non app log line"
}
`

// TestLimitPipeline is used to verify we properly parse the yaml config and create a working pipeline
func TestLimitWaitPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testLimitWaitAlloy), registry, featuregate.StabilityGenerallyAvailable)
	logs := make([]Entry, 0)
	logCount := 5
	for range logCount {
		logs = append(logs, newEntry(nil, model.LabelSet{"app": "loki"}, testMatchLogLineApp1, time.Now()))
	}
	require.NoError(t, err)
	out := processEntries(pl,
		logs...,
	)
	// Only the second line will go through.
	assert.Len(t, out, logCount)
	assert.Equal(t, out[0].Line, testMatchLogLineApp1)
}

// TestLimitPipeline is used to verify we properly parse the yaml config and create a working pipeline
func TestLimitDropPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testLimitDropAlloy), registry, featuregate.StabilityGenerallyAvailable)
	logs := make([]Entry, 0)
	logCount := 10
	for range logCount {
		logs = append(logs, newEntry(nil, model.LabelSet{"app": "loki"}, testMatchLogLineApp1, time.Now()))
	}
	require.NoError(t, err)
	out := processEntries(pl,
		logs...,
	)
	// Only the second line will go through.
	assert.Len(t, out, 1)
	assert.Equal(t, out[0].Line, testMatchLogLineApp1)
}

// TestLimitByLabelPipeline is used to verify we properly parse the yaml config and create a working pipeline
func TestLimitByLabelPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testLimitByLabelAlloy), registry, featuregate.StabilityGenerallyAvailable)
	logs := make([]Entry, 0)
	logCount := 5
	for range logCount {
		logs = append(logs, newEntry(nil, model.LabelSet{"app": "loki"}, testMatchLogLineApp1, time.Now()))
	}
	for range logCount {
		logs = append(logs, newEntry(nil, model.LabelSet{"app": "poki"}, testMatchLogLineApp2, time.Now()))
	}
	for range logCount {
		logs = append(logs, newEntry(nil, model.LabelSet{}, testNonAppLogLine, time.Now()))
	}
	require.NoError(t, err)
	out := processEntries(pl,
		logs...,
	)
	// Only one entry of each app will go through + all log lines without expected label
	assert.Len(t, out, 2+logCount)
	assert.Equal(t, out[0].Line, testMatchLogLineApp1)
	assert.Equal(t, out[1].Line, testMatchLogLineApp2)
	assert.Equal(t, out[3].Line, testNonAppLogLine)

	var hasTotal, hasByLabel bool
	mfs, _ := registry.Gather()
	for _, mf := range mfs {
		switch *mf.Name {
		case "loki_process_dropped_lines_total":
			hasTotal = true
			assert.Len(t, mf.Metric, 1)
			assert.Equal(t, 8, int(mf.Metric[0].Counter.GetValue()))
		case "loki_process_dropped_lines_by_label_total":
			hasByLabel = true
			assert.Len(t, mf.Metric, 2)
			assert.Equal(t, 4, int(mf.Metric[0].Counter.GetValue()))
			assert.Equal(t, 4, int(mf.Metric[1].Counter.GetValue()))

			assert.Equal(t, mf.Metric[0].Label[0].GetName(), "label_name")
			assert.Equal(t, mf.Metric[0].Label[0].GetValue(), "app")
			assert.Equal(t, mf.Metric[0].Label[1].GetName(), "label_value")
			assert.Equal(t, mf.Metric[0].Label[1].GetValue(), "loki")

			assert.Equal(t, mf.Metric[1].Label[0].GetName(), "label_name")
			assert.Equal(t, mf.Metric[1].Label[0].GetValue(), "app")
			assert.Equal(t, mf.Metric[1].Label[1].GetName(), "label_value")
			assert.Equal(t, mf.Metric[1].Label[1].GetValue(), "poki")
		}
	}
	assert.True(t, hasTotal)
	assert.True(t, hasByLabel)
}

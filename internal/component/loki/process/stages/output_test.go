package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestOutputStage(t *testing.T) {
	var (
		now         = time.Now()
		jsonLogLine = `
		{
			"time":"2012-11-01T22:08:41+00:00",
			"app":"loki",
			"component": ["parser","type"],
			"level" : "WARN",
			"nested" : {"child":"value"},
			"message" : "this is a log line"
		}`
		jsonLogLineWithMissingKey = `
		{
			"time":"2012-11-01T22:08:41+00:00",
			"app":"loki",
			"component": ["parser","type"],
			"level" : "WARN",
			"nested" : {"child":"value"}
		}`
	)

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "output set from extracted source",
			config: `
			stage.output {
				source = "out"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"something": "notimportant",
					"out":       "outmessage",
				}, model.LabelSet{}, "replaceme", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"something": "notimportant",
					"out":       "outmessage",
				}, model.LabelSet{}, "outmessage", now),
			},
		},
		{
			name: "output set from json expression",
			config: `
			stage.json {
				expressions = { "out" = "message" }
			}
			stage.output {
				source = "out"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, jsonLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"out": "this is a log line",
				}, model.LabelSet{}, "this is a log line", now),
			},
		},
		{
			name: "missing extracted source from json expression leaves line unchanged",
			config: `
			stage.json {
				expressions = { "out" = "message" }
			}
			stage.output {
				source = "out"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, jsonLogLineWithMissingKey, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"out": nil}, model.LabelSet{}, jsonLogLineWithMissingKey, now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected)
		})
	}
}

func TestValidateOutputConfig(t *testing.T) {
	emptyConfig := OutputConfig{Source: ""}
	require.Equal(t, validateOutputConfig(emptyConfig), errOutputSourceRequired)
}

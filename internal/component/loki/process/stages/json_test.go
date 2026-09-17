package stages

import (
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

var logFixture = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN",
	"numeric": {
		"float": 12.34,
		"integer": 123,
		"string": "123"
	},
	"nested" : {"child":"value"},
	"message" : "this is a log line",
	"complex" : {
		"log" : {"array":[{"test1":"test2"},{"test3":"test4"}],"prop":"value","prop2":"val2"}
	}
}
`

func TestJSONStage(t *testing.T) {
	var (
		now             = time.Now()
		testJSONLogLine = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
	"component": ["parser","type"],
	"level" : "WARN",
	"nested" : {"child":"value"},
	"duration" : 125,
	"message" : "this is a log line",
	"extra": "{\"user\":\"marco\"}"
}
`
	)

	type testCase struct {
		name     string
		config   string
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "single stage without source",
			config: `
stage.json {
    expressions = {"out" = "message", "app" = "", "nested" = "", duration = "", unknown = "" }
}
`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testJSONLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"out":      "this is a log line",
					"app":      "loki",
					"nested":   "{\"child\":\"value\"}",
					"duration": float64(125),
					"unknown":  nil,
				}, model.LabelSet{}, testJSONLogLine, now),
			},
		},
		{
			name: "multiple stages with source",
			config: `
			stage.json {
				expressions = { "extra" = "" }
			}

			stage.json {
				expressions = { "user" = "" }
				source      = "extra"
			}`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testJSONLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"extra": "{\"user\":\"marco\"}",
					"user":  "marco",
				}, model.LabelSet{}, testJSONLogLine, now),
			},
		},
		{
			name: "regex",
			config: `
			stage.json {
			  regex = "pod_.*"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, `{"time":"2012-11-01T22:08:41+00:00", "pod_name": "my-pod-123", "pod_label": "my-label"}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"pod_name":  "my-pod-123",
					"pod_label": "my-label",
				}, model.LabelSet{}, `{"time":"2012-11-01T22:08:41+00:00", "pod_name": "my-pod-123", "pod_label": "my-label"}`, now),
			},
		},
		{
			name: "regex matching everything",
			config: `
			stage.json {
			  regex = ".*"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testJSONLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"time":      "2012-11-01T22:08:41+00:00",
					"app":       "loki",
					"component": `["parser","type"]`,
					"level":     "WARN",
					"nested":    `{"child":"value"}`,
					"duration":  float64(125),
					"message":   "this is a log line",
					"extra":     "{\"user\":\"marco\"}",
				}, model.LabelSet{}, testJSONLogLine, now),
			},
		},
		{
			name: "expressions and regex",
			config: `
			stage.json {
			  expressions = {"out" = "message", "app" = ""}
			  regex = "(app|duration)"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, testJSONLogLine, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"out":      "this is a log line",
					"app":      "loki",
					"duration": float64(125),
				}, model.LabelSet{}, testJSONLogLine, now),
			},
		},
		{
			name: "drop malformed",
			config: `
			stage.json {
				expressions    = { "page" = "page" }
				drop_malformed = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"test_label": "unimportant value"}, model.LabelSet{"foo": "bar"}, `{"page": 1, "fruits": ["apple", "peach"]}`, now),
				newEntry(map[string]any{"test_label": "unimportant value"}, model.LabelSet{"foo": "bar"}, `{"page": 1, fruits": ["apple", "peach"]}`, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"test_label": "unimportant value",
					"foo":        "bar",
					"page":       float64(1),
				}, model.LabelSet{"foo": "bar"}, `{"page": 1, "fruits": ["apple", "peach"]}`, now),
			},
		},
		{
			name: "decode json on entry",
			config: `
			stage.json {
				expressions = {
					"time" = "",
					"app" = "",
					"component" = "",
					"level" = "",
					"float" = "numeric.float",
					"integer" = "numeric.integer",
					"string" = "numeric.string",
					"nested" = "",
					"message" = "",
					"complex" = "complex.log.array[1].test3",
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, logFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"time":      "2012-11-01T22:08:41+00:00",
					"app":       "loki",
					"component": "[\"parser\",\"type\"]",
					"level":     "WARN",
					"float":     12.34,
					"integer":   123.0,
					"string":    "123",
					"nested":    "{\"child\":\"value\"}",
					"message":   "this is a log line",
					"complex":   "test4",
				}, model.LabelSet{}, logFixture, now),
			},
		},
		{
			name: "decode json on extracted source",
			config: `
			stage.json {
				expressions = {
					"time" = "",
					"app" = "",
					"component" = "",
					"level" = "",
					"float" = "numeric.float",
					"integer" = "numeric.integer",
					"string" = "numeric.string",
					"nested" = "",
					"message" = "",
					"complex" = "complex.log.array[1].test3",
				}
				source      = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"log": logFixture}, model.LabelSet{}, "{}", now),
			},
			expected: []Entry{
				newEntry(map[string]any{
					"time":      "2012-11-01T22:08:41+00:00",
					"app":       "loki",
					"component": "[\"parser\",\"type\"]",
					"level":     "WARN",
					"float":     12.34,
					"integer":   123.0,
					"string":    "123",
					"nested":    "{\"child\":\"value\"}",
					"message":   "this is a log line",
					"complex":   "test4",
					"log":       logFixture,
				}, model.LabelSet{}, "{}", now),
			},
		},
		{
			name: "missing extracted source",
			config: `
			stage.json {
				expressions = { "app" = "" }
				source      = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, logFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, logFixture, now),
			},
		},
		{
			name: "invalid json on entry",
			config: `
			stage.json {
				expressions = { "expr1" = "" }
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "ts=now log=notjson", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "ts=now log=notjson", now),
			},
		},
		{
			name: "invalid json on extracted source",
			config: `
			stage.json {
				expressions = { "app" = "" }
				source      = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"log": "not a json"}, model.LabelSet{}, logFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"log": "not a json"}, model.LabelSet{}, logFixture, now),
			},
		},
		{
			name: "nil source",
			config: `
			stage.json {
				expressions = { "app" = "" }
				source      = "log"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"log": nil}, model.LabelSet{}, logFixture, now),
			},
			expected: []Entry{
				newEntry(map[string]any{"log": nil}, model.LabelSet{}, logFixture, now),
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

func TestValidateJSONConfig(t *testing.T) {
	t.Parallel()

	var emptyString = ""
	var logString = "log"

	type testCase struct {
		name          string
		config        *JSONConfig
		err           error
		wantExprCount int
	}

	tests := []testCase{
		{
			name: "empty config",
			err:  errEmptyJSONStageConfig,
		},
		{
			name:   "no expressions",
			config: &JSONConfig{},
			err:    errExpressionsOrRegexRequired,
		},
		{
			name: "invalid expression",
			config: &JSONConfig{
				Expressions: map[string]string{
					"extr1": "3##@$#33",
				},
			},
			err: errCouldNotCompileJMES,
		},
		{
			name: "empty source",
			config: &JSONConfig{
				Expressions: map[string]string{
					"extr1": "expr",
				},
				Source: &emptyString,
			},
			err: errEmptyJSONStageSource,
		},
		{
			name: "valid without source",
			config: &JSONConfig{
				Expressions: map[string]string{
					"expr1": "expr",
					"expr2": "",
					"expr3": "expr1.expr2",
				},
			},
			wantExprCount: 3,
		},
		{
			name: "valid with source",
			config: &JSONConfig{
				Expressions: map[string]string{
					"expr1": "expr",
					"expr2": "",
					"expr3": "expr1.expr2",
				},
				Source: &logString,
			},
			wantExprCount: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := validateJSONConfig(tt.config)
			require.ErrorIs(t, err, tt.err)
			require.Len(t, got, tt.wantExprCount)
		})
	}
}

func TestJSONConfigUnmarshal(t *testing.T) {
	t.Parallel()
	var cfg = `
  expressions = {
    key1 = "expression1",
    key2 = "expression2.expression2",
  }
`
	// testing that we can use Alloy data into the config structure.
	var got JSONConfig
	err := syntax.Unmarshal([]byte(cfg), &got)
	assert.NoError(t, err, "error while un-marshalling config: %s", err)

	want := JSONConfig{
		Expressions: map[string]string{
			"key1": "expression1",
			"key2": "expression2.expression2",
		},
	}
	assert.True(t, reflect.DeepEqual(got, want), "want: %+v got: %+v", want, got)
}

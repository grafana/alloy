package stages

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

var testJSONAlloySingleStageWithoutSource = `
stage.json {
    expressions = {"out" = "message", "app" = "", "nested" = "", duration = "", unknown = "" }
}
`

var testJSONAlloyMultiStageWithSource = `
stage.json {
    expressions = { "extra" = "" }
}

stage.json {
    expressions = { "user" = "" }
    source      = "extra"
}`

var testJSONAlloyRegex = `
stage.json {
  regex = "pod_.*"
}
`

var testJSONAlloyRegexAll = `
stage.json {
  regex = ".*"
}
`

var testJSONAlloyExpressionsAndRegex = `
stage.json {
  expressions = {"out" = "message", "app" = ""}
  regex = "(app|duration)"
}
`

var testJSONLogLine = `
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

func TestPipeline_JSON(t *testing.T) {
	t.Parallel()
	logger := util.TestAlloyLogger(t)

	tests := map[string]struct {
		config          string
		entry           string
		expectedExtract map[string]interface{}
	}{
		"successfully run a pipeline with 1 json stage without source": {
			testJSONAlloySingleStageWithoutSource,
			testJSONLogLine,
			map[string]interface{}{
				"out":      "this is a log line",
				"app":      "loki",
				"nested":   "{\"child\":\"value\"}",
				"duration": float64(125),
				"unknown":  nil,
			},
		},
		"successfully run a pipeline with 2 json stages with source": {
			testJSONAlloyMultiStageWithSource,
			testJSONLogLine,
			map[string]interface{}{
				"extra": "{\"user\":\"marco\"}",
				"user":  "marco",
			},
		},
		"successfully extract regex values from json": {
			testJSONAlloyRegex,
      `{"time":"2012-11-01T22:08:41+00:00", "pod_name": "my-pod-123", "pod_label": "my-label"}`,
			map[string]interface{}{
        "pod_name": "my-pod-123",
        "pod_label": "my-label",
			},
		},
		"successfully extract all values from json via regex": {
			testJSONAlloyRegexAll,
			testJSONLogLine,
			map[string]interface{}{
        "time": "2012-11-01T22:08:41+00:00",
        "app": "loki",
        "component": `["parser","type"]`,
        "level": "WARN",
        "nested": `{"child":"value"}`,
        "duration": float64(125),
        "message": "this is a log line",
				"extra": "{\"user\":\"marco\"}",
			},
		},
    "successfully extract values with expressions and regex from json": {
      testJSONAlloyExpressionsAndRegex,
      testJSONLogLine,
      map[string]interface{}{
        "out": "this is a log line",
        "app": "loki",
        "duration": float64(125),
      },
    },
	}

	for testName, testData := range tests {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(logger, loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			assert.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl, newEntry(nil, nil, testData.entry, time.Now()))[0]
			assert.Equal(t, testData.expectedExtract, out.Extracted)
		})
	}
}

var jsonCfg = `
  expressions = {
    key1 = "expression1",
    key2 = "expression2.expression2",
  }
`

// nolint
func TestAlloy(t *testing.T) {
	t.Parallel()

	// testing that we can use Alloy data into the config structure.
	var got JSONConfig
	err := syntax.Unmarshal([]byte(jsonCfg), &got)
	assert.NoError(t, err, "error while un-marshalling config: %s", err)

	want := JSONConfig{
		Expressions: map[string]string{
			"key1": "expression1",
			"key2": "expression2.expression2",
		},
	}
	assert.True(t, reflect.DeepEqual(got, want), "want: %+v got: %+v", want, got)
}

func TestJSONConfig_validate(t *testing.T) {
	t.Parallel()

	var emptyString = ""
	var logString = "log"

	tests := map[string]struct {
		config        *JSONConfig
		wantExprCount int
		err           error
	}{
		"empty config": {
			nil,
			0,
			errors.New(ErrEmptyJSONStageConfig),
		},
		"no expressions": {
			&JSONConfig{},
			0,
			errors.New(ErrExpressionsOrRegexRequired),
		},
		"invalid expression": {
			&JSONConfig{
				Expressions: map[string]string{
					"extr1": "3##@$#33",
				},
			},
			0,
			fmt.Errorf("%s: SyntaxError: Unknown char: '#'", ErrCouldNotCompileJMES),
		},
		"empty source": {
			&JSONConfig{
				Expressions: map[string]string{
					"extr1": "expr",
				},
				Source: &emptyString,
			},
			0,
			errors.New(ErrEmptyJSONStageSource),
		},
		"valid without source": {
			&JSONConfig{
				Expressions: map[string]string{
					"expr1": "expr",
					"expr2": "",
					"expr3": "expr1.expr2",
				},
			},
			3,
			nil,
		},
		"valid with source": {
			&JSONConfig{
				Expressions: map[string]string{
					"expr1": "expr",
					"expr2": "",
					"expr3": "expr1.expr2",
				},
				Source: &logString,
			},
			3,
			nil,
		},
	}
	for tName, tt := range tests {
		tt := tt
		t.Run(tName, func(t *testing.T) {
			got, _, err := validateJSONConfig(tt.config)
			if tt.err != nil {
				assert.NotNil(t, err, "JSONConfig.validate() expected error = %v, but got nil", tt.err)
			}
			if err != nil {
				assert.Equal(t, tt.err.Error(), err.Error(), "JSONConfig.validate() expected error = %v, actual error = %v", tt.err, err)
			}
			assert.Equal(t, tt.wantExprCount, len(got))
		})
	}
}

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

func TestJSONParser_Parse(t *testing.T) {
	t.Parallel()
	logger := util.TestAlloyLogger(t)

	var logString = "log"
	tests := map[string]struct {
		config          StageConfig
		extracted       map[string]interface{}
		entry           string
		expectedExtract map[string]interface{}
	}{
		"successfully decode json on entry": {
			StageConfig{JSONConfig: &JSONConfig{
				Expressions: map[string]string{
					"time":      "",
					"app":       "",
					"component": "",
					"level":     "",
					"float":     "numeric.float",
					"integer":   "numeric.integer",
					"string":    "numeric.string",
					"nested":    "",
					"message":   "",
					"complex":   "complex.log.array[1].test3",
				},
			}},
			map[string]interface{}{},
			logFixture,
			map[string]interface{}{
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
			},
		},
		"successfully decode json on extracted[source]": {
			StageConfig{JSONConfig: &JSONConfig{
				Expressions: map[string]string{
					"time":      "",
					"app":       "",
					"component": "",
					"level":     "",
					"float":     "numeric.float",
					"integer":   "numeric.integer",
					"string":    "numeric.string",
					"nested":    "",
					"message":   "",
					"complex":   "complex.log.array[1].test3",
				},
				Source: &logString,
			}},
			map[string]interface{}{
				"log": logFixture,
			},
			"{}",
			map[string]interface{}{
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
			},
		},
		"missing extracted[source]": {
			StageConfig{JSONConfig: &JSONConfig{
				Expressions: map[string]string{
					"app": "",
				},
				Source: &logString,
			}},
			map[string]interface{}{},
			logFixture,
			map[string]interface{}{},
		},
		"invalid json on entry": {
			StageConfig{JSONConfig: &JSONConfig{
				Expressions: map[string]string{
					"expr1": "",
				},
			}},
			map[string]interface{}{},
			"ts=now log=notjson",
			map[string]interface{}{},
		},
		"invalid json on extracted[source]": {
			StageConfig{JSONConfig: &JSONConfig{
				Expressions: map[string]string{
					"app": "",
				},
				Source: &logString,
			}},
			map[string]interface{}{
				"log": "not a json",
			},
			logFixture,
			map[string]interface{}{
				"log": "not a json",
			},
		},
		"nil source": {
			StageConfig{JSONConfig: &JSONConfig{
				Expressions: map[string]string{
					"app": "",
				},
				Source: &logString,
			}},
			map[string]interface{}{
				"log": nil,
			},
			logFixture,
			map[string]interface{}{
				"log": nil,
			},
		},
	}
	for tName, tt := range tests {
		tt := tt
		t.Run(tName, func(t *testing.T) {
			t.Parallel()
			p, err := New(logger, nil, tt.config, nil, featuregate.StabilityGenerallyAvailable)
			assert.NoError(t, err, "failed to create json parser: %s", err)
			out := processEntries(p, newEntry(tt.extracted, nil, tt.entry, time.Now()))[0]

			assert.Equal(t, tt.expectedExtract, out.Extracted)
		})
	}
}

func TestValidateJSONDrop(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	labels := map[string]string{"foo": "bar"}
	matchConfig := &JSONConfig{
		DropMalformed: true,
		Expressions:   map[string]string{"page": "page"},
	}
	s, err := newJSONStage(logger, *matchConfig)
	assert.NoError(t, err, "withMatcher() error = %v", err)
	assert.NotNil(t, s, "newJSONStage failed to create the pipeline stage and was nil")
	out := processEntries(s, newEntry(map[string]interface{}{
		"test_label": "unimportant value",
	}, toLabelSet(labels), `{"page": 1, "fruits": ["apple", "peach"]}`, time.Now()))
	assert.Equal(t, 1, len(out), "stage should have kept one valid json line but got %v", out)

	out = processEntries(s, newEntry(map[string]interface{}{
		"test_label": "unimportant value",
	}, toLabelSet(labels), `{"page": 1, fruits": ["apple", "peach"]}`, time.Now()))
	assert.Equal(t, 0, len(out), "stage should have kept zero valid json line but got %v", out)
}

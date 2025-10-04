package stages

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	util_log "github.com/grafana/loki/v3/pkg/util/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestConfigurationValidation(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		config  string
		isValid bool
	}{
		"valid configuration": {
			config:  "stage.json_field{ operations = [{ operation = \"delete\", field = \"foobar\", source = \"\"}]}",
			isValid: true,
		},
		"invalid operation single op": {
			config:  "stage.json_field{ operations = [{ operation = \"add\", field = \"foobar\", source = \"\"}]}",
			isValid: false,
		},
		"invalid operation multiple ops": {
			config:  "stage.json_field{ operations = [{ operation = \"update\", field = \"foobar\", source = \"\"},{ operation = \"delete\", field = \"foobar\", source = \"\"},{ operation = \"add\", field = \"foobar\", source = \"\"}]}",
			isValid: false,
		},
	}

	for testName, testData := range tests {
		testData := testData
		t.Run(testName, func(t *testing.T) {
			pl, err := NewPipeline(util_log.Logger, loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			if testData.isValid {
				assert.NoError(t, err, "Expected pipeline creation to not result in error")
			}
			_ = pl
		})

	}
}

var jsonfieldCfg = `
	drop_malformed = true
	operations = [{
		operation = "update",
		field = "foo",
		source = "bar",
}]
`

func TestJsonFiedAlloy(t *testing.T) {
	t.Parallel()
	// testing that we can use Alloy data into the config structure.
	var got JsonFieldConfig
	err := syntax.Unmarshal([]byte(jsonfieldCfg), &got)
	assert.NoError(t, err, "error while un-marshalling config: %s", err)

	want := JsonFieldConfig{
		DropMalformed: true,
		Operations: []JsonFieldOperation{
			{
				Operation: "update",
				Field:     "foo",
				Source:    "bar",
			},
		},
	}
	assert.True(t, reflect.DeepEqual(got, want), "want: %+v got: %+v", want, got)
}

var testJsonFieldAddWithFieldName = `
stage.json_field{
	operations = [
			{
				operation = "update",
				field = "user_id",
			},
		]
}
`
var testJsonFieldAddArrayWithFieldName = `
stage.json_field{
	operations = [
			{
				operation = "update",
				field = "skus",
			},
		]
}
`

var testJsonFieldAddWithFieldWithSource = `
stage.json_field{
	operations = [
			{
				operation = "update",
				field = "user",
				source = "user_id",
			},
		]
}
`
var testJsonFieldDeleteField = `
stage.json_field{
	operations = [
			{
				operation = "delete",
				field = "component",
			},
		]
}
`
var testJsonFieldDeleteNonExistant = `
stage.json_field{
	operations = [
			{
				operation = "delete",
				field = "foo",
			},
		]
}
`

var testJSONFieldLogLine = `
{
	"component": ["parser","type"],
	"duration" : 125,
	"message" : "this is a log line"
}
`

func TestPipeline_JSONField(t *testing.T) {
	t.Parallel()
	logger := util.TestAlloyLogger(t)
	tests := map[string]struct {
		config      string
		entry       string
		Extracted   map[string]any
		expectedOut map[string]any
	}{
		"add new field from extracted values with field as source": {
			config: testJsonFieldAddWithFieldName,
			entry:  testJSONFieldLogLine,
			Extracted: map[string]any{
				"user_id": "superadmin",
			},
			expectedOut: map[string]any{
				"component": []interface{}{
					"parser",
					"type",
				},
				"duration": float64(125),
				"message":  "this is a log line",
				"user_id":  "superadmin",
			},
		},
		"add new integer field from extracted values with field as source": {
			config: testJsonFieldAddWithFieldName,
			entry:  testJSONFieldLogLine,
			Extracted: map[string]any{
				"user_id": float64(12345),
			},
			expectedOut: map[string]any{
				"component": []any{
					"parser",
					"type",
				},
				"duration": float64(125),
				"message":  "this is a log line",
				"user_id":  float64(12345),
			},
		},
		"add new object field from extracted values with field as source": {
			config: testJsonFieldAddWithFieldName,
			entry:  testJSONFieldLogLine,
			Extracted: map[string]any{
				"user_id": map[string]any{
					"name": "admin",
					"id":   float64(12345),
				},
			},
			expectedOut: map[string]any{
				"component": []any{
					"parser",
					"type",
				},
				"duration": float64(125),
				"message":  "this is a log line",
				"user_id": map[string]any{
					"name": "admin",
					"id":   float64(12345),
				},
			},
		},
		"add new array field from extracted values with field as source": {
			config: testJsonFieldAddArrayWithFieldName,
			entry:  testJSONFieldLogLine,
			Extracted: map[string]any{
				"skus": []any{
					"sku-11",
					"sku-12",
				},
			},
			expectedOut: map[string]any{
				"component": []any{
					"parser",
					"type",
				},
				"duration": float64(125),
				"message":  "this is a log line",
				"skus": []any{
					"sku-11",
					"sku-12",
				},
			},
		},
		"add new field from extracted values with source": {
			config: testJsonFieldAddWithFieldWithSource,
			entry:  testJSONFieldLogLine,
			Extracted: map[string]any{
				"user_id": "superadmin",
			},
			expectedOut: map[string]any{
				"component": []interface{}{
					"parser",
					"type",
				},
				"duration": float64(125),
				"message":  "this is a log line",
				"user":     "superadmin",
			},
		},
		"delete non existant field": {
			config: testJsonFieldDeleteNonExistant,
			entry:  testJSONFieldLogLine,
			expectedOut: map[string]any{
				"component": []interface{}{
					"parser",
					"type",
				},
				"duration": float64(125),
				"message":  "this is a log line",
			},
		},
		"delete non field": {
			config: testJsonFieldDeleteField,
			entry:  testJSONFieldLogLine,
			expectedOut: map[string]any{
				"duration": float64(125),
				"message":  "this is a log line",
			},
		},
	}
	for testName, testData := range tests {
		testData := testData
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(logger, loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			assert.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl, newEntry(testData.Extracted, nil, testData.entry, time.Now()))[0]
			var outMap map[string]any
			err = json.Unmarshal([]byte(out.Line), &outMap)
			assert.NoError(t, err, "failed to unmarshal log line")
			assert.True(t, reflect.DeepEqual(testData.expectedOut, outMap), "want %#v got: %#v", testData.expectedOut, outMap)
		})
	}
}

var testJsonFieldDropMalformed = `
stage.json_field{
	drop_malformed = true
	operations = [
			{
				operation = "update",
				field = "user_id",
			},
		]
}
`
var testJsonFieldDropMalformedFalse = `
stage.json_field{
	drop_malformed = false
	operations = [
			{
				operation = "update",
				field = "user_id",
			},
		]
}
`

func TestPipeline_JSONFieldDrop(t *testing.T) {
	t.Parallel()
	logger := util.TestAlloyLogger(t)
	tests := map[string]struct {
		config    string
		entry     string
		shoudDrop bool
	}{
		"drop_malformed is set to true": {
			config:    testJsonFieldDropMalformed,
			entry:     "This is a log line",
			shoudDrop: true,
		},
		"drop_malformed is set to false": {
			config:    testJsonFieldDropMalformedFalse,
			entry:     "This is a log line",
			shoudDrop: false,
		},
		"drop_malformed is not set": {
			config:    testJsonFieldDropMalformedFalse,
			entry:     "This is a log line",
			shoudDrop: false,
		},
	}

	for testName, testData := range tests {
		testData := testData
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			pl, err := NewPipeline(logger, loadConfig(testData.config), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
			assert.NoError(t, err, "Expected pipeline creation to not result in error")
			out := processEntries(pl, newEntry(nil, nil, testData.entry, time.Now()))
			if testData.shoudDrop {
				assert.Empty(t, out, "log line should be dropped got: %+v", out)
			} else {
				assert.NotEmpty(t, out, "log line should not been dropped")
			}
		})
	}
}

package filter_test

import (
	"testing"

	"github.com/grafana/agent/internal/component/otelcol/processor/filter"
	"github.com/grafana/river"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/stretchr/testify/require"
)

// Source: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/filterprocessor/README.md#filter-spans-from-traces
func TestArguments_UnmarshalRiver(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]interface{}
		errMsg   string
	}{
		{
			testName: "Defaults",
			cfg: `
			output {}
			`,
			expected: map[string]interface{}{
				"error_mode": "propagate",
			},
		},
		{
			testName: "IgnoreErrors",
			cfg: `
			error_mode = "ignore"
			output {}
			`,
			expected: map[string]interface{}{
				"error_mode": "ignore",
			},
		},
		{
			testName: "DropNonHttpSpans",
			cfg: `
			error_mode = "ignore"
			traces {
				span = [
					"attributes[\"http.request.method\"] == nil",
				]
			}
			output {}
			`,
			expected: map[string]interface{}{
				"error_mode": "ignore",
				"traces": map[string]interface{}{
					"span": []interface{}{
						`attributes["http.request.method"] == nil`,
					},
				},
			},
		},
		{
			testName: "FilterForMultipleObs",
			cfg: `
			error_mode = "ignore"
			traces {
				span = [
					"attributes[\"container.name\"] == \"app_container_1\"",
					"resource.attributes[\"host.name\"] == \"localhost\"",
					"name == \"app_1\"",
				]
				spanevent = [
					"attributes[\"grpc\"] == true",
					"IsMatch(name, \".*grpc.*\")",
				]
			}
			metrics {
				metric = [
					"name == \"my.metric\" and resource.attributes[\"my_label\"] == \"abc123\"",
					"type == METRIC_DATA_TYPE_HISTOGRAM",
				]
				datapoint = [
					"metric.type == METRIC_DATA_TYPE_SUMMARY",
					"resource.attributes[\"service.name\"] == \"my_service_name\"",
				]
			}
			logs {
				log_record = [
					"IsMatch(body, \".*password.*\")",
					"severity_number < SEVERITY_NUMBER_WARN",
				]
			}
			output {}
			`,
			expected: map[string]interface{}{
				"error_mode": "ignore",
				"traces": map[string]interface{}{
					"span": []interface{}{
						`attributes["container.name"] == "app_container_1"`,
						`resource.attributes["host.name"] == "localhost"`,
						`name == "app_1"`,
					},
					"spanevent": []interface{}{
						`attributes["grpc"] == true`,
						`IsMatch(name, ".*grpc.*")`,
					},
				},
				"metrics": map[string]interface{}{
					"metric": []interface{}{
						`name == "my.metric" and resource.attributes["my_label"] == "abc123"`,
						`type == METRIC_DATA_TYPE_HISTOGRAM`,
					},
					"datapoint": []interface{}{
						`metric.type == METRIC_DATA_TYPE_SUMMARY`,
						`resource.attributes["service.name"] == "my_service_name"`,
					},
				},
				"logs": map[string]interface{}{
					"log_record": []interface{}{
						`IsMatch(body, ".*password.*")`,
						`severity_number < SEVERITY_NUMBER_WARN`,
					},
				},
			},
		},
		{
			testName: "ValidOtelFilterFunctionUsage",
			cfg: `
			error_mode = "ignore"	
			metrics {
				metric = [
					"HasAttrKeyOnDatapoint(\"http.method\")",
					"HasAttrOnDatapoint(\"http.method\", \"GET\")",
				]
			}
			output {}
			`,
			expected: map[string]interface{}{
				"error_mode": "ignore",
				"metrics": map[string]interface{}{
					"metric": []interface{}{
						`HasAttrKeyOnDatapoint("http.method")`,
						`HasAttrOnDatapoint("http.method", "GET")`,
					},
				},
			},
		},
		{
			testName: "invalidOtelFilterFunctionUsage",
			cfg: `
			error_mode = "ignore"	
			metrics {
				metric = [
					"UnknowFunction(\"http.method\")",
				]
			}
			output {}
			`,
			errMsg: `unable to parse OTTL statement "match() where UnknowFunction(\"http.method\")": undefined function "UnknowFunction"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args filter.Arguments
			err := river.Unmarshal([]byte(tc.cfg), &args)
			if tc.errMsg != "" {
				require.ErrorContains(t, err, tc.errMsg)
				return
			}
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*filterprocessor.Config)

			var expectedCfg filterprocessor.Config
			err = mapstructure.Decode(tc.expected, &expectedCfg)
			require.NoError(t, err)

			// Validate
			require.NoError(t, actual.Validate())
			require.NoError(t, expectedCfg.Validate())

			// Compare
			require.Equal(t, expectedCfg, *actual)
		})
	}
}

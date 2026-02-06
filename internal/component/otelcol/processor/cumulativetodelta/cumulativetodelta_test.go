package cumulativetodelta_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/processor/cumulativetodelta"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]any
	}{
		{
			testName: "Defaults",
			cfg: `
				output {}
			`,
			expected: map[string]any{
				"max_staleness": 0,
				"initial_value": 0,
			},
		},
		{
			testName: "Defaults Match Upstream",
			cfg: `
				output {}
			`,
			expected: map[string]any{},
		},
		{
			testName: "Initial Value Auto",
			cfg: `
				initial_value = "auto"
				output {}
			`,
			expected: map[string]any{
				"initial_value": 0,
			},
		},
		{
			testName: "Initial Value Keep",
			cfg: `
				initial_value = "keep"
				output {}
			`,
			expected: map[string]any{
				"initial_value": 1,
			},
		},
		{
			testName: "Initial Value Drop",
			cfg: `
				initial_value = "drop"
				output {}
			`,
			expected: map[string]any{
				"initial_value": 2,
			},
		},
		{
			testName: "Explicit Values",
			cfg: `
				max_staleness = "24h"
				initial_value = "drop"
				include {
					metrics = ["metric1", "metric2"]
					match_type = "strict"
					metric_types = ["histogram"]
				}
				exclude {
					metrics = [".*metric.*"]
					match_type = "regexp"
					metric_types = ["sum"]
				}
				output {}
			`,
			expected: map[string]any{
				"max_staleness": 86400000000000,
				"initial_value": 2,
				"include": map[string]any{
					"metrics":      []string{"metric1", "metric2"},
					"match_type":   "strict",
					"metric_types": []string{"histogram"},
				},
				"exclude": map[string]any{
					"metrics":      []string{".*metric.*"},
					"match_type":   "regexp",
					"metric_types": []string{"sum"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args cumulativetodelta.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*cumulativetodeltaprocessor.Config)

			var expected cumulativetodeltaprocessor.Config
			err = mapstructure.Decode(tc.expected, &expected)
			require.NoError(t, err)

			require.Equal(t, expected, *actual)
		})
	}
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		testName      string
		cfg           string
		expectedError string
	}{
		{
			testName: "Initial Value",
			cfg: `
				initial_value = "wait"
				output {}
			`,
			expectedError: `initial_value must be one of "auto", "keep", "drop"`,
		},
		{
			testName: "Missing Include Metrics",
			cfg: `
				include {
					match_type = "strict"
				}
				output {}
			`,
			expectedError: "metrics must be supplied if match_type is set",
		},
		{
			testName: "Missing Include Match Type",
			cfg: `
				include {
					metrics = ["metric"]
				}
				output {}
			`,
			expectedError: "match_type must be set if metrics are supplied",
		},
		{
			testName: "Missing Exclude Metrics",
			cfg: `
				exclude {
					match_type = "strict"
				}
				output {}
			`,
			expectedError: "metrics must be supplied if match_type is set",
		},
		{
			testName: "Missing Exclude Match Type",
			cfg: `
				exclude {
					metrics = ["metric"]
				}
				output {}
			`,
			expectedError: "match_type must be set if metrics are supplied",
		},
		{
			testName: "Incorrect Include Match Type",
			cfg: `
				include {
					metrics = ["metric"]
					match_type = "regex"
				}
				output {}
			`,
			expectedError: `match_type must be one of "strict" and "regexp"`,
		},
		{
			testName: "Incorrect Exclude Match Type",
			cfg: `
				exclude {
					metrics = ["metric"]
					match_type = "regex"
				}
				output {}
			`,
			expectedError: `match_type must be one of "strict" and "regexp"`,
		},
		{
			testName: "Incorrect Include Metric Types",
			cfg: `
				include {
					metrics = ["metric"]
					match_type = "regexp"
					metric_types = ["invalid"]
				}
				output {}
			`,
			expectedError: `metric_types must be one of "sum" and "histogram"`,
		},
		{
			testName: "Incorrect Exclude Metric Types",
			cfg: `
				exclude {
					metrics = ["metric"]
					match_type = "regexp"
					metric_types = ["invalid"]
				}
				output {}
			`,
			expectedError: `metric_types must be one of "sum" and "histogram"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args cumulativetodelta.Arguments
			require.ErrorContains(t, syntax.Unmarshal([]byte(tc.cfg), &args), tc.expectedError)
		})
	}
}

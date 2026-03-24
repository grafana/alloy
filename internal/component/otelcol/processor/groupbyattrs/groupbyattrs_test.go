package groupbyattrs_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/processor/groupbyattrs"
	"github.com/grafana/alloy/syntax"

	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]any
		errMsg   string
	}{
		{
			testName: "Default",
			cfg: `
			output {}
			`,
			expected: map[string]any{
				"keys": []string{},
			},
		},
		{
			testName: "SingleKey",
			cfg: `
			keys = ["key1"]
			output {}
			`,
			expected: map[string]any{
				"keys": []string{
					"key1",
				},
			},
		},
		{
			testName: "MultipleKeys",
			cfg: `
			keys = ["key1", "key2"]
			output {}
			`,
			expected: map[string]any{
				"keys": []string{
					"key1",
					"key2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var args groupbyattrs.Arguments
			err := syntax.Unmarshal([]byte(tt.cfg), &args)
			if tt.errMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*groupbyattrsprocessor.Config)

			var expectedCfg groupbyattrsprocessor.Config
			err = mapstructure.Decode(tt.expected, &expectedCfg)
			require.NoError(t, err)

			require.Equal(t, expectedCfg, *actual)
		})
	}
}

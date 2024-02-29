package flowmode

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExtraArgs(t *testing.T) {
	type testCase struct {
		name          string
		extraArgs     string
		expected      []string
		expectedError string
	}

	var testCases = []testCase{
		{
			name:      "integrations next with env vars",
			extraArgs: "-enable-features=integrations-next -config.expand-env",
			expected:  []string{"-enable-features", "integrations-next", "-config.expand-env"},
		},
		{
			name:      "longhand",
			extraArgs: "--key=value",
			expected:  []string{"--key", "value"},
		},
		{
			name:      "shorthand",
			extraArgs: "-k=value",
			expected:  []string{"-k", "value"},
		},
		{
			name:      "bool longhand",
			extraArgs: "--boolVariable",
			expected:  []string{"--boolVariable"},
		},
		{
			name:      "bool shorthand",
			extraArgs: "-b",
			expected:  []string{"-b"},
		},
		{
			name:      "combo",
			extraArgs: "--key=value -k=value --boolVariable -b",
			expected:  []string{"--key", "value", "-k", "value", "--boolVariable", "-b"},
		},
		{
			name:      "spaced",
			extraArgs: "--key value",
			expected:  []string{"--key", "value"},
		},
		{
			name:      "value with equals",
			extraArgs: `--key="foo=bar"`,
			expected:  []string{"--key", `"foo=bar"`},
		},
		{
			name:      "no value",
			extraArgs: "--key=",
			expected:  []string{"--key"},
		},
		{
			name:      "no dashes",
			extraArgs: "key",
			expected:  []string{"key"},
		},
		{
			name:          "no dashes with value",
			extraArgs:     "key=value",
			expectedError: "invalid flag found: key=value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := parseExtraArgs(tc.extraArgs)
			if tc.expectedError != "" {
				require.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, res)
		})
	}
}

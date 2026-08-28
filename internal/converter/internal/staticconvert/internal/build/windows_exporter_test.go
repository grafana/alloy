package build

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_split(t *testing.T) {
	tests := []struct {
		testName       string
		input          string
		expectedOutput []string
	}{
		{
			testName:       "multiple values",
			input:          "example1,example2",
			expectedOutput: []string{"example1", "example2"},
		},
		{
			testName:       "single value",
			input:          "example1",
			expectedOutput: []string{"example1"},
		},
		{
			testName:       "empty string",
			input:          "",
			expectedOutput: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			got := split(tt.input)
			require.Equal(t, tt.expectedOutput, got)
		})
	}
}

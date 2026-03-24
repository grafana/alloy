//go:build windows

package windowspriority

import (
	"testing"

	"github.com/stretchr/testify/require"

	"golang.org/x/sys/windows"
)

func TestWindowsPriority(t *testing.T) {
	tests := []struct {
		name          string
		priority      string
		expectedError string
	}{
		{
			name:     "normal priority",
			priority: "normal",
		},
		{
			name:     "high priority",
			priority: "high",
		},
		{
			name:          "unknown priority",
			priority:      "weird",
			expectedError: "invalid priority: weird",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the priority before each test
			defer func() {
				SetPriority("normal")
			}()

			err := SetPriority(tc.priority)

			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
				return
			}

			runningPriority, err := windows.GetPriorityClass(windows.CurrentProcess())
			require.NoError(t, err)
			require.Equal(t, priorityMap[tc.priority], runningPriority)
		})
	}
}

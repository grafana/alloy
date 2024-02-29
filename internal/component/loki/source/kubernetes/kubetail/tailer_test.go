package kubetail

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_parseKubernetesLog(t *testing.T) {
	tt := []struct {
		inputLine  string
		expectTS   time.Time
		expectLine string
	}{
		{
			// Test normal RFC3339Nano log line.
			inputLine:  `2023-01-23T17:00:10.000000001Z hello, world!`,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 1, time.UTC),
			expectLine: "hello, world!",
		},
		{
			// Test normal RFC3339 log line.
			inputLine:  `2023-01-23T17:00:10Z hello, world!`,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 0, time.UTC),
			expectLine: "hello, world!",
		},
		{
			// Test empty log line. There will always be a space prepended by
			// Kubernetes.
			inputLine:  `2023-01-23T17:00:10.000000001Z `,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 1, time.UTC),
			expectLine: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.inputLine, func(t *testing.T) {
			actualTS, actualLine := parseKubernetesLog(tc.inputLine)
			require.Equal(t, tc.expectTS, actualTS)
			require.Equal(t, tc.expectLine, actualLine)
		})
	}
}

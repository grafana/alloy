package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lokiURL = "http://localhost:3100/loki/api/v1/"

// LogQuery returns a formatted Loki query with the given test_name label
func LogQuery(testName string) string {
	return fmt.Sprintf("%squery_range?query=%%7Btest_name%%3D%%22%s%%22%%7D", lokiURL, testName)
}

// AssertLogsPresent checks that logs are present in Loki and match expected labels
func AssertLogsPresent(t *testing.T, testName string, expectedLabels map[string]string, expectedCount int) {
	var logResponse LogResponse
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := FetchDataFromURL(LogQuery(testName), &logResponse)
		assert.NoError(c, err)
		if len(logResponse.Data.Result) == 0 {
			return
		}

		// Verify we got all logs
		result := logResponse.Data.Result[0]
		assert.Equal(c, expectedCount, len(result.Values), "should have %d log entries", expectedCount)

		// Verify labels were enriched
		for k, v := range expectedLabels {
			assert.Equal(c, v, result.Stream[k], "label %s should be %s", k, v)
		}
	}, DefaultTimeout, DefaultRetryInterval)
}

// AssertLogsMissing checks that logs with specific labels are not present in Loki
func AssertLogsMissing(t *testing.T, testName string, labels ...string) {
	var logResponse LogResponse
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := FetchDataFromURL(LogQuery(testName), &logResponse)
		assert.NoError(c, err)
		if len(logResponse.Data.Result) == 0 {
			return
		}

		result := logResponse.Data.Result[0]
		for _, label := range labels {
			assert.NotContains(c, result.Stream, label, "label %s should not be present", label)
		}
	}, DefaultTimeout, DefaultRetryInterval)
}

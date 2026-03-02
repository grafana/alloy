package common

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lokiURL = "http://localhost:3100/loki/api/v1/"

// LogQuery returns a formatted Loki query with the given test_name label
func LogQuery(testName string) string {
	// https://grafana.com/docs/loki/latest/reference/loki-http-api/#query-logs-within-a-range-of-time
	queryFilter := fmt.Sprintf("{test_name=\"%s\"}", testName)
	query := fmt.Sprintf("%squery_range?query=%s", lokiURL, url.QueryEscape(queryFilter))

	// Loki queries require a nanosecond unix timestamp for the start time.
	if startingAt := AlloyStartTimeUnixNano(); startingAt > 0 {
		query += fmt.Sprintf("&start=%d", startingAt)
	}

	return query
}

type ExpectedLogResult struct {
	Labels     map[string]string
	EntryCount int
}

// AssertLogsPresent checks that logs are present in Loki and match expected labels
func AssertLogsPresent(t *testing.T, expected ...ExpectedLogResult) {
	t.Helper()
	AssertStatefulTestEnv(t)

	var logResponse LogResponse

	require.Eventually(t, func() bool {
		_, err := FetchDataFromURL(LogQuery(SanitizeTestName(t)), &logResponse)
		require.NoError(t, err)
		return len(logResponse.Data.Result) == len(expected)
	}, TestTimeoutEnv(t), DefaultRetryInterval)

	for _, e := range expected {
		values, ok := findStream(e.Labels, logResponse.Data.Result)
		require.True(t, ok, "no stream with labels %s", e.Labels)
		assert.Len(t, values, e.EntryCount)
	}
}

// findStream will try to find a stream from result contaning all label value pairs provided.
// It will return the first match.
func findStream(labels map[string]string, result []LogData) ([][2]string, bool) {
	for _, r := range result {
		toFind := len(labels)
		for k, v := range r.Stream {
			if rv := labels[k]; rv == v {
				toFind -= 1
			}

			if toFind == 0 {
				return r.Values, true
			}
		}

	}

	return nil, false
}

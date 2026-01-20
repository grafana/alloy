package common

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lokiURL = "http://localhost:3100/loki/api/v1/"

// LogQuery returns a formatted Loki query with the given test_name label
func LogQuery(testName string, limit int) string {
	// https://grafana.com/docs/loki/latest/reference/loki-http-api/#query-logs-within-a-range-of-time
	queryFilter := fmt.Sprintf("{test_name=\"%s\"}", testName)
	query := fmt.Sprintf("%squery_range?query=%s&limit=%d", lokiURL, url.QueryEscape(queryFilter), limit)

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

	var (
		totalExpected int
		logResponse   LogResponse
	)

	for _, e := range expected {
		totalExpected += e.EntryCount
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := FetchDataFromURL(LogQuery(SanitizeTestName(t), totalExpected), &logResponse)
		require.NoError(c, err)
		require.Len(c, logResponse.Data.Result, len(expected))

		var totalRecv int
		for _, r := range logResponse.Data.Result {
			totalRecv += len(r.Values)
		}

		require.Equal(c, totalExpected, totalRecv)
	}, TestTimeoutEnv(t), DefaultRetryInterval)

	for _, e := range expected {
		values, ok := findStream(e.Labels, logResponse.Data.Result)
		require.True(t, ok, "no stream with labels %s", e.Labels)
		assert.Len(t, values, e.EntryCount)
	}
}

// WaitForInitalLogs will try to wait until any logs can be retrieved from loki for testName.
// It will return an error if no logs are found after test timeout.
func WaitForInitalLogs(testName string) error {
	var (
		after = time.After(DefaultTimeout)
		tick  = time.NewTicker(DefaultRetryInterval)
	)

	for {
		select {
		case <-tick.C:
			var resp LogResponse
			_, err := FetchDataFromURL(LogQuery(testName, 1), &resp)
			if err != nil {
				continue
			}

			// We start seeing initial logs
			if len(resp.Data.Result) > 0 {
				return nil
			}
		case <-after:
			return errors.New("faild to get first log")
		}
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

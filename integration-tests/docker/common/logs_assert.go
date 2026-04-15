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

// AssertLogsPresent checks that logs are present in Loki and match expected labels.
func AssertLogsPresent(t *testing.T, totalCount int, expected ...ExpectedLogResult) {
	t.Helper()
	AssertStatefulTestEnv(t)

	var logResponse LogResponse

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := FetchDataFromURL(LogQuery(SanitizeTestName(t), totalCount), &logResponse)
		require.NoError(c, err)

		var totalRecv int
		for _, r := range logResponse.Data.Result {
			totalRecv += len(r.Values)
		}

		require.Equal(c, totalCount, totalRecv)
	}, TestTimeoutEnv(t), DefaultRetryInterval)

	for _, e := range expected {
		got := countMatchingEntries(e.Labels, logResponse.Data.Result)
		require.Greater(t, got, 0, "no stream with labels %s", e.Labels)
		assert.Equal(t, e.EntryCount, got, "unexpected entry count for labels %s", e.Labels)
	}
}

// AssertLabelsNotIndexed checks that the given label names are not present in Loki stream indexes for this test.
// This should be call after all logs have been ingested into loki.
func AssertLabelsNotIndexed(t *testing.T, labels ...string) {
	t.Helper()
	AssertStatefulTestEnv(t)

	var resp LogSeriesResponse
	_, err := FetchDataFromURL(LogSeriesQuery(SanitizeTestName(t)), &resp)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Data, "no Loki series found for test; call AssertLogsPresent before AssertLabelsNotIndexed")

	for _, series := range resp.Data {
		for _, label := range labels {
			if _, ok := series[label]; ok {
				require.Failf(t, "indexed label present", "label %q was unexpectedly indexed in series %v", label, series)
			}
		}
	}
}

// LogSeriesQuery returns a Loki series query scoped to the given test_name label.
func LogSeriesQuery(testName string) string {
	queryFilter := fmt.Sprintf("{test_name=\"%s\"}", testName)
	query := fmt.Sprintf("%sseries?match[]=%s", lokiURL, url.QueryEscape(queryFilter))

	if startingAt := AlloyStartTimeUnixNano(); startingAt > 0 {
		query += fmt.Sprintf("&start=%d", startingAt)
	}

	return query
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

// countMatchingEntries returns the total number of log entries across all
// streams whose labels are a superset of the provided label set. This is
// resilient to Loki splitting a logical label set across multiple streams
// (e.g. when additional labels like service_name are injected).
func countMatchingEntries(labels map[string]string, result []LogData) int {
	total := 0
	for _, r := range result {
		if streamContainsLabels(r.Stream, labels) {
			total += len(r.Values)
		}
	}
	return total
}

func streamContainsLabels(stream, labels map[string]string) bool {
	for k, v := range labels {
		if stream[k] != v {
			return false
		}
	}
	return true
}

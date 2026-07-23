package common

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/internal/lokihttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lokiURL = "http://localhost:3100/loki/api/v1/"

// LabelMatcher is an extra Loki stream selector matcher (e.g. op="create_statement")
// that callers can pass to LogQuery to narrow the query beyond test_name.
type LabelMatcher struct {
	Name, Value string
}

// LogQuery returns a formatted Loki query selecting on the given test_name
// label and any additional label matchers.
func LogQuery(testName string, limit int, extraLabelMatchers ...LabelMatcher) string {
	// https://grafana.com/docs/loki/latest/reference/loki-http-api/#query-logs-within-a-range-of-time
	parts := []string{fmt.Sprintf(`test_name=%q`, testName)}
	for _, m := range extraLabelMatchers {
		parts = append(parts, fmt.Sprintf(`%s=%q`, m.Name, m.Value))
	}
	queryFilter := "{" + strings.Join(parts, ", ") + "}"
	query := fmt.Sprintf("%squery_range?query=%s&limit=%d", lokiURL, url.QueryEscape(queryFilter), limit)

	// Loki queries require a nanosecond unix timestamp for the start time.
	if startingAt := AlloyStartTimeUnixNano(); startingAt > 0 {
		query += fmt.Sprintf("&start=%d", startingAt)
	}

	return query
}

type ExpectedLogResult struct {
	EntryCount         int
	StructuredMetadata map[string]string
	Labels             map[string]string
}

// AssertLogsPresent checks that logs are present in Loki and match expected labels.
func AssertLogsPresent(t *testing.T, totalCount int, expected ...ExpectedLogResult) {
	t.Helper()
	AssertStatefulTestEnv(t)

	var logResponse lokihttp.LogResponse

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := fetchLokiQueryRange(SanitizeTestName(t), totalCount, &logResponse)
		require.NoError(c, err)

		var totalRecv int
		for _, r := range logResponse.Data.Result {
			totalRecv += len(r.Values)
		}

		require.Equal(c, totalCount, totalRecv)
	}, TestTimeout(t), DefaultRetryInterval)

	for _, e := range expected {
		entries := matchingEntries(e.Labels, logResponse.Data.Result)
		require.NotEmpty(t, entries, "no stream with labels %s", e.Labels)
		assert.Len(t, entries, e.EntryCount, "unexpected entry count for labels %s", e.Labels)

		for _, entry := range entries {
			for key, expectedValue := range e.StructuredMetadata {
				actualValue := entry.Metadata.StructuredMetadata[key]
				assert.Equal(t, expectedValue, actualValue)
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

// AssertLabelsNotIndexed checks that the given label names are not present in Loki stream indexes for this test.
// This should be call after all logs have been ingested into loki.
func AssertLabelsNotIndexed(t *testing.T, labels ...string) {
	t.Helper()
	AssertStatefulTestEnv(t)

	var resp lokihttp.LogSeriesResponse
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

// WaitForInitalLogs will try to wait until any logs can be retrieved from loki for testName.
// It will return an error if no logs are found after test timeout.
func WaitForInitalLogs(testName string) error {
	var (
		after = time.After(TestTimeout(nil))
		tick  = time.NewTicker(DefaultRetryInterval)
	)

	for {
		select {
		case <-tick.C:
			var resp lokihttp.LogResponse

			err := fetchLokiQueryRange(testName, 1, &resp)
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

func fetchLokiQueryRange(testName string, totalExpected int, res *lokihttp.LogResponse) error {
	// We need to set this header for loki to pass structured_metadata for every entry and
	// not return it as a label in stream.
	const (
		lokiEncodingHeader      = "X-Loki-Response-Encoding-Flags"
		lokiEncodingHeaderValue = "categorize-labels"
	)

	_, err := FetchDataFromURLWithHeaders(
		LogQuery(testName, totalExpected),
		map[string]string{lokiEncodingHeader: lokiEncodingHeaderValue},
		res,
	)
	return err
}

// matchingEntries returns all log entries across streams whose labels are a
// superset of the provided label set.
func matchingEntries(labels map[string]string, result []lokihttp.LogData) []lokihttp.LogEntry {
	var entries []lokihttp.LogEntry
	for _, r := range result {
		if streamContainsLabels(r.Stream, labels) {
			entries = append(entries, r.Values...)
		}
	}
	return entries
}

func streamContainsLabels(stream, labels map[string]string) bool {
	for k, v := range labels {
		if stream[k] != v {
			return false
		}
	}
	return true
}

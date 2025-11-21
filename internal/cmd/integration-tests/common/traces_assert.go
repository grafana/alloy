package common

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tempoURL = "http://localhost:3200/api/"

// TempoTraceSearchResponse represents the response from Tempo's trace search API
type TempoTraceSearchResponse struct {
	Traces []TraceInfo `json:"traces"`
}

// TraceInfo represents basic trace information from Tempo search
type TraceInfo struct {
	TraceID           string            `json:"traceID"`
	RootServiceName   string            `json:"rootServiceName"`
	RootTraceName     string            `json:"rootTraceName"`
	StartTimeUnixNano string            `json:"startTimeUnixNano"`
	DurationMs        float64           `json:"durationMs"`
	SpanSet           map[string]string `json:"spanSet,omitempty"`
}

// TempoSearchQuery builds a Tempo search query URL for traces with given tags
func TempoSearchQuery(tags map[string]string) string {
	query := fmt.Sprintf("%ssearch?", tempoURL)
	for key, value := range tags {
		query += fmt.Sprintf("tags=%s=%s&", key, value)
	}
	// Add time range - look for traces in the last 5 minutes
	end := time.Now().Unix()
	start := end - 300 // 5 minutes ago
	query += fmt.Sprintf("start=%d&end=%d", start, end)
	return query
}

// TracesTest checks that traces with the given tags are stored in Tempo
func TracesTest(t *testing.T, tags map[string]string, testName string) {
	if tags == nil {
		tags = make(map[string]string)
	}
	tags["test_name"] = testName

	AssertTracesAvailable(t, tags)
}

// AssertTracesAvailable performs a Tempo search query and expects to eventually find traces with the given tags
func AssertTracesAvailable(t *testing.T, tags map[string]string) {
	query := TempoSearchQuery(tags)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var searchResponse TempoTraceSearchResponse
		_, err := FetchDataFromURL(query, &searchResponse)
		assert.NoError(c, err)
		assert.NotEmpty(c, searchResponse.Traces, "Expected to find traces matching the search criteria")

		// Additional validation - ensure we have actual trace data
		if len(searchResponse.Traces) > 0 {
			trace := searchResponse.Traces[0]
			assert.NotEmpty(c, trace.TraceID, "Trace should have a valid trace ID")
			assert.NotEmpty(c, trace.RootServiceName, "Trace should have a root service name")

			// TODO (erikbaranowski): Some more intrusive changes may be possible to handle this
			// but this will unblock CI flakiness. Consider looping on the traces and finding
			// one with a non-zero duration or finding a way that trace duration isn't rounded to 0.
			assert.GreaterOrEqual(c, trace.DurationMs, 0.0, "Trace duration should be non-negative")
			if trace.DurationMs == 0.0 {
				t.Logf("Note: Trace has zero duration (TraceID: %s). This can occur with very fast eBPF-instrumented operations.", trace.TraceID)
			}
		}
	}, DefaultTimeout, DefaultRetryInterval, "No traces found matching the search criteria within the time limit")
}

func (t *TempoTraceSearchResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, t)
}

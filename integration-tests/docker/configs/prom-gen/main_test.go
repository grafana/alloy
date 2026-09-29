package main

import (
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestSyntheticTargetSize(t *testing.T) {
	for _, count := range []int{1, 25, 700} {
		reg := prometheus.NewRegistry()
		setupSyntheticMetrics(reg, count)
		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		var previous string
		for scrape := 0; scrape < 2; scrape++ {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
			require.Equal(t, 200, response.Code)
			parser := expfmt.NewTextParser(model.UTF8Validation)
			families, err := parser.TextToMetricFamilies(response.Body)
			require.NoError(t, err)
			require.Len(t, families, 1)
			metrics := families["demo_target_series"].GetMetric()
			require.Len(t, metrics, count)
			seen := map[string]bool{}
			for _, metric := range metrics {
				require.Len(t, metric.Label, 1)
				id := metric.Label[0].GetValue()
				require.False(t, seen[id], "series labels must be unique")
				seen[id] = true
			}
			// Re-gathering must preserve both identities and values.
			current, err := reg.Gather()
			require.NoError(t, err)
			if scrape > 0 {
				require.Equal(t, previous, current[0].String())
			}
			previous = current[0].String()
		}
	}
}

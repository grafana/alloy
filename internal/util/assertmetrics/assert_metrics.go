package assertmetrics

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
)

func ReadMetrics(t assert.TestingT, reg *prometheus.Registry) string {
	// Start server to expose prom metrics
	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer srv.Close()

	// Get the metrics body
	resp, err := http.Get(fmt.Sprintf("%s/metrics", srv.URL))
	assert.NoError(t, err, "error fetching metrics")

	// Return body as text
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err, "error reading response body")
	assert.NoError(t, resp.Body.Close(), "error closing response body")
	return string(body)
}

func AssertValueInReg(t assert.TestingT, reg *prometheus.Registry, metricName string, labels labels.Labels, value float64) {
	AssertValueInStr(t, ReadMetrics(t, reg), metricName, labels, value)
}

func AssertValueInStr(t assert.TestingT, allMetrics string, metricName string, labels labels.Labels, value float64) {
	ls := ""
	if labels.Len() != 0 {
		ls = strings.ReplaceAll(labels.String(), ", ", ",")
	}

	// NOTE: currently no support for exemplars or explicit timestamps
	expectedMetric := fmt.Sprintf("%s%s %v", metricName, ls, value)
	assert.Contains(t, allMetrics, expectedMetric, "expected metric not found")
}

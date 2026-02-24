//go:build windows

package util

import (
	"io"
	"net/http"
	"time"

	"github.com/stretchr/testify/assert"
)

// AssertMetricsEndpoint fetches /metrics and requires a specific metric to be present.
func AssertMetricsEndpoint(c *assert.CollectT, metricsURL string, metricName string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(metricsURL)
	assert.NoError(c, err, "metrics endpoint should be reachable")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	assert.Equal(c, http.StatusOK, resp.StatusCode, "metrics endpoint returned %s", resp.Status)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(c, err)
	assert.Contains(c, string(body), metricName, "metrics response missing metric %q", metricName)
}

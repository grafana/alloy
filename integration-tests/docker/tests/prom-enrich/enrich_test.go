//go:build alloyintegrationtests

package prom_enrich

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestEnrichPromMetrics(t *testing.T) {
	testName := "enrich_prom_metrics"
	common.AssertMetricsAvailable(t, common.PromDefaultMetrics, []string{}, testName)

	var resp MetricsResponse
	_, err := common.FetchDataFromURL(common.MetricsQuery(testName), &resp)
	assert.NoError(t, err)

	expectedLabels := map[string]string{
		"environment": "production",
		"datacenter":  "us-east",
	}

	for _, metric := range resp.Data {
		for name, value := range expectedLabels {
			assert.Equal(t, value, metric[name],
				"label %s should be %s for metric %s", name, value, metric["__name__"])
		}
	}
}

func TestEnrichMismatched(t *testing.T) {
	testName := "enrich_mismatched"
	common.AssertMetricsAvailable(t, common.PromDefaultMetrics, []string{}, testName)

	var resp MetricsResponse
	_, err := common.FetchDataFromURL(common.MetricsQuery(testName), &resp)
	assert.NoError(t, err)

	missingLabels := []string{
		"environment",
		"datacenter",
	}

	for _, metric := range resp.Data {
		for _, name := range missingLabels {
			assert.Equal(t, "", metric[name],
				"label %s should be empty for metric %s", name, metric["__name__"])
		}
	}
}

type MetricsResponse struct {
	Status string              `json:"status"`
	Data   []map[string]string `json:"data"`
}

func (m *MetricsResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

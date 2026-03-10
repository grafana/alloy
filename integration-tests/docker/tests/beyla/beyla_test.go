//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/assert"
)

func TestBeylaMetrics(t *testing.T) {
	var beylaMetrics = []string{
		"beyla_internal_build_info",                // check that internal Beyla metrics are reported
		"http_server_request_duration_seconds_sum", // check that the target metrics are reported
	}
	common.MimirMetricsTest(t, beylaMetrics, []string{}, "beyla")
}

func TestBeylaTraces(t *testing.T) {
	// Test that traces are being generated and sent to Tempo
	tags := map[string]string{
		"service.name": "main", // This should match the instrumented app
	}
	common.TracesTest(t, tags, "beyla")
}

// Test that checks that the Beyla version is correctly injected into the binary.
func TestBeylaVersion(t *testing.T) {
    var metricResponse common.MetricResponse
    require.EventuallyWithT(t, func(c *assert.CollectT) {
        _, err := common.FetchDataFromURL(
            common.MetricQuery("beyla_build_info", "beyla"),
            &metricResponse,
        )
        // Ensure the version label is not the default "unset" value
        assert.NoError(c, err)
        if assert.NotEmpty(c, metricResponse.Data.Result) {
            assert.NotEqual(c, "unset", metricResponse.Data.Result[0].Metric.Version)
        }
    }, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}

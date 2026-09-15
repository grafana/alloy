//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/grafana/alloy/integration-tests/internal/lokihttp"
)

const testName = "database_observability_postgres"

func TestDatabaseObservabilityPostgresMetrics(t *testing.T) {
	metrics := []string{
		"database_observability_connection_info",
	}
	common.MimirMetricsTest(t, metrics, []string{}, testName)
}

func TestDatabaseObservabilityPostgresLogs(t *testing.T) {
	common.AssertStatefulTestEnv(t)

	expectedOps := []string{
		"health_status",
		"query_association",
		"query_parsed_table_name",
		"table_detection",
		"create_statement",
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, op := range expectedOps {
			var resp lokihttp.LogResponse
			_, err := common.FetchDataFromURL(
				// fetch one log with exact match for op label
				common.LogQuery(testName, 1, common.LabelMatcher{Name: "op", Value: op}),
				&resp,
			)
			assert.NoError(c, err)
			assert.NotEmpty(c, resp.Data.Result, "expected at least one log with op=%s", op)
		}
	}, common.TestTimeout(t), common.DefaultRetryInterval)
}

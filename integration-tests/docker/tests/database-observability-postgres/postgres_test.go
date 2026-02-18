//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testName = "database_observability_postgres"

func TestDatabaseObservabilityPostgresMetrics(t *testing.T) {
	var metrics = []string{
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
		"schema_detection",
		"table_detection",
		"create_statement",
	}

	var logResponse common.LogResponse

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := common.FetchDataFromURL(common.LogQuery(testName, 100), &logResponse)
		assert.NoError(c, err)

		ops := make(map[string]bool)
		for _, result := range logResponse.Data.Result {
			if op, ok := result.Stream["op"]; ok {
				ops[op] = true
			}
		}

		for _, op := range expectedOps {
			assert.True(c, ops[op], "expected %s logs", op)
		}
	}, common.TestTimeoutEnv(t), common.DefaultRetryInterval)
}

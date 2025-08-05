package collector

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestConnectionInfo(t *testing.T) {
	defer goleak.VerifyNone(t)

	const baseExpectedMetrics = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="%s",engine="%s",engine_version="%s",provider_name="%s",provider_region="%s"} 1
`

	testCases := []struct {
		name            string
		dsn             string
		dbVersion       string
		expectedMetrics string
	}{
		{
			name:            "generic dsn",
			dsn:             "user:pass@tcp(localhost:3306)/schema",
			dbVersion:       "8.0.32",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "mysql", "8.0.32", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "user:pass@tcp(products-db.abc123xyz.us-east-1.rds.amazonaws.com:3306)/schema",
			dbVersion:       "8.0.32",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "mysql", "8.0.32", "aws", "us-east-1"),
		},
		{
			name:            "Azure flexibleservers dsn",
			dsn:             "user:pass@tcp(products-db.mysql.database.azure.com:3306)/schema",
			dbVersion:       "8.0.32",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "mysql", "8.0.32", "azure", "unknown"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:       tc.dsn,
			Registry:  reg,
			DBVersion: tc.dbVersion,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
		require.NoError(t, err)
	}
}

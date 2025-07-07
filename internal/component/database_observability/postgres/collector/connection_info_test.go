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
	database_observability_connection_info{db_instance_identifier="%s",engine="%s",provider_name="%s",provider_region="%s"} 1
`

	testCases := []struct {
		name            string
		dsn             string
		expectedMetrics string
	}{
		{
			name:            "generic dsn",
			dsn:             "postgres://user:pass@localhost:5432/mydb",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "postgres", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "postgres://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:5432/mydb",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "aws", "us-east-1"),
		},
		{
			name:            "Azure flexibleservers dsn",
			dsn:             "postgres://user:pass@products-db.postgres.database.azure.com:5432/mydb",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "azure", "unknown"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:      tc.dsn,
			Registry: reg,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
		require.NoError(t, err)
	}
}

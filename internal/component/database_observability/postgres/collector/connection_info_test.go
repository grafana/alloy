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
	database_observability_connection_info{db_instance_identifier="%s",engine="%s",engine_version="%s",engine_version_suffix="%s",provider_name="%s",provider_region="%s"} 1
`

	testCases := []struct {
		name                string
		dsn                 string
		engineVersion       string
		engineVersionSuffix string
		expectedMetrics     string
	}{
		{
			name:            "generic dsn",
			dsn:             "postgres://user:pass@localhost:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "postgres", "15.4", "none", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "postgres://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "none", "aws", "us-east-1"),
		},
		{
			name:                "Azure flexibleservers dsn",
			dsn:                 "postgres://user:pass@products-db.postgres.database.azure.com:5432/mydb",
			engineVersion:       "15.4",
			engineVersionSuffix: "none",
			expectedMetrics:     fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "none", "azure", "unknown"),
		},
		{
			name:            "version suffix",
			dsn:             "postgres://user:pass@localhost:5432/mydb",
			engineVersion:   "15.4 (Debian 15.4-1.pgdg120+1)",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "postgres", "15.4", "(Debian 15.4-1.pgdg120+1)", "unknown", "unknown"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:       tc.dsn,
			Registry:  reg,
			DBVersion: tc.engineVersion,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
		require.NoError(t, err)
	}
}

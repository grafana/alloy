package collector

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestConnectionInfo(t *testing.T) {
	defer goleak.VerifyNone(t)

	const baseExpectedMetrics = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="%s",engine="%s",engine_version="%s",provider_account="%s",provider_name="%s",provider_region="%s"} 1
`

	testCases := []struct {
		name            string
		dsn             string
		engineVersion   string
		cloudProvider   *database_observability.CloudProvider
		expectedMetrics string
	}{
		{
			name:            "generic dsn",
			dsn:             "postgres://user:pass@localhost:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "postgres", "15.4", "unknown", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "postgres://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "unknown", "aws", "us-east-1"),
		},
		{
			name:          "AWS/RDS dsn with cloud provider info supplied",
			dsn:           "postgres://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:5432/mydb",
			engineVersion: "15.4",
			cloudProvider: &database_observability.CloudProvider{
				AWS: &database_observability.AWSCloudProviderInfo{
					ARN: arn.ARN{
						Region:    "us-east-1",
						AccountID: "some-account-123",
						Resource:  "db:products-db",
					},
				},
			},
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "some-account-123", "aws", "us-east-1"),
		},
		{
			name:            "Azure flexibleservers dsn",
			dsn:             "postgres://user:pass@products-db.postgres.database.azure.com:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "unknown", "azure", "unknown"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:           tc.dsn,
			Registry:      reg,
			EngineVersion: tc.engineVersion,
			CloudProvider: tc.cloudProvider,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
		require.NoError(t, err)
	}
}

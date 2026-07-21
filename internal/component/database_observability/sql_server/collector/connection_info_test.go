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
			dsn:             "sqlserver://user:pass@localhost:1433",
			engineVersion:   "16.0.1000.6",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "sql_server", "16.0.1000.6", "unknown", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "sqlserver://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:1433",
			engineVersion:   "16.0.1000.6",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "sql_server", "16.0.1000.6", "unknown", "aws", "us-east-1"),
		},
		{
			name:          "AWS/RDS dsn with cloud provider info supplied",
			dsn:           "sqlserver://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:1433",
			engineVersion: "16.0.1000.6",
			cloudProvider: &database_observability.CloudProvider{
				AWS: &database_observability.AWSCloudProviderInfo{
					ARN: arn.ARN{
						Region:    "us-east-1",
						AccountID: "some-account-123",
						Resource:  "db:products-db",
					},
				},
			},
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "sql_server", "16.0.1000.6", "some-account-123", "aws", "us-east-1"),
		},
		{
			name:            "Azure dsn",
			dsn:             "sqlserver://user:pass@products-db.database.windows.net:1433",
			engineVersion:   "16.0.1000.6",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "sql_server", "16.0.1000.6", "unknown", "azure", "unknown"),
		},
		{
			name:            "Azure managed instance dsn",
			dsn:             "sqlserver://user:pass@products-db.abc123.database.windows.net:1433",
			engineVersion:   "16.0.1000.6",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "sql_server", "16.0.1000.6", "unknown", "azure", "unknown"),
		},
		{
			name:          "Azure with cloud provider info supplied",
			dsn:           "sqlserver://user:pass@products-db.database.windows.net:1433",
			engineVersion: "16.0.1000.6",
			cloudProvider: &database_observability.CloudProvider{
				Azure: &database_observability.AzureCloudProviderInfo{
					ServerName:     "products-db",
					SubscriptionID: "sub-12345-abcde",
					ResourceGroup:  "my-resource-group",
				},
			},
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "sql_server", "16.0.1000.6", "sub-12345-abcde", "azure", "my-resource-group"),
		},
		{
			name:          "GCP with cloud provider info supplied",
			dsn:           "sqlserver://user:pass@10.0.0.1:1433",
			engineVersion: "16.0.1000.6",
			cloudProvider: &database_observability.CloudProvider{
				GCP: &database_observability.GCPCloudProviderInfo{
					ProjectID:  "my-gcp-project",
					Region:     "us-central1",
					InstanceID: "my-cloud-sql-instance",
				},
			},
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "my-cloud-sql-instance", "sql_server", "16.0.1000.6", "my-gcp-project", "gcp", "us-central1"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()

			collector, err := NewConnectionInfo(ConnectionInfoArguments{
				DSN:           tc.dsn,
				Registry:      reg,
				EngineVersion: tc.engineVersion,
				CloudProvider: tc.cloudProvider,
				DB:            nil, // no DB in tests: goroutine not started, metric stays set
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			err = collector.Start(t.Context())
			require.NoError(t, err)

			err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
			require.NoError(t, err)
		})
	}
}

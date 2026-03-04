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

func TestConnectionInfo_Unregister(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	reg := prometheus.NewRegistry()
	c, err := NewConnectionInfo(ConnectionInfoArguments{
		DSN:           "postgres://user:pass@localhost:5432/mydb",
		Registry:      reg,
		EngineVersion: "15.4",
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))

	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, mfs, 1, "metric should be present before Unregister")

	c.Unregister()

	mfs, err = reg.Gather()
	require.NoError(t, err)
	require.Empty(t, mfs, "metric should be absent after Unregister")
	require.False(t, c.IsRegistered())
}

func TestConnectionInfo_Reregister(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	reg := prometheus.NewRegistry()
	c, err := NewConnectionInfo(ConnectionInfoArguments{
		DSN:           "postgres://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:5432/mydb",
		Registry:      reg,
		EngineVersion: "15.4",
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context()))

	c.Unregister()
	require.False(t, c.IsRegistered())

	c.Reregister()
	require.True(t, c.IsRegistered())

	const expected = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="products-db",engine="postgres",engine_version="15.4",provider_account="unknown",provider_name="aws",provider_region="us-east-1"} 1
`
	err = testutil.GatherAndCompare(reg, strings.NewReader(expected))
	require.NoError(t, err, "metric should be restored with original label values after Reregister")
}

func TestConnectionInfo(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

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
			name:          "Azure with cloud provider info supplied",
			dsn:           "postgres://user:pass@products-db.postgres.database.azure.com:5432/mydb",
			engineVersion: "15.4",
			cloudProvider: &database_observability.CloudProvider{
				Azure: &database_observability.AzureCloudProviderInfo{
					ServerName:     "products-db",
					SubscriptionID: "sub-12345-abcde",
					ResourceGroup:  "my-resource-group",
				},
			},
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "sub-12345-abcde", "azure", "my-resource-group"),
		},
		{
			name:            "Azure flexibleservers dsn",
			dsn:             "postgres://user:pass@products-db.postgres.database.azure.com:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "unknown", "azure", "unknown"),
		},
		{
			name:            "Azure privatelink dsn",
			dsn:             "postgres://user:pass@products-db.privatelink.postgres.database.azure.com:5432/mydb",
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

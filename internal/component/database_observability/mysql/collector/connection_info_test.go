package collector

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	# HELP database_observability_connection_up Database connection successful (1) or failed (0)
	# TYPE database_observability_connection_up gauge
	database_observability_connection_up{db_instance_identifier="%s",engine="%s",engine_version="%s",provider_account="%s",provider_name="%s",provider_region="%s"} 0
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
			dsn:             "user:pass@tcp(localhost:3306)/schema",
			engineVersion:   "8.0.32",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "mysql", "8.0.32", "unknown", "unknown", "unknown", "unknown", "mysql", "8.0.32", "unknown", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "user:pass@tcp(products-db.abc123xyz.us-east-1.rds.amazonaws.com:3306)/schema",
			engineVersion:   "8.0.32",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "mysql", "8.0.32", "unknown", "aws", "us-east-1", "products-db", "mysql", "8.0.32", "unknown", "aws", "us-east-1"),
		},
		{
			name:          "AWS/RDS dsn with cloud provider info supplied",
			dsn:           "user:pass@tcp(products-db.abc123xyz.us-east-1.rds.amazonaws.com:3306)/schema",
			engineVersion: "8.0.32",
			cloudProvider: &database_observability.CloudProvider{
				AWS: &database_observability.AWSCloudProviderInfo{
					ARN: arn.ARN{
						Region:    "us-east-1",
						AccountID: "some-account-123",
						Resource:  "db:products-db",
					},
				},
			},
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "mysql", "8.0.32", "some-account-123", "aws", "us-east-1", "products-db", "mysql", "8.0.32", "some-account-123", "aws", "us-east-1"),
		},
		{
			name:            "Azure flexibleservers dsn",
			dsn:             "user:pass@tcp(products-db.mysql.database.azure.com:3306)/schema",
			engineVersion:   "8.0.32",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "mysql", "8.0.32", "unknown", "azure", "unknown", "products-db", "mysql", "8.0.32", "unknown", "azure", "unknown"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:           tc.dsn,
			Registry:      reg,
			EngineVersion: tc.engineVersion,
			CloudProvider: tc.cloudProvider,
			CheckInterval: time.Second,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		err = collector.Start(t.Context())
		require.NoError(t, err)
		defer collector.Stop()

		err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
		require.NoError(t, err)
	}
}

func TestConnectionUpFollowsDBPing(t *testing.T) {
	defer goleak.VerifyNone(t)

	const expectedUpOne = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="unknown",engine="mysql",engine_version="8.0.32",provider_account="unknown",provider_name="unknown",provider_region="unknown"} 1
	# HELP database_observability_connection_up Database connection successful (1) or failed (0)
	# TYPE database_observability_connection_up gauge
	database_observability_connection_up{db_instance_identifier="unknown",engine="mysql",engine_version="8.0.32",provider_account="unknown",provider_name="unknown",provider_region="unknown"} 1
`
	const expectedUpZero = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="unknown",engine="mysql",engine_version="8.0.32",provider_account="unknown",provider_name="unknown",provider_region="unknown"} 1
	# HELP database_observability_connection_up Database connection successful (1) or failed (0)
	# TYPE database_observability_connection_up gauge
	database_observability_connection_up{db_instance_identifier="unknown",engine="mysql",engine_version="8.0.32",provider_account="unknown",provider_name="unknown",provider_region="unknown"} 0
`

	// Up == 1 when DB ping succeeds
	{
		reg := prometheus.NewRegistry()
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:           "user:pass@tcp(localhost:3306)/schema",
			Registry:      reg,
			EngineVersion: "8.0.32",
			CheckInterval: time.Hour,
			DB:            db,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		require.NoError(t, collector.Start(t.Context()))
		defer collector.Stop()

		require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(expectedUpOne)))
	}

	// Up == 0 when DB ping fails
	{
		reg := prometheus.NewRegistry()
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing().WillReturnError(fmt.Errorf("ping error"))

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:           "user:pass@tcp(localhost:3306)/schema",
			Registry:      reg,
			EngineVersion: "8.0.32",
			CheckInterval: time.Hour,
			DB:            db,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)
		require.NoError(t, collector.Start(t.Context()))
		defer collector.Stop()

		require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(expectedUpZero)))
	}
}

package collector

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	# HELP database_observability_connection_up Database connection successful (1) or failed (0)
	# TYPE database_observability_connection_up gauge
	database_observability_connection_up{db_instance_identifier="%s",engine="%s",engine_version="%s",provider_name="%s",provider_region="%s"} 0
`

	testCases := []struct {
		name            string
		dsn             string
		engineVersion   string
		expectedMetrics string
	}{
		{
			name:            "generic dsn",
			dsn:             "postgres://user:pass@localhost:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "postgres", "15.4", "unknown", "unknown", "unknown", "postgres", "15.4", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "postgres://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "aws", "us-east-1", "products-db", "postgres", "15.4", "aws", "us-east-1"),
		},
		{
			name:            "Azure flexibleservers dsn",
			dsn:             "postgres://user:pass@products-db.postgres.database.azure.com:5432/mydb",
			engineVersion:   "15.4",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "postgres", "15.4", "azure", "unknown", "products-db", "postgres", "15.4", "azure", "unknown"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:           tc.dsn,
			Registry:      reg,
			EngineVersion: tc.engineVersion,
			CheckInterval: time.Second,
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		require.NoError(t, collector.Start(t.Context()))
		defer collector.Stop()

		require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics)))
	}
}

func TestConnectionUpFollowsDBPing(t *testing.T) {
	defer goleak.VerifyNone(t)

	const expectedUpOne = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="unknown",engine="postgres",engine_version="15.4",provider_name="unknown",provider_region="unknown"} 1
	# HELP database_observability_connection_up Database connection successful (1) or failed (0)
	# TYPE database_observability_connection_up gauge
	database_observability_connection_up{db_instance_identifier="unknown",engine="postgres",engine_version="15.4",provider_name="unknown",provider_region="unknown"} 1
`
	const expectedUpZero = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="unknown",engine="postgres",engine_version="15.4",provider_name="unknown",provider_region="unknown"} 1
	# HELP database_observability_connection_up Database connection successful (1) or failed (0)
	# TYPE database_observability_connection_up gauge
	database_observability_connection_up{db_instance_identifier="unknown",engine="postgres",engine_version="15.4",provider_name="unknown",provider_region="unknown"} 0
`

	// Up == 1 when DB ping succeeds
	{
		reg := prometheus.NewRegistry()
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectPing()

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:           "postgres://user:pass@localhost:5432/mydb",
			Registry:      reg,
			EngineVersion: "15.4",
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
			DSN:           "postgres://user:pass@localhost:5432/mydb",
			Registry:      reg,
			EngineVersion: "15.4",
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

package collector

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestConnectionInfo_getProviderAndInstanceInfo(t *testing.T) {
	defer goleak.VerifyNone(t)

	const baseExpectedMetrics = `
	# HELP database_observability_connection_info Information about the connection
	# TYPE database_observability_connection_info gauge
	database_observability_connection_info{db_instance_identifier="%s",provider_name="%s",provider_region="%s"} 1
`

	testCases := []struct {
		name            string
		dsn             string
		expectedMetrics string
	}{
		{
			name:            "generic dsn",
			dsn:             "user:pass@tcp(localhost:3306)/schema",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "unknown", "unknown", "unknown"),
		},
		{
			name:            "AWS/RDS dsn",
			dsn:             "user:pass@tcp(products-db.abc123xyz.us-east-1.rds.amazonaws.com:3306)/schema",
			expectedMetrics: fmt.Sprintf(baseExpectedMetrics, "products-db", "aws", "us-east-1"),
		},
	}

	for _, tc := range testCases {
		reg := prometheus.NewRegistry()
		db, _, err := sqlmock.New() // ignore anything related to selectSetupConsumers in this test
		require.NoError(t, err)

		collector, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:            tc.dsn,
			Registry:       reg,
			DB:             db,
			ScrapeInterval: 1 * time.Second,
			Logger:         log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		err = testutil.GatherAndCompare(reg, strings.NewReader(tc.expectedMetrics))
		require.NoError(t, err)

		db.Close()
		collector.Stop()
	}
}

func Test_getSetupConsumers(t *testing.T) {
	t.Run("both consumers enabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectSetupConsumers).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"NAME", "ENABLED"}).
				AddRow("events_statements_cpu", "YES").
				AddRow("events_statements_history", "YES"))

		reg := prometheus.NewRegistry()
		c, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:            "user:pass@tcp(localhost:3306)/schema",
			Registry:       reg,
			DB:             db,
			ScrapeInterval: 1 * time.Second,
			Logger:         log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)

		assert.NoError(t, c.getSetupConsumers(context.Background()))

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(`
			# HELP database_observability_setup_consumer_enabled Whether each performance_schema consumer is enabled (1) or disabled (0)
			# TYPE database_observability_setup_consumer_enabled gauge
			database_observability_setup_consumer_enabled{consumer_name="events_statements_cpu"} 1
			database_observability_setup_consumer_enabled{consumer_name="events_statements_history"} 1
			`)))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("one consumer disabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectSetupConsumers).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"NAME", "ENABLED"}).
				AddRow("events_statements_cpu", "YES").
				AddRow("events_statements_history", "NO"))

		reg := prometheus.NewRegistry()
		c, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:            "user:pass@tcp(localhost:3306)/schema",
			Registry:       reg,
			DB:             db,
			ScrapeInterval: 1 * time.Second,
			Logger:         log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)

		assert.NoError(t, c.getSetupConsumers(context.Background()))

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(`
			# HELP database_observability_setup_consumer_enabled Whether each performance_schema consumer is enabled (1) or disabled (0)
			# TYPE database_observability_setup_consumer_enabled gauge
			database_observability_setup_consumer_enabled{consumer_name="events_statements_cpu"} 1
			database_observability_setup_consumer_enabled{consumer_name="events_statements_history"} 0
			`)))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectSetupConsumers).WillReturnError(fmt.Errorf("some error"))

		c, err := NewConnectionInfo(ConnectionInfoArguments{
			DSN:            "user:pass@tcp(localhost:3306)/schema",
			DB:             db,
			Registry:       prometheus.NewRegistry(),
			ScrapeInterval: 1 * time.Second,
			Logger:         log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)

		assert.Error(t, c.getSetupConsumers(context.Background()))
	})
}

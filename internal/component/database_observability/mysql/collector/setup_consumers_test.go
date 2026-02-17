package collector

import (
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
)

func Test_getSetupConsumers(t *testing.T) {
	t.Run("two consumers enabled", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectSetupConsumers).WithoutArgs().
			WillReturnRows(sqlmock.NewRows([]string{"NAME", "ENABLED"}).
				AddRow("events_statements_cpu", "YES").
				AddRow("events_statements_history", "YES"))

		reg := prometheus.NewRegistry()
		c, err := NewSetupConsumers(SetupConsumersArguments{
			Registry:        reg,
			DB:              db,
			CollectInterval: 1 * time.Second,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)

		assert.NoError(t, c.getSetupConsumers(t.Context()))

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(`
			# HELP database_observability_setup_consumers_enabled Whether each performance_schema consumer is enabled (1) or disabled (0)
			# TYPE database_observability_setup_consumers_enabled gauge
			database_observability_setup_consumers_enabled{consumer_name="events_statements_cpu"} 1
			database_observability_setup_consumers_enabled{consumer_name="events_statements_history"} 1
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
		c, err := NewSetupConsumers(SetupConsumersArguments{
			Registry:        reg,
			DB:              db,
			CollectInterval: 1 * time.Second,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)

		assert.NoError(t, c.getSetupConsumers(t.Context()))

		assert.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(`
			# HELP database_observability_setup_consumers_enabled Whether each performance_schema consumer is enabled (1) or disabled (0)
			# TYPE database_observability_setup_consumers_enabled gauge
			database_observability_setup_consumers_enabled{consumer_name="events_statements_cpu"} 1
			database_observability_setup_consumers_enabled{consumer_name="events_statements_history"} 0
			`)))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error when query fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(selectSetupConsumers).WillReturnError(fmt.Errorf("some error"))

		c, err := NewSetupConsumers(SetupConsumersArguments{
			DB:              db,
			Registry:        prometheus.NewRegistry(),
			CollectInterval: 1 * time.Second,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)

		assert.Error(t, c.getSetupConsumers(t.Context()))
	})
}

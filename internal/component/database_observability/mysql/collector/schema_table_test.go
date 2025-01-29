package collector

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSchemaTable(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	t.Run("detect table schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheTTL:        time.Minute,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"BASE TABLE",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"create_statement",
				}).AddRow(
					"some_schema.some_table",
					"CREATE TABLE some_table (id INT)",
				),
			)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		for _, entry := range lokiEntries {
			require.Equal(t, model.LabelSet{"job": database_observability.JobName}, entry.Labels)
		}
		require.Equal(t, `level=info msg="schema detected" op="schema_detection" instance="mysql-db" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, `level=info msg="table detected" op="table_detection" instance="mysql-db" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, `level=info msg="create table" op="create_statement" instance="mysql-db" schema="some_schema" table="some_table" create_statement="CREATE TABLE some_table (id INT)"`, lokiEntries[2].Line)
	})
	t.Run("detect view schema", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheTTL:        time.Minute,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})

		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow(
					"some_schema",
				),
			)

		mock.ExpectQuery(selectTableName).WithArgs("some_schema").RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"table_type",
					"create_time",
					"update_time",
				}).AddRow(
					"some_table",
					"VIEW",
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
				),
			)

		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"table_name",
					"create_statement",
					"character_set_client",
					"collation_connection",
				}).AddRow(
					"some_schema.some_table",
					"CREATE VIEW some_view (id INT)",
					"some_charset",
					"some_charset_connection",
				),
			)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		for _, entry := range lokiEntries {
			require.Equal(t, model.LabelSet{"job": database_observability.JobName}, entry.Labels)
		}
		require.Equal(t, `level=info msg="schema detected" op="schema_detection" instance="mysql-db" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, `level=info msg="table detected" op="table_detection" instance="mysql-db" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, `level=info msg="create table" op="create_statement" instance="mysql-db" schema="some_schema" table="some_table" create_statement="CREATE VIEW some_view (id INT)"`, lokiEntries[2].Line)
	})
	t.Run("schemas result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheTTL:        time.Minute,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnRows(
			sqlmock.NewRows(
				[]string{"schema_name"},
			).AddRow(
				"some_schema",
			).RowError(1, fmt.Errorf("rs error"))) // error on the second row

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		for _, entry := range lokiEntries {
			require.Equal(t, model.LabelSet{"job": database_observability.JobName}, entry.Labels)
		}
		require.Equal(t, `level=info msg="schema detected" op="schema_detection" instance="mysql-db" schema="some_schema"`, lokiEntries[0].Line)
	})
	t.Run("tables result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheTTL:        time.Minute,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"schema_name",
			}).AddRow(
				"some_schema",
			),
		)
		mock.ExpectQuery(selectTableName).WithArgs("some_schema").WillReturnRows(
			sqlmock.NewRows([]string{
				"table_name",
				"table_type",
				"create_time",
				"update_time",
			}).AddRow(
				"some_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			).RowError(1, fmt.Errorf("rs error")), // error on the second row
		)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 2
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		for _, entry := range lokiEntries {
			require.Equal(t, model.LabelSet{"job": database_observability.JobName}, entry.Labels)
		}
		require.Equal(t, `level=info msg="schema detected" op="schema_detection" instance="mysql-db" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, `level=info msg="table detected" op="table_detection" instance="mysql-db" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
	})
	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewSchemaTable(SchemaTableArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheTTL:        time.Minute,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(selectSchemaName).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"schema_name",
			}).AddRow(
				"some_schema",
			),
		)
		mock.ExpectQuery(selectTableName).WithArgs("some_schema").WillReturnRows(
			sqlmock.NewRows([]string{
				"table_name",
				"table_type",
				"create_time",
				"update_time",
			}).AddRow(
				"some_table",
				"BASE TABLE",
				time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			),
		)
		mock.ExpectQuery("SHOW CREATE TABLE some_schema.some_table").WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"table_name",
				"create_statement",
			}).AddRow(
				"some_schema.some_table",
				"CREATE TABLE some_table (id INT)",
			),
		)

		err = collector.Start(context.Background())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		lokiEntries := lokiClient.Received()
		for _, entry := range lokiEntries {
			require.Equal(t, model.LabelSet{"job": database_observability.JobName}, entry.Labels)
		}
		require.Equal(t, `level=info msg="schema detected" op="schema_detection" instance="mysql-db" schema="some_schema"`, lokiEntries[0].Line)
		require.Equal(t, `level=info msg="table detected" op="table_detection" instance="mysql-db" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
		require.Equal(t, `level=info msg="create table" op="create_statement" instance="mysql-db" schema="some_schema" table="some_table" create_statement="CREATE TABLE some_table (id INT)"`, lokiEntries[2].Line)
	})
}

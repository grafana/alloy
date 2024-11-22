package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSchemaTableRun(t *testing.T) {
	// The goroutine which deletes expired entries runs indefinitely,
	// see https://github.com/hashicorp/golang-lru/blob/v2.0.7/expirable/expirable_lru.go#L79-L80
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/hashicorp/golang-lru/v2/expirable.NewLRU[...].func1"))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki_fake.NewClient(func() {})

	collector, err := NewSchemaTable(SchemaTableArguments{
		DB:             db,
		ScrapeInterval: time.Second,
		EntryHandler:   lokiClient,
		CacheTTL:       time.Minute,
		Logger:         log.NewLogfmtLogger(os.Stderr),
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
			"create_time",
			"update_time",
		}).AddRow(
			"some_table",
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

	err = collector.Run(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 3
	}, 5*time.Second, 100*time.Millisecond)

	collector.Stop()
	lokiClient.Stop()

	lokiEntries := lokiClient.Received()
	for _, entry := range lokiEntries {
		require.Equal(t, model.LabelSet{"job": "integrations/db-o11y"}, entry.Labels)
	}
	require.Equal(t, `level=info msg="schema detected" op="schema_detection" schema="some_schema"`, lokiEntries[0].Line)
	require.Equal(t, `level=info msg="table detected" op="table_detection" schema="some_schema" table="some_table"`, lokiEntries[1].Line)
	require.Equal(t, `level=info msg="create table" op="create_statement" schema="some_schema" table="some_table" create_statement="CREATE TABLE some_table (id INT)"`, lokiEntries[2].Line)

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

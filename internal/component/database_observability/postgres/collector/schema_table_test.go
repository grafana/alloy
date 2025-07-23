package collector

import (
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
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
			InstanceKey:     "postgres-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			CacheEnabled:    false,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectDatabaseName).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"datname",
				}).AddRow(
					"books_store",
				),
			)

		mock.ExpectQuery(selectSchemaNames).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"schema_name",
				}).AddRow("public").
					AddRow("postgis"),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 3
		}, 2*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()

		assert.Len(t, lokiEntries, 3)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_DATABASE_DETECTION, "instance": "postgres-db"}, lokiEntries[0].Labels)
		require.Equal(t, `level="info" db="books_store"`, lokiEntries[0].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[1].Labels)
		require.Equal(t, `level="info" schema="public"`, lokiEntries[1].Line)
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_SCHEMA_DETECTION, "instance": "postgres-db"}, lokiEntries[2].Labels)
		require.Equal(t, `level="info" schema="postgis"`, lokiEntries[2].Line)
	})
}

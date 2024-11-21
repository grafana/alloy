package collector

import (
	"context"
	"os"
	"testing"
	"time"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/prometheus/common/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestQuerySampleRun(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki_fake.NewClient(func() {})

	collector, err := NewQuerySample(QuerySampleArguments{
		DB:             db,
		ScrapeInterval: time.Minute,
		EntryHandler:   lokiClient,
		Logger:         log.NewLogfmtLogger(os.Stderr),
	})
	require.NoError(t, err)
	require.NotNil(t, collector)

	mock.ExpectQuery(selectQuerySamples).WithoutArgs().WillReturnRows(
		sqlmock.NewRows([]string{
			"digest",
			"query_sample_text",
			"query_sample_seen",
			"query_sample_timer_wait",
		}).AddRow(
			"abc123",
			"select * from some_table where id = 1",
			"2024-01-01T00:00:00.000Z",
			"1000",
		),
	)

	err = collector.Run(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(lokiClient.Received()) == 2
	}, 5*time.Second, 100*time.Millisecond)

	collector.Stop()
	lokiClient.Stop()

	lokiEntries := lokiClient.Received()
	for _, entry := range lokiEntries {
		require.Equal(t, model.LabelSet{"job": "integrations/db-o11y"}, entry.Labels)
	}
	require.Equal(t, `level=info msg="query samples fetched" op="query_sample" digest="abc123" query_sample_text="select * from some_table where id = 1" query_sample_seen="2024-01-01T00:00:00.000Z" query_sample_timer_wait="1000" query_redacted="select * from some_table where id = :redacted1"`, lokiEntries[0].Line)
	require.Equal(t, `level=info msg="table name parsed" op="query_parsed_table_name" digest="abc123" table="some_table"`, lokiEntries[1].Line)

	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

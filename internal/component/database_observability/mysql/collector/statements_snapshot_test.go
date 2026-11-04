package collector

import (
	"database/sql"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

const testStatementsLimit = 100

// newTestStatements returns a snapshot that reads the table on every Get.
func newTestStatements(db *sql.DB, excludeSchemas ...string) *StatementsSnapshot {
	return NewStatementsSnapshot(StatementsSnapshotArguments{DB: db, Limit: testStatementsLimit, ExcludeSchemas: excludeSchemas})
}

func TestExplainPlansNeedAStatementsSnapshot(t *testing.T) {
	_, err := NewExplainPlans(ExplainPlansArguments{Logger: slog.New(slog.DiscardHandler)})
	require.ErrorContains(t, err, "explain plans need a statements snapshot")
}

func TestStatementsSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	s := NewStatementsSnapshot(StatementsSnapshotArguments{DB: db, Limit: 2, ExcludeSchemas: []string{"other"}, MaxAge: time.Minute})
	clock := time.Now()
	s.now = func() time.Time { return clock }
	lastSeen := clock.Add(-time.Hour)
	query := fmt.Sprintf(selectTopStatements, buildExcludedSchemasClause([]string{"other"}), 2)

	t.Run("reads the top statements, with empty values for NULL", func(t *testing.T) {
		mock.ExpectQuery(query).WithoutArgs().RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows([]string{"schema_name", "digest", "query_sample_text", "last_seen"}).
				AddRow("s", "d1", "select 1", lastSeen).
				AddRow("s", nil, nil, lastSeen))

		got, err := s.Get(t.Context())
		require.NoError(t, err)
		require.Equal(t, []Statement{
			{SchemaName: "s", Digest: "d1", QuerySampleText: "select 1", LastSeen: lastSeen},
			{SchemaName: "s", LastSeen: lastSeen},
		}, got)
	})

	t.Run("reuses a read younger than MaxAge", func(t *testing.T) {
		clock = clock.Add(time.Minute - time.Second)
		got, err := s.Get(t.Context())
		require.NoError(t, err)
		require.Len(t, got, 2)
	})

	t.Run("reads again once MaxAge has passed", func(t *testing.T) {
		clock = clock.Add(time.Second)
		mock.ExpectQuery(query).WithoutArgs().RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows([]string{"schema_name", "digest", "query_sample_text", "last_seen"}).
				AddRow("s", "d2", "select 2", lastSeen))

		got, err := s.Get(t.Context())
		require.NoError(t, err)
		require.Equal(t, []Statement{{SchemaName: "s", Digest: "d2", QuerySampleText: "select 2", LastSeen: lastSeen}}, got)
	})

	t.Run("a failed read is not cached", func(t *testing.T) {
		clock = clock.Add(time.Minute)
		mock.ExpectQuery(query).WithoutArgs().WillReturnError(fmt.Errorf("connection lost"))
		_, err := s.Get(t.Context())
		require.ErrorContains(t, err, "connection lost")

		mock.ExpectQuery(query).WithoutArgs().RowsWillBeClosed().WillReturnRows(
			sqlmock.NewRows([]string{"schema_name", "digest", "query_sample_text", "last_seen"}))
		got, err := s.Get(t.Context())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

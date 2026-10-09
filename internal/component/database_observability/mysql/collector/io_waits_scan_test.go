package collector

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

func ioWaitsRows(fetch int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
		AddRow("books_store", "books", nil, fetch)
}

func TestIOWaitsScan_ReusesResultWithinMaxAge(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	scan := NewIOWaitsScan(db, nil, util.TestAlloyLogger(t).Slog())
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillReturnRows(ioWaitsRows(1))
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillReturnRows(ioWaitsRows(2))

	first, err := scan.get(t.Context(), time.Hour)
	require.NoError(t, err)
	again, err := scan.get(t.Context(), time.Hour)
	require.NoError(t, err)
	require.Equal(t, first, again, "a second call within maxAge must not scan again")
	require.EqualValues(t, 1, first[0].countFetch)

	// Too old for a zero maxAge: scans again.
	fresh, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, fresh[0].countFetch)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIOWaitsScan_WaitingCallerGivesUpWhenContextIsDone(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	scan := NewIOWaitsScan(db, nil, util.TestAlloyLogger(t).Slog())
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillDelayFor(time.Hour).WillReturnRows(sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}))

	// The first caller holds the scan until its own context is cancelled.
	holderCtx, cancelHolder := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { _, _ = scan.get(holderCtx, 0); close(done) }()
	time.Sleep(100 * time.Millisecond)

	waiterCtx, cancelWaiter := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancelWaiter()
	_, err = scan.get(waiterCtx, 0)
	require.ErrorIs(t, err, waiterCtx.Err(), "a waiting caller must stop waiting when its context is done")

	cancelHolder()
	<-done
}

// TestTableStatsAndIndexStatsShareOneScan guards the point of IOWaitsScan: with
// both collectors enabled, performance_schema is scanned once per cycle.
func TestTableStatsAndIndexStatsShareOneScan(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	// The collectors run concurrently, so their queries arrive in either order.
	mock.MatchExpectationsInOrder(false)

	// A single scan query is expected; a second one would be an unexpected call.
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
			AddRow("books_store", "books", nil, 39).
			AddRow("books_store", "books", "idx_books_title", 7))
	mock.ExpectQuery(fmt.Sprintf(selectTableRowCount, exclusionClause)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"database_name", "table_name", "n_rows"}).AddRow("books_store", "books", 500))
	mock.ExpectQuery(fmt.Sprintf(selectIndexSizeBytes, exclusionClause)).RowsWillBeClosed().
		WillReturnRows(sqlmock.NewRows([]string{"database_name", "table_name", "index_name", "size_bytes", "non_unique"}).
			AddRow("books_store", "books", "idx_books_title", 100, 1))

	registry := prometheus.NewRegistry()
	scan := NewIOWaitsScan(db, nil, util.TestAlloyLogger(t).Slog())
	logger := util.TestAlloyLogger(t).Slog()

	ts, err := NewTableStats(TableStatsArguments{DB: db, Registry: registry, CollectInterval: time.Hour, IOWaits: scan, Logger: logger})
	require.NoError(t, err)
	is, err := NewIndexStats(IndexStatsArguments{DB: db, Registry: registry, CollectInterval: time.Hour, IOWaits: scan, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, ts.Start(t.Context()))
	require.NoError(t, is.Start(t.Context()))

	require.Eventually(t, func() bool {
		return testutil.CollectAndCount(registry, "database_observability_mysql_table_stats_no_idx_fetch_total") == 1 &&
			testutil.CollectAndCount(registry, "database_observability_mysql_index_stats_idx_fetch_total") == 1 &&
			testutil.CollectAndCount(registry, "database_observability_mysql_table_stats_row_count") == 1 &&
			testutil.CollectAndCount(registry, "database_observability_mysql_index_stats_size_bytes") == 1
	}, 5*time.Second, 10*time.Millisecond)

	// Each collector only exposes its own rows of the shared scan.
	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(`
	# HELP database_observability_mysql_table_stats_no_idx_fetch_total Count of index I/O wait events for fetch operations that did not use an index
	# TYPE database_observability_mysql_table_stats_no_idx_fetch_total counter
	database_observability_mysql_table_stats_no_idx_fetch_total{schema="books_store",table="books"} 39
	`), "database_observability_mysql_table_stats_no_idx_fetch_total"))
	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(`
	# HELP database_observability_mysql_index_stats_idx_fetch_total Count of index I/O wait events for fetch operations
	# TYPE database_observability_mysql_index_stats_idx_fetch_total counter
	database_observability_mysql_index_stats_idx_fetch_total{index="idx_books_title",schema="books_store",table="books"} 7
	`), "database_observability_mysql_index_stats_idx_fetch_total"))

	ts.Stop()
	is.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

func rowsFor(schema string, fetch int) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"OBJECT_SCHEMA", "OBJECT_NAME", "INDEX_NAME", "COUNT_FETCH"}).
		AddRow(schema, "t", nil, fetch).
		AddRow(schema, "t", "idx_t", fetch)
}

// schemaRead is the expected result of reading one schema.
type schemaRead struct {
	schema string
	rows   *sqlmock.Rows
	err    error
}

// expectSchemaReads expects the statement for one schema to be prepared once
// and run once per entry, in order, then closed.
func expectSchemaReads(mock sqlmock.Sqlmock, reads ...schemaRead) {
	prepared := mock.ExpectPrepare(selectIOWaitsSchema)
	for _, r := range reads {
		q := prepared.ExpectQuery().WithArgs(r.schema)
		if r.err != nil {
			q.WillReturnError(r.err)
		} else {
			q.WillReturnRows(r.rows)
		}
	}
	prepared.WillBeClosed()
}

func schemaList(names ...string) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"database_name"})
	for _, n := range names {
		rows.AddRow(n)
	}
	return rows
}

func newRestrictedScan(t *testing.T, excluded []string, filters ...SchemaFilter) (*IOWaitsScan, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	scan := NewIOWaitsScan(db, excluded, util.TestAlloyLogger(t).Slog())
	for _, f := range filters {
		scan.use(f)
	}
	return scan, mock
}

func schemasOf(rows []ioWaitsRow) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		if !seen[r.schema] {
			seen[r.schema] = true
			out = append(out, r.schema)
		}
	}
	return out
}

func TestIOWaitsScan_ReadsOnlyIncludedSchemas(t *testing.T) {
	scan, mock := newRestrictedScan(t, nil, SchemaFilter{Include: []string{"hg_%", "app"}})

	mock.ExpectQuery(selectSchemaNames).WillReturnRows(schemaList("app", "hg_a", "hg_b", "hgwarm_1", "mysql", "sys"))
	expectSchemaReads(mock,
		schemaRead{schema: "app", rows: rowsFor("app", 1)},
		schemaRead{schema: "hg_a", rows: rowsFor("hg_a", 1)},
		schemaRead{schema: "hg_b", rows: rowsFor("hg_b", 1)})

	rows, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	require.Equal(t, []string{"app", "hg_a", "hg_b"}, schemasOf(rows))
	// hgwarm_1 does not match and the system schemas are always excluded, so
	// no query was made for them: any further query would fail the mock.
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIOWaitsScan_NeverReadsTheComponentsExcludedSchemas(t *testing.T) {
	// exclude_schemas of the component, which applies to every collector, still
	// holds when a collector selects schemas: they are not read.
	scan, mock := newRestrictedScan(t, []string{"rdsadmin", "scratch"}, SchemaFilter{Include: []string{"%"}})

	mock.ExpectQuery(selectSchemaNames).WillReturnRows(schemaList("app", "rdsadmin", "scratch", "mysql"))
	expectSchemaReads(mock, schemaRead{schema: "app", rows: rowsFor("app", 1)})

	rows, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	require.Equal(t, []string{"app"}, schemasOf(rows))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIOWaitsScan_ReadsTheUnionOfAllFilters(t *testing.T) {
	scan, mock := newRestrictedScan(t, nil,
		SchemaFilter{Include: []string{"a"}},
		SchemaFilter{Include: []string{"b"}})

	mock.ExpectQuery(selectSchemaNames).WillReturnRows(schemaList("a", "b", "c"))
	expectSchemaReads(mock,
		schemaRead{schema: "a", rows: rowsFor("a", 1)},
		schemaRead{schema: "b", rows: rowsFor("b", 1)})

	rows, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, schemasOf(rows))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIOWaitsScan_ScansEverythingInOneStatementWhenAnyFilterIsUnrestricted(t *testing.T) {
	scan, mock := newRestrictedScan(t, nil, SchemaFilter{Include: []string{"a"}}, SchemaFilter{})

	// One statement, no schema listing: the unrestricted collector wants it all.
	mock.ExpectQuery(fmt.Sprintf(selectIOWaits, exclusionClause)).WillReturnRows(rowsFor("a", 1))

	rows, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIOWaitsScan_FailedSchemaKeepsItsPreviousRows(t *testing.T) {
	scan, mock := newRestrictedScan(t, nil, SchemaFilter{Include: []string{"a", "b"}})

	mock.ExpectQuery(selectSchemaNames).WillReturnRows(schemaList("a", "b"))
	expectSchemaReads(mock,
		schemaRead{schema: "a", rows: rowsFor("a", 1)},
		schemaRead{schema: "b", rows: rowsFor("b", 1)})
	mock.ExpectQuery(selectSchemaNames).WillReturnRows(schemaList("a", "b"))
	expectSchemaReads(mock,
		schemaRead{schema: "a", rows: rowsFor("a", 2)},
		schemaRead{schema: "b", err: errors.New("boom")})

	first, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	require.Len(t, first, 4)

	second, err := scan.get(t.Context(), 0)
	require.NoError(t, err)
	byKey := map[string]uint64{}
	for _, r := range second {
		byKey[r.schema+"/"+r.index.String] = r.countFetch
	}
	require.EqualValues(t, 2, byKey["a/"], "schema a is refreshed")
	require.EqualValues(t, 1, byKey["b/"], "schema b keeps the rows of the first scan")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIOWaitsScan_SchemaListingFailureFailsTheScan(t *testing.T) {
	scan, mock := newRestrictedScan(t, nil, SchemaFilter{Include: []string{"a"}})
	mock.ExpectQuery(selectSchemaNames).WillReturnError(errors.New("boom"))

	_, err := scan.get(t.Context(), 0)
	require.Error(t, err)
}

// TestTableStatsAndIndexStatsExposeOnlyTheirOwnSchemas guards the union rule:
// the scan reads what either collector wants, but each collector reports only
// the schemas its own filter selects, in the shared scan and in its own queries.
func TestTableStatsAndIndexStatsExposeOnlyTheirOwnSchemas(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	mock.MatchExpectationsInOrder(false)

	mock.ExpectQuery(selectSchemaNames).WillReturnRows(schemaList("a", "b", "c"))
	expectSchemaReads(mock,
		schemaRead{schema: "a", rows: rowsFor("a", 11)},
		schemaRead{schema: "b", rows: rowsFor("b", 22)})
	mock.ExpectQuery(fmt.Sprintf(selectTableRowCount, exclusionClause)).
		WillReturnRows(sqlmock.NewRows([]string{"database_name", "table_name", "n_rows"}).AddRow("a", "t", 5).AddRow("b", "t", 6).AddRow("c", "t", 7))
	mock.ExpectQuery(fmt.Sprintf(selectIndexSizeBytes, exclusionClause)).
		WillReturnRows(sqlmock.NewRows([]string{"database_name", "table_name", "index_name", "size_bytes", "non_unique"}).
			AddRow("a", "t", "idx_t", 100, 1).AddRow("b", "t", "idx_t", 200, 1).AddRow("c", "t", "idx_t", 300, 1))

	registry := prometheus.NewRegistry()
	scan := NewIOWaitsScan(db, nil, util.TestAlloyLogger(t).Slog())
	logger := util.TestAlloyLogger(t).Slog()

	ts, err := NewTableStats(TableStatsArguments{DB: db, Registry: registry, CollectInterval: time.Hour, IOWaits: scan,
		SchemaFilter: SchemaFilter{Include: []string{"a"}}, Logger: logger})
	require.NoError(t, err)
	is, err := NewIndexStats(IndexStatsArguments{DB: db, Registry: registry, CollectInterval: time.Hour, IOWaits: scan,
		SchemaFilter: SchemaFilter{Include: []string{"b"}}, Logger: logger})
	require.NoError(t, err)
	require.NoError(t, ts.Start(t.Context()))
	require.NoError(t, is.Start(t.Context()))

	require.Eventually(t, func() bool {
		return testutil.CollectAndCount(registry, "database_observability_mysql_table_stats_row_count") == 1 &&
			testutil.CollectAndCount(registry, "database_observability_mysql_index_stats_size_bytes") == 1
	}, 5*time.Second, 10*time.Millisecond)

	// table_stats: only schema a (11 from the shared scan, 5 from its own query).
	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(`
	# HELP database_observability_mysql_table_stats_no_idx_fetch_total Count of index I/O wait events for fetch operations that did not use an index
	# TYPE database_observability_mysql_table_stats_no_idx_fetch_total counter
	database_observability_mysql_table_stats_no_idx_fetch_total{schema="a",table="t"} 11
	# HELP database_observability_mysql_table_stats_row_count Estimated number of rows in this table
	# TYPE database_observability_mysql_table_stats_row_count gauge
	database_observability_mysql_table_stats_row_count{schema="a",table="t"} 5
	`), "database_observability_mysql_table_stats_no_idx_fetch_total", "database_observability_mysql_table_stats_row_count"))

	// index_stats: only schema b.
	require.NoError(t, testutil.CollectAndCompare(registry, strings.NewReader(`
	# HELP database_observability_mysql_index_stats_idx_fetch_total Count of index I/O wait events for fetch operations
	# TYPE database_observability_mysql_index_stats_idx_fetch_total counter
	database_observability_mysql_index_stats_idx_fetch_total{index="idx_t",schema="b",table="t"} 22
	`), "database_observability_mysql_index_stats_idx_fetch_total"))

	ts.Stop()
	is.Stop()
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestIOWaitsQueriesCanUseTheServersIndex guards the condition that makes the
// per-schema read cheap. performance_schema.table_io_waits_summary_by_index_usage
// has one index, on (OBJECT_TYPE, OBJECT_SCHEMA, OBJECT_NAME, INDEX_NAME), so a
// query that does not pin OBJECT_TYPE first scans the whole table.
func TestIOWaitsQueriesCanUseTheServersIndex(t *testing.T) {
	require.Contains(t, selectIOWaitsSchema, "OBJECT_TYPE = 'TABLE' AND OBJECT_SCHEMA = ?")
	require.Contains(t, selectIOWaits, "OBJECT_TYPE = 'TABLE'")
}

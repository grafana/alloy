package collector

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/util"
)

// decodedExplainPlanPayload extracts and base64-decodes the explain_plan_output
// field's raw JSON from a log line, so tests can assert on absent keys
// (json.Marshal's omitempty) rather than just zero-valued struct fields.
func decodedExplainPlanPayload(t *testing.T, line string) string {
	t.Helper()
	_, rest, ok := strings.Cut(line, `explain_plan_output="`)
	require.True(t, ok, "line missing explain_plan_output field: %s", line)
	encoded := strings.TrimSuffix(rest, `"`)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	return string(decoded)
}

func mockExplainPlanRows(rows ...[]driver.Value) *sqlmock.Rows {
	r := sqlmock.NewRows([]string{"query_hash", "query_plan"})
	for _, row := range rows {
		r.AddRow(row...)
	}
	return r
}

// expectExplainPlans sets up the two queries one collect cycle runs: the
// Query Store state preflight and the hash-to-plan resolution query.
func expectExplainPlans(t *testing.T, mock sqlmock.Sqlmock, hashes []string, rows ...[]driver.Value) {
	t.Helper()

	query, args, err := buildExplainPlansStatement(hashes)
	require.NoError(t, err)

	mockSelectQueryStoreState(mock, "READ_WRITE")
	mock.ExpectQuery(query).
		WithArgs(namedArgs(args)...).
		RowsWillBeClosed().
		WillReturnRows(mockExplainPlanRows(rows...))
}

const simplePlanXML = `<ShowPlanXML xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan">
  <BatchSequence><Batch><Statements><StmtSimple><QueryPlan>
    <RelOp PhysicalOp="Table Scan" LogicalOp="Table Scan" EstimateRows="10" EstimatedTotalSubtreeCost="0.1">
      <TableScan><Object Database="[MyDb]" Schema="[dbo]" Table="[Orders]" /></TableScan>
    </RelOp>
  </QueryPlan></StmtSimple></Statements></Batch></BatchSequence>
</ShowPlanXML>`

// planXMLWithCost returns simplePlanXML with a different EstimateRows/cost so
// tests can assert the emission gate ignores exactly those two fields.
func planXMLWithEstimate(rows string, cost string) string {
	return fmt.Sprintf(`<ShowPlanXML xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan">
  <BatchSequence><Batch><Statements><StmtSimple><QueryPlan>
    <RelOp PhysicalOp="Table Scan" LogicalOp="Table Scan" EstimateRows="%s" EstimatedTotalSubtreeCost="%s">
      <TableScan><Object Database="[MyDb]" Schema="[dbo]" Table="[Orders]" /></TableScan>
    </RelOp>
  </QueryPlan></StmtSimple></Statements></Batch></BatchSequence>
</ShowPlanXML>`, rows, cost)
}

func newExplainPlansForTest(t *testing.T, db *sql.DB, entryHandler loki.EntryHandler, tracker QueryTracker) *ExplainPlans {
	t.Helper()

	c, err := NewExplainPlans(ExplainPlansArguments{
		DB:              db,
		CollectInterval: time.Minute,
		Tracker:         tracker,
		EntryHandler:    entryHandler,
		Logger:          util.TestAlloyLogger(t).Slog(),
	})
	require.NoError(t, err)
	return c
}

func TestExplainPlans_NoTrackerIsNoOp(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c := newExplainPlansForTest(t, db, lokiClient, nil)

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, lokiClient.Received())
}

func TestExplainPlans_EmptyTrackerEmitsNothing(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c := newExplainPlansForTest(t, db, lokiClient, fakeTracker{hashes: map[string][]string{}})

	mockSelectQueryStoreState(mock, "READ_WRITE")

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, lokiClient.Received())
}

func TestExplainPlans_SkipsWhenQueryStoreUnavailable(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c := newExplainPlansForTest(t, db, lokiClient, trackerFor(testHash))

	mockSelectQueryStoreState(mock, "OFF")

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, lokiClient.Received())
}

func TestExplainPlans_EmitsOnFirstSighting(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c := newExplainPlansForTest(t, db, lokiClient, trackerFor(testHash))

	expectExplainPlans(t, mock, []string{testHash}, []driver.Value{queryHashBytes, simplePlanXML})

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Eventually(t, func() bool { return len(lokiClient.Received()) == 1 }, 5*time.Second, 20*time.Millisecond)
	entries := lokiClient.Received()
	require.Equal(t, model.LabelSet{"op": OP_EXPLAIN_PLAN_OUTPUT}, entries[0].Labels)
	require.Contains(t, entries[0].Line, `database="books_store"`)
	require.Contains(t, entries[0].Line, `query_hash="0011223344556677"`)

	// Engine identity is carried as a Loki label (added by addLokiLabels at
	// the component level, not exercised by this collector-level test) - the
	// metadata payload itself must not carry databaseEngine/databaseVersion
	// at all now that they're omitempty and sql_server no longer sets them.
	payload := decodedExplainPlanPayload(t, entries[0].Line)
	require.NotContains(t, payload, "databaseEngine")
	require.NotContains(t, payload, "databaseVersion")
}

func TestExplainPlans_SkipsTrackedHashWithNoPlanYet(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c := newExplainPlansForTest(t, db, lokiClient, trackerFor(testHash))

	// No rows returned: Query Store has no captured plan yet for this hash.
	expectExplainPlans(t, mock, []string{testHash})

	require.NoError(t, c.collect(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Eventually(t, func() bool { return len(lokiClient.Received()) == 1 }, 5*time.Second, 20*time.Millisecond)
	require.Contains(t, lokiClient.Received()[0].Line, `explain_plan_output=`)
}

func TestExplainPlans_EmissionGate(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	c := newExplainPlansForTest(t, db, lokiClient, trackerFor(testHash))

	base := time.Now()
	c.now = func() time.Time { return base }
	key := queryMetricsKey{database: testDatabase, queryHash: testHash}

	poll := func(planXML string) {
		t.Helper()
		expectExplainPlans(t, mock, []string{testHash}, []driver.Value{queryHashBytes, planXML})
		require.NoError(t, c.collect(context.Background()))
	}

	// First cycle: always emits.
	poll(simplePlanXML)
	require.Equal(t, base, c.lastEmittedAt[key])
	firstHash := c.lastEmittedHash[key]

	// Second cycle, moments later, same structural shape but different
	// EstimateRows/cost (optimizer jitter): must NOT emit, and the recorded
	// hash must be unchanged since the structural hash excludes those fields.
	c.now = func() time.Time { return base.Add(time.Minute) }
	poll(planXMLWithEstimate("999", "9.9"))
	require.Equal(t, base, c.lastEmittedAt[key], "jittered estimate must not trigger emission")
	require.Equal(t, firstHash, c.lastEmittedHash[key])

	// Third cycle, still within EmitInterval, but the plan is now genuinely
	// different (a different physical operator): must emit immediately,
	// regardless of EmitInterval not having elapsed.
	changedAt := base.Add(2 * time.Minute)
	c.now = func() time.Time { return changedAt }
	differentPlanXML := `<ShowPlanXML xmlns="http://schemas.microsoft.com/sqlserver/2004/07/showplan">
	  <BatchSequence><Batch><Statements><StmtSimple><QueryPlan>
	    <RelOp PhysicalOp="Clustered Index Scan" LogicalOp="Clustered Index Scan" EstimateRows="10" EstimatedTotalSubtreeCost="0.1">
	      <IndexScan><Object Database="[MyDb]" Schema="[dbo]" Table="[Orders]" Index="[PK_Orders]" /></IndexScan>
	    </RelOp>
	  </QueryPlan></StmtSimple></Statements></Batch></BatchSequence>
	</ShowPlanXML>`
	poll(differentPlanXML)
	require.Equal(t, changedAt, c.lastEmittedAt[key], "a structurally different plan must emit immediately")
	require.NotEqual(t, firstHash, c.lastEmittedHash[key])

	// Fourth cycle: unchanged plan again, but EmitInterval has now elapsed
	// since the last emission - must emit a fresh copy for the UI.
	afterInterval := changedAt.Add(database_observability.EmitInterval + time.Minute)
	c.now = func() time.Time { return afterInterval }
	poll(differentPlanXML)
	require.Equal(t, afterInterval, c.lastEmittedAt[key], "unchanged plan past EmitInterval must still refresh")

	require.NoError(t, mock.ExpectationsWereMet())
	lokiClient.Stop()
	require.Len(t, lokiClient.Received(), 3)
}

func TestExplainPlans_PrunesStaleKeys(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()

	lokiClient := loki.NewCollectingHandler()
	defer lokiClient.Stop()

	tracker := fakeTracker{hashes: map[string][]string{testDatabase: {testHash}}}
	c := newExplainPlansForTest(t, db, lokiClient, tracker)

	expectExplainPlans(t, mock, []string{testHash}, []driver.Value{queryHashBytes, simplePlanXML})
	require.NoError(t, c.collect(context.Background()))

	key := queryMetricsKey{database: testDatabase, queryHash: testHash}
	require.Contains(t, c.lastEmittedAt, key)

	// The hash falls out of query_metrics's tracked set.
	tracker.hashes[testDatabase] = nil
	mockSelectQueryStoreState(mock, "READ_WRITE")
	require.NoError(t, c.collect(context.Background()))

	require.NotContains(t, c.lastEmittedAt, key)
	require.NotContains(t, c.lastEmittedHash, key)
	require.NoError(t, mock.ExpectationsWereMet())
}

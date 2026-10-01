package collector

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

// TestLogsCollector_SlowQuery_RealSample pins the exact shape captured live
// from a local log_min_duration_statement instance (db-o11y-playground):
// "duration: <ms> ms  statement: <sql>", a single physical line (no
// continuation) for a single-statement query.
func TestLogsCollector_SlowQuery_RealSample(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "duration: 1502.201 ms  statement: SELECT pg_sleep(1.5), count(*) FROM pg_catalog.pg_class;"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "62", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "slow_query", string(got[0].Labels["op"]))

	fields := parseLogfmt(t, got[0].Line)
	require.Equal(t, "1502.201", fields["duration_ms"])
	require.Equal(t, "books_store", fields["datname"])
	require.Equal(t, "62", fields["pid"])

	expectedFP, err := fingerprint.Fingerprint("SELECT pg_sleep(1.5), count(*) FROM pg_catalog.pg_class;")
	require.NoError(t, err)
	require.Equal(t, expectedFP, fields["query_fingerprint"])
}

// TestLogsCollector_SlowQuery_CorrelationFieldsPresent pins that pid, vxid,
// xid, and session_id -- the fields a concurrent op="server_log"
// category="lock_wait" entry on the same backend also carries -- are all
// populated when the log_line_prefix run they come from is present. These
// (plus the timestamp) are the only join keys back to lock_wait: unlike
// query_fingerprint, there is no query identifier shared between the two
// log shapes.
func TestLogsCollector_SlowQuery_CorrelationFieldsPresent(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "duration: 1205.598 ms  statement: SELECT 1"
	line := fullPrefixServerLogLine(c, "199", "58/0", "12345", msg)
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, got[0].Line)

	require.Equal(t, "199", fields["pid"])
	require.Equal(t, "58/0", fields["vxid"])
	require.Equal(t, "12345", fields["xid"])
	require.NotEmpty(t, fields["session_id"])
}

// TestLogsCollector_SlowQuery_NeverEmitsRawStatementText pins that, like
// every other log-sourced op in this collector (op="error_message" never
// emits the raw SQL it fingerprints either), the raw statement text itself
// is never emitted -- only its fingerprint. A literal bound into the
// statement (an email here) must not leak into the Loki line.
func TestLogsCollector_SlowQuery_NeverEmitsRawStatementText(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "duration: 1200.000 ms  statement: SELECT * FROM customers WHERE email = 'john@example.com'"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "1", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.NotContains(t, got[0].Line, "john@example.com")
	require.NotContains(t, got[0].Line, "customers")
}

// TestLogsCollector_SlowQuery_TraceparentExtracted pins that a W3C
// traceparent embedded as a SQL comment is still extracted and emitted,
// same as op="error_message" already does: it's trace context, not query
// content, so it's exempt from the no-raw-text rule above.
func TestLogsCollector_SlowQuery_TraceparentExtracted(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := `duration: 1200.000 ms  statement: SELECT 1 /*traceparent='00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01'*/`
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "1", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, got[0].Line)
	require.Equal(t, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", fields["traceparent"])
}

// TestLogsCollector_SlowQuery_EmptyStatementFallsThrough pins that a
// statement that's empty after trimming is dropped safely rather than
// emitting a fingerprint of nothing.
func TestLogsCollector_SlowQuery_EmptyStatementFallsThrough(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "duration: 5.000 ms  statement: "
	require.NotPanics(t, func() {
		require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "1", msg)}}))
	})

	got := drainEntries(t, entryCh, 1, 300*time.Millisecond)
	require.Len(t, got, 0)
}

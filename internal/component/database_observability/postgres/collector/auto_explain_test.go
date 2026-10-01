package collector

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

// decodeExplainPlanOutput parses an op="explain_plan_output" entry's body
// (schema="..." digest="..." explain_plan_output="<base64 JSON>") back into
// its structured form, for assertions.
func decodeExplainPlanOutput(t *testing.T, line string) (fields map[string]string, output database_observability.ExplainPlanOutput) {
	t.Helper()
	fields = parseLogfmt(t, line)
	raw, err := base64.StdEncoding.DecodeString(fields["explain_plan_output"])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &output))
	return fields, output
}

// TestLogsCollector_AutoExplain_RealSample pins the exact shape captured
// live from a local auto_explain.log_format=json instance (db-o11y-playground):
// a single-tab-indented, pretty-printed JSON object with "Query Text" and
// "Plan" as sibling keys, spanning multiple physical lines.
func TestLogsCollector_AutoExplain_RealSample(t *testing.T) {
	c, entryCh := newServerLogCollector(t)
	ts := logTS(c)

	msg := "duration: 6687.304 ms  plan:\n" +
		"\t{\n" +
		"\t  \"Query Text\": \"SELECT generate_books_and_authors(target_books, target_authors, book_batch_size)\",\n" +
		"\t  \"Plan\": {\n" +
		"\t    \"Node Type\": \"Result\",\n" +
		"\t    \"Parallel Aware\": false,\n" +
		"\t    \"Async Capable\": false,\n" +
		"\t    \"Startup Cost\": 0.00,\n" +
		"\t    \"Total Cost\": 0.26,\n" +
		"\t    \"Plan Rows\": 1,\n" +
		"\t    \"Plan Width\": 4\n" +
		"\t  }\n" +
		"\t}"
	line := ts + "::app-user@books_store:[93]:52:00000:LOG:  " + msg
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "explain_plan_output", string(got[0].Labels["op"]))

	fields, output := decodeExplainPlanOutput(t, got[0].Line)
	require.Equal(t, "books_store", fields["schema"])
	require.NotEmpty(t, fields["digest"])

	require.Equal(t, "PostgreSQL", output.Metadata.DatabaseEngine)
	require.Equal(t, database_observability.ExplainProcessingResultSuccess, output.Metadata.ProcessingResult)
	require.Equal(t, fields["digest"], output.Metadata.QueryIdentifier)
	require.Equal(t, database_observability.ExplainPlanOutputOperation("Result"), output.Plan.Operation)
	require.Equal(t, int64(1), output.Plan.Details.EstimatedRows)
	require.Equal(t, 0.26, *output.Plan.Details.EstimatedCost)
}

// TestLogsCollector_AutoExplain_SameQueryMatchesLiveDigest pins that this
// log-sourced digest is a fingerprint of the query text, not a
// pg_stat_statements queryid -- so the same query explained by both this
// path and the live EXPLAIN (FORMAT JSON) collector end up with the *same*
// digest here (both fingerprint-based), by construction.
func TestLogsCollector_AutoExplain_DigestIsQueryFingerprint(t *testing.T) {
	c, entryCh := newServerLogCollector(t)
	ts := logTS(c)

	msg := "duration: 12.000 ms  plan:\n" +
		"\t{\n" +
		"\t  \"Query Text\": \"SELECT 1\",\n" +
		"\t  \"Plan\": {\n" +
		"\t    \"Node Type\": \"Result\",\n" +
		"\t    \"Startup Cost\": 0.00,\n" +
		"\t    \"Total Cost\": 0.01,\n" +
		"\t    \"Plan Rows\": 1,\n" +
		"\t    \"Plan Width\": 4\n" +
		"\t  }\n" +
		"\t}"
	line := ts + "::user@books_store:[1]:1:00000:LOG:  " + msg
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields, _ := decodeExplainPlanOutput(t, got[0].Line)

	expectedFP, err := fingerprint.Fingerprint("SELECT 1")
	require.NoError(t, err)
	require.Equal(t, expectedFP, fields["digest"])
}

// TestLogsCollector_AutoExplain_RedactsFilterLiterals pins that a plan
// node's Filter/Condition still goes through RedactSql via the shared
// PlanNode.ToExplainPlanOutputNode() conversion -- the exact same
// redaction the live EXPLAIN (FORMAT JSON) collector already relies on,
// reused unchanged for this log-sourced path.
func TestLogsCollector_AutoExplain_RedactsFilterLiterals(t *testing.T) {
	c, entryCh := newServerLogCollector(t)
	ts := logTS(c)

	msg := "duration: 1200.000 ms  plan:\n" +
		"\t{\n" +
		"\t  \"Query Text\": \"SELECT * FROM customers WHERE email = 'john@example.com'\",\n" +
		"\t  \"Plan\": {\n" +
		"\t    \"Node Type\": \"Seq Scan\",\n" +
		"\t    \"Relation Name\": \"customers\",\n" +
		"\t    \"Alias\": \"customers\",\n" +
		"\t    \"Filter\": \"(email = 'john@example.com'::text)\",\n" +
		"\t    \"Startup Cost\": 0.00,\n" +
		"\t    \"Total Cost\": 100.00,\n" +
		"\t    \"Plan Rows\": 1,\n" +
		"\t    \"Plan Width\": 50\n" +
		"\t  }\n" +
		"\t}"
	line := ts + "::user@books_store:[1]:1:00000:LOG:  " + msg
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	_, output := decodeExplainPlanOutput(t, got[0].Line)

	require.NotNil(t, output.Plan.Details.Condition)
	require.NotContains(t, *output.Plan.Details.Condition, "john@example.com", "the bound literal must be redacted")
}

// TestLogsCollector_AutoExplain_TextFormatFallsThrough pins that
// auto_explain's default text format (log_format=text, the shape this
// collector deliberately doesn't model as its own category) isn't
// mistaken for JSON and doesn't panic -- it's dropped the same way any
// other unrecognized LOG line is.
func TestLogsCollector_AutoExplain_TextFormatFallsThrough(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "duration: 1507.025 ms  plan:\n\tQuery Text: SELECT pg_sleep(1.5);\n\tResult  (cost=0.00..0.01 rows=1 width=4)"
	require.NotPanics(t, func() {
		require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "120", msg)}}))
	})

	got := drainEntries(t, entryCh, 1, 300*time.Millisecond)
	require.Len(t, got, 0, "text-format auto_explain output must never be captured")
}

// TestLogsCollector_AutoExplain_MalformedJSONFallsThrough pins that a
// truncated/invalid JSON payload (log corruption, a future auto_explain
// output change, ...) is dropped safely rather than panicking or emitting
// a broken explain_plan_output entry.
func TestLogsCollector_AutoExplain_MalformedJSONFallsThrough(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "duration: 12.000 ms  plan:\n\t{\n\t  \"Query Text\": \"SELECT 1\",\n\t  \"Plan\": {"
	require.NotPanics(t, func() {
		require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "121", msg)}}))
	})

	got := drainEntries(t, entryCh, 1, 300*time.Millisecond)
	require.Len(t, got, 0)
}

package collector

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// autoExplainGate and autoExplainRegex detect and extract an auto_explain
// LOG message's JSON payload (auto_explain.log_format = json). "ms  plan:"
// (two spaces) is auto_explain's own shape, distinct from the plain
// log_min_duration_statement "ms  statement:" line (one space, no plan --
// deliberately left unrecognized, see
// TestLogsCollector_ServerLog_UnrecognizedLogLineDroppedSilently).
const autoExplainGate = "ms  plan:"

// (?s) lets '.' cross the embedded newlines auto_explain's pretty-printed
// JSON puts in this message; each continuation line carries PostgreSQL's
// own leading tab (same convention as every other multi-line LOG message
// this collector handles), which is valid JSON whitespace and needs no
// stripping before Unmarshal.
var autoExplainRegex = regexp.MustCompile(`(?s)^duration: [\d.]+ ms  plan:\n(?P<json>.*)$`)

// autoExplainPayload is auto_explain's own JSON shape: a plain object with
// "Query Text" and "Plan" as sibling keys -- not array-wrapped the way
// interactive EXPLAIN (FORMAT JSON) is, since auto_explain logs exactly one
// plan per call. Plan reuses the exact same PlanNode type the live
// EXPLAIN (FORMAT JSON)-based collector (explain_plans.go) already
// deserializes: PostgreSQL's explain.c JSON formatter is shared by both
// the interactive EXPLAIN path and auto_explain, so the per-node fields
// are identical.
type autoExplainPayload struct {
	QueryText string   `json:"Query Text"`
	Plan      PlanNode `json:"Plan"`
}

// tryEmitAutoExplainPlan checks whether message is an auto_explain
// JSON-format LOG line and, if so, converts it into the same
// op="explain_plan_output" shape the live EXPLAIN (FORMAT JSON)-based
// collector produces (reusing PlanNode.ToExplainPlanOutputNode() -- same
// redaction of filter/condition expressions, same cost math, same
// recursive child handling), emits it, and returns true.
//
// Returns false for anything else, including auto_explain's own default
// text format (when auto_explain.log_format hasn't been switched to json
// on a given instance yet): the caller falls through to the server-log
// pattern registry in that case, which doesn't model this shape either, so
// it's dropped the same way an unrecognized LOG line always is.
//
// QueryIdentifier is query_fingerprint -- the same SQL fingerprint used
// everywhere else in this collector (op="error_message", deadlock's
// query_fingerprint_blocker, ...) -- not a pg_stat_statements queryid
// (unavailable from a log line). It's comparable to this collector's own
// other query_fingerprint fields for the same query, but not to the live
// EXPLAIN (FORMAT JSON) collector's own op="explain_plan_output" entries,
// which still key on "digest" (that queryid): a known, accepted difference
// between the two collectors' identifier namespaces. sourced_from=logs
// marks which of the two produced a given entry.
func (l *Logs) tryEmitAutoExplainPlan(message string, datname string, ts time.Time) bool {
	if !strings.Contains(message, autoExplainGate) {
		return false
	}
	m := autoExplainRegex.FindStringSubmatch(message)
	if m == nil {
		return false
	}

	var payload autoExplainPayload
	if err := json.Unmarshal([]byte(m[1]), &payload); err != nil {
		return false
	}

	planNode, err := payload.Plan.ToExplainPlanOutputNode()
	if err != nil {
		return false
	}

	queryText := strings.TrimSpace(payload.QueryText)
	queryFingerprint, err := fingerprint.Fingerprint(queryText)
	if err != nil {
		return false
	}

	if ts.IsZero() {
		ts = time.Now()
	}

	output := database_observability.ExplainPlanOutput{
		Metadata: database_observability.ExplainPlanMetadataInfo{
			DatabaseEngine:   "PostgreSQL",
			QueryIdentifier:  queryFingerprint,
			GeneratedAt:      ts.Format(time.RFC3339),
			ProcessingResult: database_observability.ExplainProcessingResultSuccess,
		},
		Plan: planNode,
	}

	explainPlanOutputJSON, err := json.Marshal(output)
	if err != nil {
		return false
	}

	// query_fingerprint (unquoted, like every other op's query_fingerprint
	// field -- never raw SQL) and sourced_from=logs distinguish this
	// log-parsed plan from the live EXPLAIN (FORMAT JSON) collector's own
	// op="explain_plan_output" entries, which still key on "digest" (a
	// pg_stat_statements queryid, not a fingerprint -- see
	// tryEmitAutoExplainPlan's doc comment on the two digest namespaces).
	logMessage := fmt.Sprintf(
		`schema=%q query_fingerprint=%s sourced_from=logs explain_plan_output="%s"`,
		datname,
		queryFingerprint,
		base64.StdEncoding.EncodeToString(explainPlanOutputJSON),
	)

	select {
	case l.entryHandler.Chan() <- database_observability.BuildLokiEntryWithTimestamp(
		logging.LevelInfo,
		database_observability.OP_EXPLAIN_PLAN_OUTPUT,
		logMessage,
		ts.UnixNano(),
	):
	case <-l.ctx.Done():
	}
	return true
}

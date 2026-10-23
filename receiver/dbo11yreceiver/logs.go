package dbo11yreceiver

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// entriesToLogs converts labeled db-o11y entries to plog, one ResourceLogs
// per job/instance/server_id combination. Every Loki label becomes a log
// record attribute (logRecordLabel decides about server_id), structured
// metadata does too, and the line is the body.
func entriesToLogs(entries []loki.Entry, observed time.Time) plog.Logs {
	ld := plog.NewLogs()
	scopes := make(map[[3]string]plog.ScopeLogs)

	for _, e := range entries {
		labels := make(map[string]string, len(e.Labels))
		for k, v := range e.Labels {
			labels[string(k)] = string(v)
		}
		key := [3]string{labels[labelJob], labels[labelInstance], labels[labelServerID]}
		sl, ok := scopes[key]
		if !ok {
			rl := ld.ResourceLogs().AppendEmpty()
			putResource(rl.Resource().Attributes(), labels)
			sl = rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName(scopeName)
			scopes[key] = sl
		}

		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(pcommon.NewTimestampFromTime(e.Timestamp))
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(observed))
		lr.Body().SetStr(e.Line)
		attrs := lr.Attributes()
		for name, value := range labels {
			if logRecordLabel(name) {
				attrs.PutStr(name, value)
			}
		}
		for _, md := range e.StructuredMetadata {
			attrs.PutStr(md.Name, md.Value)
		}
	}
	return ld
}

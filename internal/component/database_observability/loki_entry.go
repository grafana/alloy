package database_observability

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/loki/pkg/push"
)

func BuildLokiEntryWithTimestamp(level logging.Level, op, line string, timestamp int64) loki.Entry {
	return loki.NewEntry(
		model.LabelSet{"op": model.LabelValue(op)},
		push.Entry{
			Timestamp: time.Unix(0, timestamp),
			Line:      fmt.Sprintf(`level="%s" %s`, level, line),
		},
	)
}

func BuildLokiEntry(level logging.Level, op, line string) loki.Entry {
	return BuildLokiEntryWithTimestamp(level, op, line, time.Now().UnixNano())
}

func BuildLokiEntryWithStructuredMetadata(level logging.Level, op, line string, metadata push.LabelsAdapter) loki.Entry {
	e := BuildLokiEntry(level, op, line)
	e.Entry.StructuredMetadata = metadata
	return e
}

func BuildLokiEntryWithTimestampAndStructuredMetadata(level logging.Level, op, line string, timestamp int64, metadata push.LabelsAdapter) loki.Entry {
	e := BuildLokiEntryWithTimestamp(level, op, line, timestamp)
	e.Entry.StructuredMetadata = metadata
	return e
}

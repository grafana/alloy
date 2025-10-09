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
	return loki.Entry{
		Labels: model.LabelSet{
			"op": model.LabelValue(op),
		},
		Entry: push.Entry{
			Timestamp: time.Unix(0, timestamp),
			Line:      fmt.Sprintf(`level="%s" %s`, level, line),
		},
	}
}

func BuildLokiEntry(level logging.Level, op, line string) loki.Entry {
	return BuildLokiEntryWithTimestamp(level, op, line, time.Now().UnixNano())
}

package database_observability

import (
	"fmt"
	"time"

	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func BuildLokiEntryWithTimestamp(level logging.Level, op, instanceKey, line string, timestamp int64) loki.Entry {
	return loki.Entry{
		Labels: model.LabelSet{
			"job":      JobName,
			"op":       model.LabelValue(op),
			"instance": model.LabelValue(instanceKey),
		},
		Entry: logproto.Entry{
			Timestamp: time.Unix(0, timestamp),
			Line:      fmt.Sprintf(`level="%s" %s`, level, line),
		},
	}
}

func BuildLokiEntry(level logging.Level, op, instanceKey, line string) loki.Entry {
	return BuildLokiEntryWithTimestamp(level, op, instanceKey, line, time.Now().UnixNano())
}

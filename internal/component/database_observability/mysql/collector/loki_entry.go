package collector

import (
	"time"

	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability"
)

func buildLokiEntry(op string, instanceKey string, line string) loki.Entry {
	return loki.Entry{
		Labels: model.LabelSet{
			"job":      database_observability.JobName,
			"op":       model.LabelValue(op),
			"instance": model.LabelValue(instanceKey),
		},
		Entry: logproto.Entry{
			Timestamp: time.Unix(0, time.Now().UnixNano()),
			Line:      line,
		},
	}
}

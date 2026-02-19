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

// Field is a name-value pair routed to an indexed label, structured metadata,
// or the log line depending on feature flags. Empty values are omitted.
type Field struct {
	Name  string
	Value string
}

// BuildV2LokiEntry routes indexableFields to indexed labels (or prepends them
// to the log line) and structuredMetadataFields to structured metadata (or appends them),
// depending on the feature flags.
func BuildV2LokiEntry(
	level logging.Level,
	op, baseLogLine string,
	indexableFields []Field,
	structuredMetadataFields []Field,
	enableIndexedLabels, enableStructuredMetadata bool,
	timestamp int64,
) loki.Entry {
	logLine := baseLogLine
	labels := model.LabelSet{"op": model.LabelValue(op)}
	var smLabels push.LabelsAdapter

	for i := len(indexableFields) - 1; i >= 0; i-- {
		f := indexableFields[i]
		if f.Value == "" {
			continue
		}
		if enableIndexedLabels {
			labels[model.LabelName(f.Name)] = model.LabelValue(f.Value)
		} else {
			logLine = fmt.Sprintf(`%s="%s" `, f.Name, f.Value) + logLine
		}
	}

	for _, f := range structuredMetadataFields {
		if f.Value == "" {
			continue
		}
		if enableStructuredMetadata {
			smLabels = append(smLabels, push.LabelAdapter{Name: f.Name, Value: f.Value})
		} else {
			logLine += fmt.Sprintf(` %s="%s"`, f.Name, f.Value)
		}
	}

	return loki.Entry{
		Labels: labels,
		Entry: push.Entry{
			Timestamp:          time.Unix(0, timestamp),
			Line:               fmt.Sprintf(`level="%s" %s`, level, logLine),
			StructuredMetadata: smLabels,
		},
	}
}

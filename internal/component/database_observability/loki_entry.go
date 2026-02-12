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

// BuildLokiEntryWithStructuredMetadata creates a Loki entry with structured metadata.
// structuredMetadata: High-cardinality metadata not indexed but still queryable (e.g., "queryid", "digest", "wait_event_type")
// Empty string values are omitted from structured metadata.
func BuildLokiEntryWithStructuredMetadata(level logging.Level, op, line string, structuredMetadata map[string]string, timestamp int64) loki.Entry {
	labels := model.LabelSet{
		"op": model.LabelValue(op),
	}

	// Convert structured metadata to LabelsAdapter
	var structuredMetadataLabels push.LabelsAdapter
	for key, value := range structuredMetadata {
		if value != "" {
			structuredMetadataLabels = append(structuredMetadataLabels, push.LabelAdapter{
				Name:  key,
				Value: value,
			})
		}
	}

	return loki.Entry{
		Labels: labels,
		Entry: push.Entry{
			Timestamp:          time.Unix(0, timestamp),
			Line:               fmt.Sprintf(`level="%s" %s`, level, line),
			StructuredMetadata: structuredMetadataLabels,
		},
	}
}

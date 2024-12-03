package prometheus

import (
	"context"

	"github.com/prometheus/prometheus/model/labels"
)

type BulkAppendable interface {
	// Appender returns a new appender for the storage. The implementation
	// can choose whether or not to use the context, for deadlines or to check
	// for errors.
	Appender(ctx context.Context) BulkAppender
}

type PromMetric struct {
	Value  float64
	TS     int64
	Labels labels.Labels
}

type BulkAppender interface {
	Append(metadata map[string]string, metrics []PromMetric) error
}

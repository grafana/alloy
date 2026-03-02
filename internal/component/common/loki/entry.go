package loki

import (
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

// Entry is a log entry with labels.
type Entry struct {
	Labels model.LabelSet
	push.Entry
}

// Clone returns a copy of the entry so that it can be safely fanned out.
func (e *Entry) Clone() Entry {
	return Entry{
		Labels: e.Labels.Clone(),
		Entry:  e.Entry,
	}
}

// Returns the size of the entry in bytes.
func (e *Entry) Size() int {
	// FIXME(kalleep): This is not correct but computing
	// the actual size an entry would take when serialized to proto
	// is quite expensive.
	size := len(e.Line)
	for _, label := range e.StructuredMetadata {
		size += label.Size()
	}
	return size
}

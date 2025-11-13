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

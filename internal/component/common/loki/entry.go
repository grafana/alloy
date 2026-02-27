package loki

import (
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

func NewEntry(lset model.LabelSet, e push.Entry) Entry {
	return Entry{
		Labels:  lset,
		Entry:   e,
		created: time.Now(),
	}
}

func NewEntryWithCreated(lset model.LabelSet, created time.Time, e push.Entry) Entry {
	return Entry{
		Labels:  lset,
		Entry:   e,
		created: created,
	}
}

// Entry is a push.Entry with labels.
// It should be created using either NewEntry or NewEntryWithTimestamp.
type Entry struct {
	Labels model.LabelSet
	push.Entry

	// FIXME check if we should use unix nano instead..
	created time.Time
}

// Clone returns a copy of the entry so that it can be safely fanned out.
func (e *Entry) Clone() Entry {
	return Entry{
		Labels:  e.Labels.Clone(),
		Entry:   e.Entry,
		created: e.created,
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

func (e *Entry) Created() time.Time {
	return e.created
}

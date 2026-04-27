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
		created: time.Now().UnixMicro(),
	}
}

func NewEntryWithCreated(lset model.LabelSet, created time.Time, e push.Entry) Entry {
	return Entry{
		Labels:  lset,
		Entry:   e,
		created: created.UnixMicro(),
	}
}

func NewEntryWithCreatedUnixMicro(lset model.LabelSet, created int64, e push.Entry) Entry {
	return Entry{
		Labels:  lset,
		Entry:   e,
		created: created,
	}
}

// Entry is a push.Entry with labels.
// It should be created using either NewEntry or NewEntryWithCreated.
type Entry struct {
	Labels model.LabelSet
	push.Entry

	// Created is a unix timestamp in micro seconds.
	// FIXME(kalleep): Currently we store created for each entry.
	// When moving to batching we can store it per batch.
	created int64
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

func (e *Entry) Created() int64 {
	return e.created
}

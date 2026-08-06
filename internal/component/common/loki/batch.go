package loki

import (
	"slices"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

type Batch struct {
	// created is a unix timestamp in micro seconds.
	created  int64
	entryLen int
	streams  []Stream
}

// NewBatch creates an empty Batch.
func NewBatch() Batch {
	return Batch{}
}

// NewBatchWithCreatedUnixMicro creates an empty Batch with created.
func NewBatchWithCreatedUnixMicro(created int64) Batch {
	return Batch{created: created}
}

// Add adds a stream to the batch.
// Ownership of the stream data is transferred to the batch and it must not be
// mutated or retained after calling Add.
func (b *Batch) Add(stream Stream) {
	if b.created == 0 {
		b.created = time.Now().UnixMicro()
	}
	b.add(stream.Labels, stream.Entries...)
	b.entryLen += len(stream.Entries)
}

func (b *Batch) add(labels model.LabelSet, entries ...push.Entry) {
	i := slices.IndexFunc(b.streams, func(s Stream) bool {
		return s.Labels.Equal(labels)
	})

	if i >= 0 {
		b.streams[i].Entries = append(b.streams[i].Entries, entries...)
		return
	}

	b.streams = append(b.streams, NewStream(labels, entries...))
}

// FilterMap calls fn for each entry in the batch. If fn returns true the
// entry is kept, if fn returns false the entry is dropped. Kept entries are
// written back, and entries whose labels change are moved to a different stream.
func (b *Batch) FilterMap(fn func(entry *Entry) (keep bool)) {
	var (
		newLen int
		moves  []Entry
	)

	// Process each entry and compact each stream in place.
	// The callback mutates a temporary Entry view. Kept entries are written back,
	// dropped entries are skipped, and moved entries are deferred
	// so we do not mutate the stream set while iterating it.
	for i := range b.streams {
		var (
			// dst is where the next kept entry is written. It only moves forward
			// when an entry is kept, so it never gets ahead of the entry being read
			// and we only write to slots the loop has already read.
			dst    = 0
			stream = &b.streams[i]
		)

		for _, e := range stream.Entries {
			// FIXME(kalleep): This clone protects stream.Labels from in-place mutation
			// through Entry.Labels, which is a map. Most FilterMap callbacks do not change
			// labels, so once the new batched pipeline owns this path, consider replacing
			// direct label access with copy-on-write label mutation methods.
			entry := NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), b.created, e)
			if !fn(&entry) {
				continue
			}

			if !stream.Labels.Equal(entry.Labels) {
				moves = append(moves, entry)
				newLen++

				continue
			}

			stream.Entries[dst] = entry.Entry
			dst++
			newLen++
		}

		// Entries from dst onwards were either dropped or saved in moves, so it is
		// safe to cut them here and let add reuse that spare capacity.
		stream.Entries = stream.Entries[:dst]
	}

	// Reinsert entries whose labels changed into their destination streams.
	for _, moved := range moves {
		b.add(moved.Labels, moved.Entry)
	}
	b.entryLen = newLen

	// Remove any empty streams.
	streamDst := 0
	for i := range b.streams {
		if len(b.streams[i].Entries) == 0 {
			continue
		}
		b.streams[streamDst] = b.streams[i]
		streamDst++
	}
	b.streams = b.streams[:streamDst]
}

// FilterMapStreams calls fn once for each stream in the batch, passing a mutable
// view of that stream. If fn returns true the stream is kept, if fn returns
// false the stream and all of its entries are dropped.
func (b *Batch) FilterMapStreams(fn func(stream *Stream) (keep bool)) {
	var (
		newLen int
		// dst is where the next kept stream is written. It only moves forward when
		// a stream is kept, so it never gets ahead of i and we only write to slots
		// the loop has already read.
		dst   int
		moves []Stream
	)

	// Process each stream and compact the stream slice in place. Dropped streams
	// are skipped, and streams whose labels changed are deferred into moves so we
	// do not mutate the stream set while iterating it. They are reinserted below
	// and may merge into an existing stream.
	for i := range b.streams {
		stream := Stream{
			// FIXME(kalleep): Once the new batched pipeline owns this path, consider
			// copy-on-write label mutation methods so unchanged streams skip the copy.
			Labels:  b.streams[i].Labels.Clone(),
			Entries: b.streams[i].Entries,
		}

		keep := fn(&stream)
		if !keep {
			continue
		}

		newLen += len(stream.Entries)

		if !b.streams[i].Labels.Equal(stream.Labels) {
			moves = append(moves, stream)
			continue
		}

		b.streams[dst] = stream
		dst++
	}

	// Streams from dst onwards were either dropped or saved in moves, so it is
	// safe to cut them here and let add reuse those slots.
	b.streams = b.streams[:dst]

	// Reinsert streams whose labels changed into their destination streams.
	for _, moved := range moves {
		b.add(moved.Labels, moved.Entries...)
	}

	b.entryLen = newLen
}

// StreamLen returns the number of streams in the batch.
func (b *Batch) StreamLen() int {
	return len(b.streams)
}

// EntryLen returns the number of entries in the batch.
func (b *Batch) EntryLen() int {
	return b.entryLen
}

func (b *Batch) Created() int64 {
	return b.created
}

// Clone returns a clone of the batch.
func (b *Batch) Clone() Batch {
	clone := Batch{
		created:  b.created,
		streams:  make([]Stream, 0, len(b.streams)),
		entryLen: b.entryLen,
	}

	for _, stream := range b.streams {
		clonedStream := stream.Clone()
		clone.streams = append(clone.streams, clonedStream)
	}

	return clone
}

// ConsumeStreams calls fn for each stream in the batch and then resets the batch.
// The callback receives ownership of the stream.
// If callback returns errors interation will stop
func (b *Batch) ConsumeStreams(fn func(stream Stream, created int64) error) error {
	defer b.Reset()
	for _, s := range b.streams {
		if err := fn(s, b.created); err != nil {
			return err
		}
	}
	return nil
}

// Reset clears the batch so it can be reused.
func (b *Batch) Reset() {
	b.created = 0
	b.entryLen = 0
	b.streams = b.streams[:0]
}

// NewStream creates a Stream that owns the provided labels and entries.
func NewStream(labels model.LabelSet, entries ...push.Entry) Stream {
	return Stream{
		Labels:  labels,
		Entries: entries,
	}
}

// Stream is a group of entries sharing the same labels.
type Stream struct {
	Labels  model.LabelSet
	Entries []push.Entry
}

// Clone returns a clone of the stream.
func (s Stream) Clone() Stream {
	cloned := Stream{
		Labels:  s.Labels.Clone(),
		Entries: make([]push.Entry, 0, len(s.Entries)),
	}
	for _, entry := range s.Entries {
		e := push.Entry{
			Timestamp:          entry.Timestamp,
			Line:               entry.Line,
			StructuredMetadata: slices.Clone(entry.StructuredMetadata),
		}

		cloned.Entries = append(cloned.Entries, e)
	}
	return cloned
}

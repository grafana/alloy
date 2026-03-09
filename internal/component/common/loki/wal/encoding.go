package wal

import (
	"errors"
	"fmt"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// RecordType represents the type of the WAL/Checkpoint record.
type RecordType byte

const (
	_ = iota // ignore first value so the zero value doesn't look like a record type.
	// WALRecordSeries is the type for the WAL record for series.
	WALRecordSeries RecordType = iota
	// WALRecordEntriesV1 is the type for the WAL record for samples.
	WALRecordEntriesV1
	// CheckpointRecord is the type for the Checkpoint record based on protos.
	CheckpointRecord
	// WALRecordEntriesV2 is the type for the WAL record for samples with an
	// additional counter value for use in replaying without the ordering constraint.
	WALRecordEntriesV2
	// WALRecordEntriesV3 is the type for the WAL record for samples with structured metadata.
	WALRecordEntriesV3
	// WALRecordEntriesV4 are entries with included created time.
	WALRecordEntriesV4
)

// The current type of Entries that WAL writes.
const CurrentEntriesRec = WALRecordEntriesV4

type RefEntries struct {
	// Counter is unused.
	Counter int64
	// Created is a unix timestamp in micro seconds that represent
	// the time entries was ingested.
	Created int64
	// Ref identifies the series these entries belong to.
	Ref chunks.HeadSeriesRef
	// Entries are log entries belonging to the same series.
	Entries []push.Entry
}

// EntryAt returns the entry at i with the provided label set.
// i must be a valid index into Entries.
func (r RefEntries) EntryAt(lset model.LabelSet, i int) loki.Entry {
	return loki.NewEntryWithCreatedUnixMicro(lset, r.Created, r.Entries[i])
}

// Record is a struct combining the series and samples record.
type Record struct {
	// UserID is unused.
	UserID     string
	Series     []record.RefSeries
	RefEntries []RefEntries
}

func (r *Record) IsEmpty() bool {
	return len(r.Series) == 0 && len(r.RefEntries) == 0
}

func (r *Record) Reset() {
	r.UserID = ""
	if len(r.Series) > 0 {
		r.Series = r.Series[:0]
	}

	r.RefEntries = r.RefEntries[:0]
}

func (r *Record) EncodeSeries(b []byte) []byte {
	buf := encWith(b)
	buf.PutByte(byte(WALRecordSeries))
	buf.PutUvarintStr(r.UserID)

	var enc record.Encoder
	// The 'encoded' already has the type header and userID here, hence re-using
	// the remaining part of the slice (i.e. encoded[len(encoded):])) to encode the series.
	encoded := buf.Get()
	encoded = append(encoded, enc.Series(r.Series, encoded[len(encoded):])...)

	return encoded
}

func (r *Record) EncodeEntries(version RecordType, b []byte) []byte {
	buf := encWith(b)
	buf.PutByte(byte(version))
	buf.PutUvarintStr(r.UserID)

	// Placeholder for the first timestamp of any sample encountered.
	// All others in this record will store their timestamps as diffs relative to this
	// as a space optimization.
	var first int64

outer:
	for _, ref := range r.RefEntries {
		for _, entry := range ref.Entries {
			first = entry.Timestamp.UnixNano()
			buf.PutBE64int64(first)
			break outer
		}
	}

	for _, ref := range r.RefEntries {
		// ignore refs with 0 entries
		if len(ref.Entries) < 1 {
			continue
		}

		// Write fingerprint.
		buf.PutBE64(uint64(ref.Ref))

		if version >= WALRecordEntriesV2 {
			// Write highest counter value.
			buf.PutBE64int64(ref.Counter)
		}

		if version >= WALRecordEntriesV4 {
			// V4 has one created timestamp per RefEntries.
			buf.PutBE64int64(ref.Created)
		}

		// Write number of entries.
		buf.PutUvarint(len(ref.Entries))

		for _, s := range ref.Entries {
			buf.PutVarint64(s.Timestamp.UnixNano() - first)
			buf.PutUvarint(len(s.Line))
			buf.PutString(s.Line)

			if version >= WALRecordEntriesV3 {
				// Write structured metadata.
				buf.PutUvarint(len(s.StructuredMetadata))
				for _, l := range s.StructuredMetadata {
					buf.PutUvarint(len(l.Name))
					buf.PutString(l.Name)
					buf.PutUvarint(len(l.Value))
					buf.PutString(l.Value)
				}
			}
		}
	}
	return buf.Get()
}

func DecodeEntries(b []byte, version RecordType, rec *Record) error {
	if len(b) == 0 {
		return nil
	}

	dec := decWith(b)

	baseTime := dec.Be64int64()

	for len(dec.B) > 0 && dec.Err() == nil {
		refEntries := RefEntries{
			Ref: chunks.HeadSeriesRef(dec.Be64()),
		}

		if version >= WALRecordEntriesV2 {
			refEntries.Counter = dec.Be64int64()
		}

		if version >= WALRecordEntriesV4 {
			refEntries.Created = dec.Be64int64()
		}

		n := dec.Uvarint()
		refEntries.Entries = make([]push.Entry, 0, n)
		rem := n

		for ; dec.Err() == nil && rem > 0; rem-- {
			timeOffset := dec.Varint64()
			lineLength := dec.Uvarint()
			line := dec.Bytes(lineLength)

			var structuredMetadata []push.LabelAdapter
			if version >= WALRecordEntriesV3 {
				nStructuredMetadata := dec.Uvarint()
				if nStructuredMetadata > 0 {
					structuredMetadata = make([]push.LabelAdapter, 0, nStructuredMetadata)
					for i := 0; dec.Err() == nil && i < nStructuredMetadata; i++ {
						nameLength := dec.Uvarint()
						name := dec.Bytes(nameLength)
						valueLength := dec.Uvarint()
						value := dec.Bytes(valueLength)
						structuredMetadata = append(structuredMetadata, push.LabelAdapter{
							Name:  string(name),
							Value: string(value),
						})
					}
				}
			}

			refEntries.Entries = append(refEntries.Entries, push.Entry{
				Timestamp:          time.Unix(0, baseTime+timeOffset),
				Line:               string(line),
				StructuredMetadata: structuredMetadata,
			})
		}

		if dec.Err() != nil {
			return fmt.Errorf("entry decode error after %d RefEntries: %w", n-rem, dec.Err())
		}

		rec.RefEntries = append(rec.RefEntries, refEntries)
	}

	if dec.Err() != nil {
		return fmt.Errorf("refEntry decode error: %w", dec.Err())
	}

	if len(dec.B) > 0 {
		return fmt.Errorf("unexpected %d bytes left in entry", len(dec.B))
	}

	return nil
}

func DecodeRecord(b []byte, walRec *Record) (err error) {
	var (
		userID  string
		dec     record.Decoder
		rSeries []record.RefSeries

		decbuf = decWith(b)
		t      = RecordType(decbuf.Byte())
	)

	switch t {
	case WALRecordSeries:
		userID = decbuf.UvarintStr()
		rSeries, err = dec.Series(decbuf.B, walRec.Series)
	case WALRecordEntriesV1, WALRecordEntriesV2, WALRecordEntriesV3, WALRecordEntriesV4:
		userID = decbuf.UvarintStr()
		err = DecodeEntries(decbuf.B, t, walRec)
	default:
		return errors.New("unknown record type")
	}

	// We reach here only if its a record with type header.
	if decbuf.Err() != nil {
		return decbuf.Err()
	}

	if err != nil {
		return err
	}

	walRec.UserID = userID
	walRec.Series = rSeries
	return nil
}

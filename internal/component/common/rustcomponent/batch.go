package rustcomponent

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// encodeBatch encodes batch in the wire format of alloy_core::logs.
func encodeBatch(batch loki.Batch) []byte {
	streams := batch.Streams()

	var buf []byte
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(streams)))
	for _, s := range streams {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s.Labels)))
		for name, value := range s.Labels {
			buf = appendString(buf, string(name))
			buf = appendString(buf, string(value))
		}

		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s.Entries)))
		for _, e := range s.Entries {
			buf = binary.LittleEndian.AppendUint64(buf, uint64(e.Timestamp.UnixNano()))
			buf = appendString(buf, e.Line)
			buf = appendLabels(buf, e.StructuredMetadata)
		}
	}
	return buf
}

func appendLabels(buf []byte, labels push.LabelsAdapter) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(labels)))
	for _, l := range labels {
		buf = appendString(buf, l.Name)
		buf = appendString(buf, l.Value)
	}
	return buf
}

func appendString(buf []byte, s string) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
	return append(buf, s...)
}

// decodeBatch decodes a batch in the wire format of alloy_core::logs, copying
// everything out of data. extra labels are added to every stream; a stream's
// own labels win on conflict.
func decodeBatch(data []byte, extra model.LabelSet) (loki.Batch, error) {
	d := decoder{data: data}
	created := time.Now().UnixMicro()

	var batch loki.Batch
	for range d.u32() {
		labels := extra.Clone()
		for range d.u32() {
			name, value := d.str(), d.str()
			labels[model.LabelName(name)] = model.LabelValue(value)
		}

		entries := make([]push.Entry, d.u32())
		for i := range entries {
			entries[i] = push.Entry{
				Timestamp: time.Unix(0, int64(d.u64())),
				Line:      d.str(),
			}
			if n := d.u32(); n > 0 {
				sm := make(push.LabelsAdapter, n)
				for j := range sm {
					sm[j] = push.LabelAdapter{Name: d.str(), Value: d.str()}
				}
				entries[i].StructuredMetadata = sm
			}
		}
		batch.Add(loki.NewStreamWithCreatedUnixMicro(labels, created, entries...))
	}

	if d.err != nil {
		return loki.Batch{}, d.err
	}
	if len(d.data) > 0 {
		return loki.Batch{}, errors.New("trailing bytes after batch")
	}
	return batch, nil
}

type decoder struct {
	data []byte
	err  error
}

func (d *decoder) take(n int) []byte {
	if d.err != nil || len(d.data) < n {
		d.err = errors.New("batch is truncated")
		// Enough zeroes for u32/u64, without allocating a bogus length.
		var zero [8]byte
		return zero[:min(n, len(zero))]
	}
	b := d.data[:n]
	d.data = d.data[n:]
	return b
}

func (d *decoder) u32() uint32 { return binary.LittleEndian.Uint32(d.take(4)) }
func (d *decoder) u64() uint64 { return binary.LittleEndian.Uint64(d.take(8)) }
func (d *decoder) str() string { return string(d.take(int(d.u32()))) }

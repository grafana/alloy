package wal

// KEEP IN SYNC WITH:
// https://github.com/grafana/loki/blob/main/pkg/util/encoding/encoding.go
// Local modifications should be minimized.

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/prometheus/prometheus/tsdb/encoding"
)

func encWith(b []byte) (res encbuf) {
	res.B = b
	return res
}

// encbuf extends encoding.Encbuf with support for multi byte encoding
type encbuf struct {
	encoding.Encbuf
}

func (e *encbuf) PutString(s string) { e.B = append(e.B, s...) }

func (e *encbuf) Skip(i int) {
	e.B = e.B[:len(e.B)+i]
}

func decWith(b []byte) (res decbuf) {
	res.B = b
	return res
}

// decbuf extends encoding.Decbuf with support for multi byte decoding
type decbuf struct {
	encoding.Decbuf
}

func (d *decbuf) Bytes(n int) []byte {
	if d.E != nil {
		return nil
	}
	if len(d.B) < n {
		d.E = encoding.ErrInvalidSize
		return nil
	}
	x := d.B[:n]
	d.B = d.B[n:]
	return x
}

func (d *decbuf) CheckCrc(castagnoliTable *crc32.Table) error {
	if d.E != nil {
		return d.E
	}
	if len(d.B) < 4 {
		d.E = encoding.ErrInvalidSize
		return d.E
	}

	offset := len(d.B) - 4
	expCRC := binary.BigEndian.Uint32(d.B[offset:])
	d.B = d.B[:offset]

	if d.Crc32(castagnoliTable) != expCRC {
		d.E = encoding.ErrInvalidChecksum
		return d.E
	}
	return nil
}

package wal

import (
	"sync"
)

var bytesPool = &sync.Pool{
	New: func() any {
		buf := new([]byte)            // Attempt to force allocation on heap.
		*buf = make([]byte, 0, 1<<10) // 1kb
		return buf
	},
}

func getBytes() *[]byte {
	return bytesPool.Get().(*[]byte)
}

func putBytes(b *[]byte) {
	*b = (*b)[:0]
	bytesPool.Put(b)
}

var recordPool = &sync.Pool{
	New: func() any {
		return &Record{}
	},
}

func getRecord() *Record {
	return recordPool.Get().(*Record)
}

func putRecord(r *Record) {
	r.Reset()
	recordPool.Put(r)
}

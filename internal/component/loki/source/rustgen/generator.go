package rustgen

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../../rust/alloy-ffi/include
#cgo LDFLAGS: ${SRCDIR}/../../../../../rust/target/release/liballoy_ffi.a -lgcc_s -lutil -lrt -lpthread -lm -ldl -lc
#include <stdlib.h>
#include "alloy.h"
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"time"
	"unsafe"

	"github.com/grafana/loki/pkg/push"
)

// generator wraps the Rust loki.source.rustgen component. run and next must
// run on separate goroutines, and next must be called until it returns false,
// otherwise run can't return. close may be called at any time; free only once
// run and next have returned.
type generator struct {
	c *C.alloy_component
}

func newGenerator(linesPerBatch int, interval time.Duration) (*generator, error) {
	name := C.CString("loki.source.rustgen")
	defer C.free(unsafe.Pointer(name))

	args := C.rustgen_args{
		lines_per_batch: C.uint32_t(linesPerBatch),
		interval_ms:     C.uint64_t(interval.Milliseconds()),
	}

	var cerr C.alloy_buf
	c := C.alloy_component_new(name, unsafe.Pointer(&args), C.size_t(C.sizeof_rustgen_args), &cerr)
	if c == nil {
		msg := C.GoStringN((*C.char)(unsafe.Pointer(cerr.ptr)), C.int(cerr.len))
		C.alloy_buf_free(&cerr)
		return nil, errors.New(msg)
	}
	return &generator{c: c}, nil
}

// run blocks until the generator is closed.
func (g *generator) run() {
	C.alloy_source_run(g.c)
}

// next blocks until the next batch and calls fn for each entry. It returns
// false once the generator has stopped and every batch has been received.
func (g *generator) next(fn func(push.Entry)) bool {
	var buf C.alloy_buf
	if C.alloy_source_recv(g.c, &buf) == C.ALLOY_CLOSED {
		return false
	}
	defer C.alloy_buf_free(&buf)

	data := unsafe.Slice((*byte)(unsafe.Pointer(buf.ptr)), int(buf.len))
	for len(data) > 0 {
		ts := int64(binary.LittleEndian.Uint64(data[0:8]))
		n := binary.LittleEndian.Uint32(data[8:12])
		line := string(data[12 : 12+n])
		data = data[12+n:]
		fn(push.Entry{Timestamp: time.Unix(0, ts), Line: line})
	}
	return true
}

func (g *generator) close() {
	C.alloy_component_close(g.c)
}

func (g *generator) free() {
	C.alloy_component_free(g.c)
}

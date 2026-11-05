package rustcomponent

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../rust/alloy-ffi/include
#cgo LDFLAGS: ${SRCDIR}/../../../../rust/target/release/liballoy_ffi.a -lgcc_s -lutil -lrt -lpthread -lm -ldl -lc
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

// Source is a Rust source component. Run and Recv must be called on separate
// goroutines, and Recv must be called until it returns false, otherwise Run
// can't return. Close may be called at any time, Free only once Run and Recv
// have returned.
type Source struct {
	c *C.alloy_component
}

// NewSource creates the Rust source registered as name with args encoded by
// EncodeArgs.
func NewSource(name string, args []byte) (*Source, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var cerr C.alloy_buf
	c := C.alloy_component_new(cname, unsafe.Pointer(&args[0]), C.size_t(len(args)), &cerr)
	if c == nil {
		msg := C.GoStringN((*C.char)(unsafe.Pointer(cerr.ptr)), C.int(cerr.len))
		C.alloy_buf_free(&cerr)
		return nil, errors.New(msg)
	}
	return &Source{c: c}, nil
}

// Run blocks until the source is closed.
func (s *Source) Run() {
	C.alloy_source_run(s.c)
}

// Recv blocks until the next batch and calls fn for each entry. It returns
// false once the source has stopped and every batch has been received.
func (s *Source) Recv(fn func(push.Entry)) bool {
	var buf C.alloy_buf
	if C.alloy_source_recv(s.c, &buf) == C.ALLOY_CLOSED {
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

func (s *Source) Close() {
	C.alloy_component_close(s.c)
}

func (s *Source) Free() {
	C.alloy_component_free(s.c)
}

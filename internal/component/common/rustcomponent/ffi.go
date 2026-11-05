package rustcomponent

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../rust/alloy-ffi/include
#cgo LDFLAGS: ${SRCDIR}/../../../../rust/target/release/liballoy_ffi.a -lgcc_s -lutil -lrt -lpthread -lm -ldl -lc
#include <stdlib.h>
#include "alloy.h"
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func newComponent(name string, args []byte) (*C.alloy_component, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var cerr C.alloy_buf
	c := C.alloy_component_new(cname, bytesPtr(args), C.size_t(len(args)), &cerr)
	if c == nil {
		return nil, takeErr(&cerr)
	}
	return c, nil
}

func bytesPtr(b []byte) unsafe.Pointer {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Pointer(&b[0])
}

func takeErr(cerr *C.alloy_buf) error {
	msg := C.GoStringN((*C.char)(unsafe.Pointer(cerr.ptr)), C.int(cerr.len))
	C.alloy_buf_free(cerr)
	return errors.New(msg)
}

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
	c, err := newComponent(name, args)
	if err != nil {
		return nil, err
	}
	return &Source{c: c}, nil
}

// Run blocks until the source is closed.
func (s *Source) Run() {
	C.alloy_source_run(s.c)
}

// Recv blocks until the next batch and returns it with labels added to every
// stream, see decodeBatch. The bool is false once the source has stopped and
// every batch has been received.
func (s *Source) Recv(labels model.LabelSet) (loki.Batch, bool, error) {
	var buf C.alloy_buf
	if C.alloy_source_recv(s.c, &buf) == C.ALLOY_CLOSED {
		return loki.Batch{}, false, nil
	}
	defer C.alloy_buf_free(&buf)

	data := unsafe.Slice((*byte)(unsafe.Pointer(buf.ptr)), int(buf.len))
	batch, err := decodeBatch(data, labels)
	return batch, true, err
}

func (s *Source) Close() {
	C.alloy_component_close(s.c)
}

func (s *Source) Free() {
	C.alloy_component_free(s.c)
}

// Sink is a Rust sink component. Consume may be called concurrently and at
// any time before Free. Free must only be called once no Consume is running.
type Sink struct {
	c *C.alloy_component
}

// NewSink creates the Rust sink registered as name with args encoded by
// EncodeArgs.
func NewSink(name string, args []byte) (*Sink, error) {
	c, err := newComponent(name, args)
	if err != nil {
		return nil, err
	}
	return &Sink{c: c}, nil
}

// Consume passes batch to the sink. It returns loki.ErrConsumerStopped once
// the sink is closed.
func (s *Sink) Consume(batch loki.Batch) error {
	buf := encodeBatch(batch)

	var cerr C.alloy_buf
	switch C.alloy_sink_consume(s.c, (*C.uint8_t)(bytesPtr(buf)), C.size_t(len(buf)), &cerr) {
	case C.ALLOY_OK:
		return nil
	case C.ALLOY_CLOSED:
		return loki.ErrConsumerStopped
	default:
		return takeErr(&cerr)
	}
}

func (s *Sink) Close() {
	C.alloy_component_close(s.c)
}

func (s *Sink) Free() {
	C.alloy_component_free(s.c)
}

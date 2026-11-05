package rustgen

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../../rust/rustgen/include
#cgo LDFLAGS: ${SRCDIR}/../../../../../rust/target/release/librustgen.a -lgcc_s -lutil -lrt -lpthread -lm -ldl -lc
#include "rustgen.h"
*/
import "C"

import (
	"encoding/binary"
	"time"
	"unsafe"

	"github.com/grafana/loki/pkg/push"
)

// generator wraps the Rust generator. close may be called concurrently with
// next; free must only be called once next has returned.
type generator struct {
	g *C.rg_gen
}

func newGenerator(linesPerBatch int, interval time.Duration) *generator {
	return &generator{g: C.rg_new(C.uint32_t(linesPerBatch), C.uint64_t(interval.Milliseconds()))}
}

// next blocks until the next batch is generated and calls fn for each entry.
// It returns false once the generator is closed.
func (g *generator) next(fn func(push.Entry)) bool {
	var buf C.rg_buf
	if C.rg_next_batch(g.g, &buf) == C.RG_CLOSED {
		return false
	}
	defer C.rg_buf_free(&buf)

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
	C.rg_close(g.g)
}

func (g *generator) free() {
	C.rg_free(g.g)
}

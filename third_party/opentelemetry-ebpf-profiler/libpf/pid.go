package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

import (
	"fmt"
)

// PID represent Unix Process ID (pid_t)
type PID uint32

func (p PID) Hash32() uint32 {
	return uint32(p)
}

// PIDTID encodes a process id and a thread id
type PIDTID uint64

func (pt PIDTID) PID() PID {
	return PID(pt >> 32)
}

func (pt PIDTID) TID() PID {
	return PID(pt & 0xFFFFFFFF)
}

func (pt PIDTID) String() string {
	return fmt.Sprintf("PID: %v TID: %v", pt.PID(), pt.TID())
}

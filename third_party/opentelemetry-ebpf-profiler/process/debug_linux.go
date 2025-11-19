//go:build linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process // import "go.opentelemetry.io/ebpf-profiler/process"

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/remotememory"
)

type ptraceProcess struct {
	systemProcess
}

var _ Process = &ptraceProcess{}

func ptraceGetRegset(tid, regset int, data []byte) error {
	iovec := unix.Iovec{
		Base: &data[0],
		Len:  uint64(len(data)),
	}
	_, _, errno := unix.RawSyscall6(unix.SYS_PTRACE, unix.PTRACE_GETREGSET,
		uintptr(tid), uintptr(regset), uintptr(unsafe.Pointer(&iovec)), 0, 0)
	if errno != 0 {
		return fmt.Errorf("ptrace GETREGSET failed with errno %d", errno)
	}

	return nil
}

// NewPtrace attaches the calling goroutine to the target PID using unix
// PTrace API. The goroutine is locked to a system thread due to the PTrace
// API requirements.
// WARNING: All usage of Process interface to this implementation should be
// from one goroutine. If this is not sufficient in future, the implementation
// should be refactored to pass all requests via a proxy goroutine through
// channels so that the kernel requirements are fulfilled.
func NewPtrace(pid libpf.PID) (Process, error) {
	// Lock this goroutine to the OS thread. It is ptrace API requirement
	// that all ptrace calls must come from same thread.
	runtime.LockOSThread()

	sp := &ptraceProcess{}
	sp.pid = pid
	sp.remoteMemory = remotememory.RemoteMemory{ReaderAt: sp}
	if err := sp.attach(); err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	return sp, nil
}

func (sp *ptraceProcess) GetThreads() ([]ThreadInfo, error) {
	tidFiles, err := os.ReadDir(fmt.Sprintf("/proc/%d/task", sp.pid))
	if err != nil {
		return nil, err
	}

	threadInfo := make([]ThreadInfo, 0, len(tidFiles))

	ti, err := sp.getThreadInfo(int(sp.pid))
	if err != nil {
		return nil, err
	}
	threadInfo = append(threadInfo, ti)

	for _, tidFile := range tidFiles {
		if !tidFile.IsDir() {
			continue
		}
		tidNum, err := strconv.ParseInt(tidFile.Name(), 10, 32)
		if err != nil {
			continue
		}
		tid := int(tidNum)
		// The main thread is handled separately above.
		if tid == int(sp.pid) {
			continue
		}
		// Attach to the thread so the state can be queried.
		if err = unix.PtraceAttach(tid); err != nil {
			continue
		}
		status := unix.WaitStatus(0)
		_, _ = unix.Wait4(tid, &status, 0, nil)
		ti, err = sp.getThreadInfo(tid)
		_ = unix.PtraceDetach(tid)
		if err != nil {
			return nil, err
		}
		threadInfo = append(threadInfo, ti)
	}
	return threadInfo, nil
}

func (sp *ptraceProcess) attach() error {
	// Attach the main thread
	// Per ptrace API, this will send a SIGSTOP to the process
	// and suspend the whole process. However, the stopping happens
	// asynchronously and needs to be wait for.
	if err := unix.PtraceAttach(int(sp.pid)); err != nil {
		return err
	}

	// Synchronize with process stop.
	status := unix.WaitStatus(0)
	_, _ = unix.Wait4(int(sp.pid), &status, 0, nil)

	return nil
}

func (sp *ptraceProcess) ReadAt(p []byte, off int64) (n int, err error) {
	return unix.PtracePeekText(int(sp.pid), uintptr(off), p)
}

func (sp *ptraceProcess) Close() error {
	err := unix.PtraceDetach(int(sp.pid))
	runtime.UnlockOSThread()
	return err
}

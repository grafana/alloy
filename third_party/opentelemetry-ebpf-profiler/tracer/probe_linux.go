//go:build linux
// +build linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracer // import "go.opentelemetry.io/ebpf-profiler/tracer"

import (
	"bytes"
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// ProbeBPFSyscall checks if the syscall EBPF is available on the system.
func ProbeBPFSyscall() error {
	_, _, errNo := unix.Syscall(unix.SYS_BPF, uintptr(unix.BPF_PROG_TYPE_UNSPEC), uintptr(0), 0)
	if errNo == unix.ENOSYS {
		return errors.New("eBPF syscall is not available on your system")
	}
	return nil
}

// GetCurrentKernelVersion returns the major, minor and patch version of the kernel of the host
// from the utsname struct.
func GetCurrentKernelVersion() (major, minor, patch uint32, err error) {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return 0, 0, 0, fmt.Errorf("could not get Kernel Version: %v", err)
	}
	_, _ = fmt.Fscanf(bytes.NewReader(uname.Release[:]), "%d.%d.%d", &major, &minor, &patch)
	return major, minor, patch, nil
}

//go:build linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotememory // import "go.opentelemetry.io/ebpf-profiler/remotememory"

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func (vm ProcessVirtualMemory) ReadAt(p []byte, off int64) (int, error) {
	numBytesWanted := len(p)
	if numBytesWanted == 0 {
		return 0, nil
	}
	localIov := []unix.Iovec{{Base: &p[0], Len: uint64(numBytesWanted)}}
	remoteIov := []unix.RemoteIovec{{Base: uintptr(off), Len: numBytesWanted}}
	numBytesRead, err := unix.ProcessVMReadv(int(vm.pid), localIov, remoteIov, 0)
	if err != nil {
		err = fmt.Errorf("failed to read PID %v at 0x%x: %w", vm.pid, off, err)
	} else if numBytesRead != numBytesWanted {
		err = fmt.Errorf("failed to read PID %v at 0x%x: got only %d of %d",
			vm.pid, off, numBytesRead, numBytesWanted)
	}
	return numBytesRead, err
}

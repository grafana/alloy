// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rlimit // import "go.opentelemetry.io/ebpf-profiler/rlimit"

import (
	"fmt"

	"golang.org/x/sys/unix"

	log "github.com/sirupsen/logrus"
)

// MaximizeMemlock updates the memlock resource limit to RLIM_INFINITY.
// It returns a function to reset the resource limit to its original value or an error.
func MaximizeMemlock() (func(), error) {
	var oldLimit unix.Rlimit
	tmpLimit := unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	}

	if err := unix.Prlimit(0, unix.RLIMIT_MEMLOCK, &tmpLimit, &oldLimit); err != nil {
		return nil, fmt.Errorf("failed to set temporary rlimit: %w", err)
	}

	return func() {
		if err := unix.Setrlimit(unix.RLIMIT_MEMLOCK, &oldLimit); err != nil {
			log.Fatalf("Failed to set old rlimit: %v", err)
		}
	}, nil
}

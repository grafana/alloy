// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// successfailurecounter provides a wrapper to atomically increment success or failure counters.
//
// This package is **not** thread safe. Multiple increments to the same SuccessFailureCounter from
// different threads can result in incorrect counter results.
package successfailurecounter // import "go.opentelemetry.io/ebpf-profiler/successfailurecounter"

import (
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// SuccessFailureCounter implements a wrapper to increment success or failure counters exactly once.
type SuccessFailureCounter struct {
	success, fail *atomic.Uint64
	sealed        bool
}

// New returns a SuccessFailureCounter that can be incremented exactly once.
func New(success, fail *atomic.Uint64) SuccessFailureCounter {
	return SuccessFailureCounter{success: success, fail: fail}
}

// ReportSuccess increments the success counter or logs an error otherwise.
func (sfc *SuccessFailureCounter) ReportSuccess() {
	if sfc.sealed {
		log.Errorf("Attempted to report success/failure status more than once.")
		return
	}
	sfc.success.Add(1)
	sfc.sealed = true
}

// ReportFailure increments the failure counter or logs an error otherwise.
func (sfc *SuccessFailureCounter) ReportFailure() {
	if sfc.sealed {
		log.Errorf("Attempted to report failure/success status more than once.")
		return
	}
	sfc.fail.Add(1)
	sfc.sealed = true
}

// DefaultToSuccess increments the success counter if no counter was updated before.
func (sfc *SuccessFailureCounter) DefaultToSuccess() {
	if !sfc.sealed {
		sfc.success.Add(1)
	}
}

// DefaultToFailure increments the failure counter if no counter was updated before.
func (sfc *SuccessFailureCounter) DefaultToFailure() {
	if !sfc.sealed {
		sfc.fail.Add(1)
	}
}

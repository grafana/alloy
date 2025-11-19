// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reporter // import "go.opentelemetry.io/ebpf-profiler/reporter"

import (
	"time"

	"go.opentelemetry.io/ebpf-profiler/times"
)

// Compile time check to make sure config.Times satisfies the interfaces.
var _ Times = (*times.Times)(nil)

// Times is a subset of config.IntervalsAndTimers.
type Times interface {
	ReportInterval() time.Duration
	GRPCConnectionTimeout() time.Duration
	GRPCOperationTimeout() time.Duration
	GRPCStartupBackoffTime() time.Duration
	GRPCAuthErrorDelay() time.Duration
}

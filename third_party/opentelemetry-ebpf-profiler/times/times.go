// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package times // import "go.opentelemetry.io/ebpf-profiler/times"

import (
	"context"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/ebpf-profiler/periodiccaller"
)

const (
	// Number of timing samples to use when retrieving system boot time.
	sampleSize = 5
	// GRPCAuthErrorDelay defines the delay before triggering a global process exit after a
	// gRPC auth error.
	GRPCAuthErrorDelay = 10 * time.Minute
	// GRPCConnectionTimeout defines the timeout for each established gRPC connection.
	GRPCConnectionTimeout = 3 * time.Second
	// GRPCOperationTimeout defines the timeout for each gRPC operation.
	GRPCOperationTimeout = 5 * time.Second
	// GRPCStartupBackoffTimeout defines the time between failed gRPC requests during startup
	// phase.
	GRPCStartupBackoffTimeout = 1 * time.Minute
)

// Compile time check for interface adherence
var _ IntervalsAndTimers = (*Times)(nil)

var (
	// Monotonic-to-unixtime delta that can be added to a monotonic (CLOCK_MONOTONIC)
	// timestamp to convert it to time-since-epoch.
	bootTimeUnixNano atomic.Int64
)

// Times hold all the intervals and timeouts that are used across the host agent in a central place
// and comes with Getters to read them.
type Times struct {
	monitorInterval           time.Duration
	tracePollInterval         time.Duration
	reportInterval            time.Duration
	grpcConnectionTimeout     time.Duration
	grpcOperationTimeout      time.Duration
	grpcStartupBackoffTimeout time.Duration
	grpcAuthErrorDelay        time.Duration
	pidCleanupInterval        time.Duration
	probabilisticInterval     time.Duration
}

// IntervalsAndTimers is a meta-interface that exists purely to document its functionality.
type IntervalsAndTimers interface {
	// MonitorInterval defines the interval for PID event monitoring and metric collection.
	MonitorInterval() time.Duration
	// TracePollInterval defines the interval at which we read the trace perf event buffer.
	TracePollInterval() time.Duration
	// ReportInterval defines the interval at which collected data is sent to collection agent.
	ReportInterval() time.Duration
	// GRPCConnectionTimeout defines the timeout for each established gRPC connection.
	GRPCConnectionTimeout() time.Duration
	// GRPCOperationTimeout defines the timeout for each gRPC operation.
	GRPCOperationTimeout() time.Duration
	// GRPCStartupBackoffTime defines the time between failed gRPC requests during startup
	// phase.
	GRPCStartupBackoffTime() time.Duration
	// GRPCAuthErrorDelay defines the delay before triggering a global process exit after a
	// gRPC auth error.
	GRPCAuthErrorDelay() time.Duration
	// PIDCleanupInterval defines the interval at which monitored PIDs are checked for
	// liveness, and no longer living PIDs are cleaned up.
	PIDCleanupInterval() time.Duration
	// ProbabilisticInterval defines the interval for which probabilistic profiling will
	// be enabled or disabled.
	ProbabilisticInterval() time.Duration
}

func (t *Times) MonitorInterval() time.Duration { return t.monitorInterval }

func (t *Times) TracePollInterval() time.Duration { return t.tracePollInterval }

func (t *Times) ReportInterval() time.Duration { return t.reportInterval }

func (t *Times) GRPCConnectionTimeout() time.Duration { return t.grpcConnectionTimeout }

func (t *Times) GRPCOperationTimeout() time.Duration { return t.grpcOperationTimeout }

func (t *Times) GRPCStartupBackoffTime() time.Duration { return t.grpcStartupBackoffTimeout }

func (t *Times) GRPCAuthErrorDelay() time.Duration { return t.grpcAuthErrorDelay }

func (t *Times) PIDCleanupInterval() time.Duration { return t.pidCleanupInterval }

func (t *Times) ProbabilisticInterval() time.Duration { return t.probabilisticInterval }

// StartRealtimeSync calculates a delta between the monotonic clock
// (CLOCK_MONOTONIC, rebased to unixtime) and the realtime clock. If syncInterval is
// greater than zero, it also starts a goroutine to perform that calculation periodically.
func StartRealtimeSync(ctx context.Context, syncInterval time.Duration) {
	bootTimeUnixNano.Store(getBootTimeUnixNano())

	if syncInterval > 0 {
		periodiccaller.Start(ctx, syncInterval, func() {
			bootTimeUnixNano.Store(getBootTimeUnixNano())
		})
	}
}

// New returns a new Times instance.
func New(reportInterval, monitorInterval, probabilisticInterval time.Duration) *Times {
	return &Times{
		grpcAuthErrorDelay:        GRPCAuthErrorDelay,
		grpcConnectionTimeout:     GRPCConnectionTimeout,
		grpcOperationTimeout:      GRPCOperationTimeout,
		grpcStartupBackoffTimeout: GRPCStartupBackoffTimeout,
		pidCleanupInterval:        5 * time.Minute,
		tracePollInterval:         250 * time.Millisecond,
		reportInterval:            reportInterval,
		monitorInterval:           monitorInterval,
		probabilisticInterval:     probabilisticInterval,
	}
}

// getBootTimeUnixNano returns system boot time in nanoseconds since the
// epoch, temporarily locking the calling goroutine to its OS thread.
func getBootTimeUnixNano() int64 {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	samples := make([]struct {
		t1    time.Time
		ktime int64
		t2    time.Time
	}, sampleSize)

	for i := range samples {
		// To avoid noise from scheduling / other delays, we perform a
		// series of measurements and pick the one with the lowest delta.
		samples[i].t1 = time.Now()
		samples[i].ktime = int64(GetKTime())
		samples[i].t2 = time.Now()
	}

	sort.Slice(samples, func(i, j int) bool {
		di := samples[i].t2.UnixNano() - samples[i].t1.UnixNano()
		dj := samples[j].t2.UnixNano() - samples[j].t1.UnixNano()
		if di < 0 {
			di = -di
		}
		if dj < 0 {
			dj = -dj
		}
		return di < dj
	})

	// This should never be negative, as t1.UnixNano() >> ktime
	return samples[0].t1.UnixNano() - samples[0].ktime
}

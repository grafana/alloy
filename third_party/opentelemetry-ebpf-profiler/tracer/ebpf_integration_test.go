//go:build integration && linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracer_test

import (
	"math"
	"runtime"
	"sync"
	"testing"
	"time"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/ebpf-profiler/host"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/pyroscope/dynamicprofiling"
	"go.opentelemetry.io/ebpf-profiler/rlimit"
	"go.opentelemetry.io/ebpf-profiler/support"
	"go.opentelemetry.io/ebpf-profiler/tracer"
	tracertypes "go.opentelemetry.io/ebpf-profiler/tracer/types"
)

type mockIntervals struct{}

func (mockIntervals) MonitorInterval() time.Duration    { return 1 * time.Second }
func (mockIntervals) TracePollInterval() time.Duration  { return 250 * time.Millisecond }
func (mockIntervals) PIDCleanupInterval() time.Duration { return 1 * time.Second }

// forceContextSwitch makes sure two Go threads are running concurrently
// and that there will be a context switch between those two.
func forceContextSwitch() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		wg.Done()
	}(wg)
	wg.Wait()
}

// runKernelFrameProbe executes a perf event on the sched/sched_switch tracepoint
// that sends a selection of hand-crafted, predictable traces.
func runKernelFrameProbe(t *testing.T, tr *tracer.Tracer) {
	coll, err := support.LoadCollectionSpec()
	require.NoError(t, err)

	//nolint:staticcheck
	err = coll.RewriteMaps(tr.GetEbpfMaps())
	require.NoError(t, err)

	restoreRlimit, err := rlimit.MaximizeMemlock()
	require.NoError(t, err)
	defer restoreRlimit()

	prog, err := cebpf.NewProgram(coll.Programs["tracepoint_integration__sched_switch"])
	require.NoError(t, err)
	defer prog.Close()

	ev, err := link.Tracepoint("sched", "sched_switch", prog, nil)
	require.NoError(t, err)
	t.Logf("probe for Kernel frames installed on sched/sched_switch")

	// Manually trigger the tracepoint on sched/sched_switch.
	forceContextSwitch()

	t.Logf("tracepoint sched_switch triggered")
	err = ev.Close()
	require.NoError(t, err)
}

func validateTrace(t *testing.T, expected, returned *host.Trace) {
	t.Helper()

	require.Len(t, returned.Frames, len(expected.Frames))

	for i, expFrame := range expected.Frames {
		retFrame := returned.Frames[i]
		assert.Equal(t, expFrame.File, retFrame.File)
		assert.Equal(t, expFrame.Lineno, retFrame.Lineno)
		assert.Equal(t, expFrame.Type, retFrame.Type)
	}
}

func generateMaxLengthTrace() host.Trace {
	var trace host.Trace
	for i := 0; i < support.MaxFrameUnwinds; i++ {
		trace.Frames = append(trace.Frames, host.Frame{
			File:   ^host.FileID(i),
			Lineno: libpf.AddressOrLineno(i),
			Type:   support.FrameMarkerNative,
		})
	}
	return trace
}

func TestTraceTransmissionAndParsing(t *testing.T) {
	ctx := t.Context()

	enabledTracers, _ := tracertypes.Parse("")
	enabledTracers.Enable(tracertypes.PythonTracer)
	tr, err := tracer.NewTracer(ctx, &tracer.Config{
		Intervals:              &mockIntervals{},
		IncludeTracers:         enabledTracers,
		FilterErrorFrames:      false,
		SamplesPerSecond:       20,
		MapScaleFactor:         0,
		Policy:                 dynamicprofiling.AlwaysOnPolicy{},
		KernelVersionCheck:     true,
		BPFVerifierLogLevel:    0,
		ProbabilisticInterval:  100,
		ProbabilisticThreshold: 100,
		OffCPUThreshold:        1 * math.MaxUint32,
		VerboseMode:            true,
	})
	require.NoError(t, err)

	traceChan := make(chan *host.Trace, 16)
	err = tr.StartMapMonitors(ctx, traceChan)
	require.NoError(t, err)

	runKernelFrameProbe(t, tr)

	traces := make(map[uint8]*host.Trace)
	timeout := time.NewTimer(1 * time.Second)

	// Wait 1 second for traces to arrive.
Loop:
	for {
		select {
		case <-timeout.C:
			break Loop
		case trace := <-traceChan:
			require.GreaterOrEqual(t, len(trace.Comm), 4)
			require.Equal(t, "\xAA\xBB\xCC", trace.Comm[0:3])
			traces[trace.Comm[3]] = trace
		}
	}

	tests := map[string]struct {
		// id identifies the trace to inspect (encoded in COMM[3]).
		id uint8
		// hasKernelFrames indicates if the trace should contain kernel frames.
		hasKernelFrames bool
		// userSpaceTrace holds a single Trace with just the user-space portion of the trace
		// that will be verified against the returned Trace.
		userSpaceTrace host.Trace
	}{
		"Single Native Frame": {
			id: 1,
			userSpaceTrace: host.Trace{
				Frames: []host.Frame{{
					File:   1337,
					Lineno: 21,
					Type:   support.FrameMarkerNative,
				}},
			},
		},
		"Single Native Frame with Kernel Frames": {
			id:              2,
			hasKernelFrames: true,
			userSpaceTrace: host.Trace{
				Frames: []host.Frame{{
					File:   1337,
					Lineno: 21,
					Type:   support.FrameMarkerNative,
				}},
			},
		},
		"Three Python Frames": {
			id: 3,
			userSpaceTrace: host.Trace{
				Frames: []host.Frame{{
					File:   1337,
					Lineno: 42,
					Type:   support.FrameMarkerNative,
				}, {
					File:   1338,
					Lineno: 21,
					Type:   support.FrameMarkerNative,
				}, {
					File:   1339,
					Lineno: 22,
					Type:   support.FrameMarkerPython,
				}},
			},
		},
		"Maximum Length Trace": {
			id:              4,
			hasKernelFrames: true,
			userSpaceTrace:  generateMaxLengthTrace(),
		},
	}

	for name, testcase := range tests {
		testcase := testcase
		t.Run(name, func(t *testing.T) {
			trace, ok := traces[testcase.id]
			require.Truef(t, ok, "trace ID %d not received", testcase.id)

			numKernelFrames := len(trace.KernelFrames)
			userspaceFrameCount := len(trace.Frames)

			assert.Equal(t, len(testcase.userSpaceTrace.Frames), userspaceFrameCount)
			assert.False(t, !testcase.hasKernelFrames && numKernelFrames > 0,
				"unexpected kernel frames")

			// If this check fails it _could_ be a false positive, in that there is not
			// in fact anything wrong with the code being tested. We hope that the
			// kernel stack we capture has at least two frames, but it is possible that
			// it does not. If this happens frequently we should consider if there is a
			// different approach to checking this property without the possibility of
			// false positives.
			assert.Falsef(t, testcase.hasKernelFrames && numKernelFrames < 2,
				"expected at least 2 kernel frames, but got %d", numKernelFrames)

			t.Logf("Received %d user frames and %d kernel frames",
				userspaceFrameCount, numKernelFrames)

			validateTrace(t, &testcase.userSpaceTrace, trace)
		})
	}
}

func TestAllTracers(t *testing.T) {
	_, err := tracer.NewTracer(t.Context(), &tracer.Config{
		Intervals:              &mockIntervals{},
		IncludeTracers:         tracertypes.AllTracers(),
		SamplesPerSecond:       20,
		ProbabilisticInterval:  100,
		ProbabilisticThreshold: 100,
		OffCPUThreshold:        uint32(math.MaxUint32 / 100),
		VerboseMode:            true,
		LoadProbe:              true,
	})
	require.NoError(t, err)
}

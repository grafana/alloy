//go:build linux && (amd64 || arm64) && !nodocker

package reporter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter/args"
	"github.com/grafana/alloy/internal/component/pyroscope/testutil"
	"github.com/grafana/alloy/internal/component/pyroscope/util/test"
	pyroutil "github.com/grafana/alloy/internal/component/pyroscope/util/test/container"
	"github.com/grafana/alloy/internal/component/pyroscope/util/testlog"
)

// Reproduces https://github.com/grafana/opentelemetry-ebpf-profiler/issues/88.
func TestPPROFReporter_PyroscopeKeepsPartiallySymbolizedStacks(t *testing.T) {
	wg := sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := testlog.TestLogger(t)
	_, pyroscopeEndpoint := pyroutil.StartPyroscopeContainer(t, ctx, l)
	writeReceiver, writeComponent, err := testutil.CreateWriteComponent(l, prometheus.NewRegistry(), pyroscopeEndpoint)
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = writeComponent.Run(ctx)
	}()

	testMapping := func(name string, id uint64) libpf.FrameMapping {
		return libpf.NewFrameMapping(libpf.FrameMappingData{
			File: libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
				FileID:   libpf.NewFileID(id, id),
				FileName: libpf.Intern(name),
			}),
			Start: libpf.Address(id << 20),
			End:   libpf.Address(id<<20 + 0x10000),
		})
	}
	testFrames := func(leafFirst ...*libpf.Frame) libpf.Frames {
		res := make(libpf.Frames, 0, len(leafFirst))
		for _, f := range leafFirst {
			res.Append(f)
		}
		return res
	}
	libc := testMapping("libc.so.6", 1)
	exe := testMapping("work_c", 2)
	goExe := testMapping("work_go", 3)
	native := func(m libpf.FrameMapping, addr libpf.AddressOrLineno, name string) *libpf.Frame {
		return &libpf.Frame{Type: libpf.NativeFrame, AddressOrLineno: addr, FunctionName: libpf.Intern(name), Mapping: m}
	}
	python := func(name string, line libpf.SourceLineno) *libpf.Frame {
		return &libpf.Frame{Type: libpf.PythonFrame, FunctionName: libpf.Intern(name), SourceFile: libpf.Intern("work.py"), SourceLine: line}
	}
	// One comm label per stack, so each query returns a single sample and String() is stable.
	cases := []struct {
		comm     string
		frames   libpf.Frames
		expected string
	}{
		{
			comm: "native",
			frames: testFrames(
				native(exe, 0x2000, "main"),
				native(libc, 0x1100, ""),
				native(libc, 0x1200, "__libc_start_main"),
				native(exe, 0x2100, "_start"),
			),
			expected: `PeriodType: cpu nanoseconds
Period: 1000000000
Samples:
cpu/nanoseconds[dflt]
   10309278: 1 2 3 4 
Locations
     1: 0x0 M=1 main :0:0 s=0()
     2: 0x1100 M=2 
     3: 0x1200 M=2 __libc_start_main :0:0 s=0()
     4: 0x0 M=1 _start :0:0 s=0()
Mappings
1: 0x0/0x0/0x0 work_c  [FN]
2: 0x100000/0x110000/0x0 libc.so.6  
`,
		},
		{
			comm: "python",
			frames: testFrames(
				&libpf.Frame{Type: libpf.KernelFrame, AddressOrLineno: 0xffff0000},
				python("leaf_py", 3),
				python("main_py", 1),
			),
			expected: `PeriodType: cpu nanoseconds
Period: 1000000000
Samples:
cpu/nanoseconds[dflt]
   10309278: 1 2 3 
Locations
     1: 0xffff0000 M=1 
     2: 0x0 M=2 leaf_py work.py:3:0 s=0()
     3: 0x0 M=2 main_py work.py:1:0 s=0()
Mappings
1: 0x0/0x0/0x0   
2: 0x0/0x0/0x0   [FN][LN]
`,
		},
		{
			comm: "go",
			frames: testFrames(
				native(goExe, 0x3000, "main.leaf"),
				native(goExe, 0x3100, "main.main"),
			),
			expected: `PeriodType: cpu nanoseconds
Period: 1000000000
Samples:
cpu/nanoseconds[dflt]
   10309278: 1 2 
Locations
     1: 0x0 M=1 main.leaf :0:0 s=0()
     2: 0x0 M=1 main.main :0:0 s=0()
Mappings
1: 0x0/0x0/0x0 work_go  [FN]
`,
		},
	}
	events := samples.SampleToEvents{}
	for _, tc := range cases {
		events[samples.SampleKey{Comm: libpf.NewCommFromString(tc.comm)}] = &samples.TraceEvents{Timestamps: []uint64{1}, Frames: tc.frames}
	}
	reporter := newReporter()
	reporter.cfg.CommMode = args.CommModeLabel
	profiles := reporter.createProfile(samples.ResourceKey{PID: 123}, profileTypeSampling, events)
	require.Len(t, profiles, 1)
	err = writeReceiver.Appender().Append(ctx, profiles[0].Labels, []*pyroscope.RawSample{{RawProfile: profiles[0].Raw}})
	require.NoError(t, err)

	// Pyroscope zeroes the addresses of locations whose mapping claims HasFunctions, including unsymbolized ones.
	for _, tc := range cases {
		t.Run(tc.comm, func(t *testing.T) {
			var got string
			require.Eventually(t, func() bool {
				res, err := test.Query(pyroscopeEndpoint, &querierv1.SelectMergeProfileRequest{
					ProfileTypeID: `process_cpu:cpu:nanoseconds:cpu:nanoseconds`,
					LabelSelector: fmt.Sprintf(`{service_name="service_a", comm=%q}`, tc.comm),
					Start:         time.Now().Add(-time.Hour).UnixMilli(),
					End:           time.Now().Add(time.Hour).UnixMilli(),
				})
				if err != nil {
					t.Logf("Error querying endpoint: %v", err)
					return false
				}
				res.TimeNanos = 0
				got = res.String()
				return len(res.Sample) == 1
			}, 120*time.Second, time.Second, "stack never arrived; got\n%s", got)
			require.Equal(t, tc.expected, got)
		})
	}
}

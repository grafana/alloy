//go:build linux && (amd64 || arm64)

package reporter

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter/samples"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/testutil"
	"github.com/grafana/alloy/internal/component/pyroscope/util/test"
	pyroutil "github.com/grafana/alloy/internal/component/pyroscope/util/test/container"
	"github.com/grafana/alloy/internal/component/pyroscope/util/testlog"
)

// Reproduces https://github.com/grafana/opentelemetry-ebpf-profiler/issues/88.
func TestPPROFReporter_PyroscopeKeepsPartiallySymbolizedStacks(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("GITHUB_JOB") != "test_pyroscope" {
		t.Skip("Skipping Pyroscope integration test in GitHub Actions (job name is not test_pyroscope)")
	}
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

	libc := testMapping("libc.so.6", 1)
	exe := testMapping("work_c", 2)
	goExe := testMapping("work_go", 3)
	native := func(m libpf.FrameMapping, addr libpf.AddressOrLineno, name string) *libpf.Frame {
		return &libpf.Frame{Type: libpf.NativeFrame, AddressOrLineno: addr, FunctionName: libpf.Intern(name), Mapping: m}
	}
	python := func(name string, line libpf.SourceLineno) *libpf.Frame {
		return &libpf.Frame{Type: libpf.PythonFrame, FunctionName: libpf.Intern(name), SourceFile: libpf.Intern("work.py"), SourceLine: line}
	}
	events := samples.SampleToEvents{
		{TID: 1}: {Timestamps: []uint64{1}, Frames: testFrames(
			native(exe, 0x2000, "main"),
			native(libc, 0x1100, ""),
			native(libc, 0x1200, "__libc_start_main"),
			native(exe, 0x2100, "_start"),
		)},
		{TID: 2}: {Timestamps: []uint64{1}, Frames: testFrames(
			&libpf.Frame{Type: libpf.KernelFrame, AddressOrLineno: 0xffff0000},
			python("leaf_py", 3),
			python("main_py", 1),
		)},
		{TID: 3}: {Timestamps: []uint64{1}, Frames: testFrames(
			native(goExe, 0x3000, "main.leaf"),
			native(goExe, 0x3100, "main.main"),
		)},
	}
	profiles := newReporter().createProfile(samples.ResourceKey{PID: 123}, profileTypeSampling, events)
	require.Len(t, profiles, 1)
	// Pyroscope only sanitizes when a query merges more than one block, so keep pushing like pyroscope.ebpf does.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			_ = writeReceiver.Appender().Append(ctx, profiles[0].Labels, []*pyroscope.RawSample{{RawProfile: profiles[0].Raw}})
			time.Sleep(time.Second)
		}
	}()
	const minPushes = 10
	period := time.Second.Nanoseconds() / 97

	// Root first; "" is an unsymbolized frame that kept its address. The Go stack is fully symbolized and always survives.
	expected := []string{
		"_start;__libc_start_main;;main",
		"main.main;main.leaf",
		"main_py;leaf_py;",
	}
	var got []string
	require.Eventually(t, func() bool {
		res, err := test.Query(pyroscopeEndpoint, &querierv1.SelectMergeProfileRequest{
			ProfileTypeID: `process_cpu:cpu:nanoseconds:cpu:nanoseconds`,
			LabelSelector: `{service_name="service_a"}`,
			Start:         time.Now().Add(-time.Hour).UnixMilli(),
			End:           time.Now().Add(time.Hour).UnixMilli(),
		})
		if err != nil {
			t.Logf("Error querying endpoint: %v", err)
			return false
		}
		values := map[string]int64{}
		got = got[:0]
		for st, v := range rootFirstStacks(res) {
			values[st] = v
			got = append(got, st)
		}
		slices.Sort(got)
		return values["main.main;main.leaf"] >= minPushes*period
	}, 120*time.Second, time.Second, "fully symbolized stack never arrived from %d pushes; got %q", minPushes, got)
	require.Equal(t, expected, got)
}

func testMapping(name string, id uint64) libpf.FrameMapping {
	return libpf.NewFrameMapping(libpf.FrameMappingData{
		File: libpf.NewFrameMappingFile(libpf.FrameMappingFileData{
			FileID:   libpf.NewFileID(id, id),
			FileName: libpf.Intern(name),
		}),
		Start: libpf.Address(id << 20),
		End:   libpf.Address(id<<20 + 0x10000),
	})
}

func testFrames(leafFirst ...*libpf.Frame) libpf.Frames {
	res := make(libpf.Frames, 0, len(leafFirst))
	for _, f := range leafFirst {
		res.Append(f)
	}
	return res
}

func rootFirstStacks(p *profile.Profile) map[string]int64 {
	res := make(map[string]int64, len(p.Sample))
	for _, s := range p.Sample {
		names := make([]string, 0, len(s.Location))
		for i := len(s.Location) - 1; i >= 0; i-- {
			name := ""
			if l := s.Location[i]; len(l.Line) > 0 {
				name = l.Line[0].Function.Name
			}
			names = append(names, name)
		}
		res[strings.Join(names, ";")] += s.Value[0]
	}
	return res
}

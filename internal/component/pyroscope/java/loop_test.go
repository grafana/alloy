//go:build (linux || darwin) && (amd64 || arm64)

package java

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockProfiler struct {
	mock.Mock
}

func (m *mockProfiler) CopyLib(pid int) error {
	args := m.Called(pid)
	return args.Error(0)
}

func (m *mockProfiler) Execute(argv []string) (string, string, error) {
	args := m.Called(argv)
	return args.String(0), args.String(1), args.Error(2)
}

type mockAppendable struct {
	mock.Mock
}

func (m *mockAppendable) Upload(j debuginfo.UploadJob) {

}

func (m *mockAppendable) Client() debuginfogrpc.DebuginfoServiceClient {
	return nil
}

func (m *mockAppendable) Appender() pyroscope.Appender {
	return m
}

func (m *mockAppendable) Append(ctx context.Context, labels labels.Labels, samples []*pyroscope.RawSample) error {
	args := m.Called(ctx, labels, samples)
	return args.Error(0)
}

func (m *mockAppendable) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

func newTestProfilingLoop(_ *testing.T, profiler *mockProfiler, appendable pyroscope.Appendable) *profilingLoop {
	reg := prometheus.NewRegistry()
	output := pyroscope.NewFanout([]pyroscope.Appendable{appendable}, "test-appendable", reg)
	logger := log.NewNopLogger()
	cfg := ProfilingConfig{
		Interval:   10 * time.Millisecond,
		SampleRate: 1000,
		CPU:        true,
		Event:      "cpu",
	}
	target := discovery.NewTargetFromMap(map[string]string{"foo": "bar"})
	return newProfilingLoop(os.Getpid(), target, logger, profiler, output, cfg)
}

func TestProfilingLoop_StartStop(t *testing.T) {
	profiler := &mockProfiler{}
	appendable := &mockAppendable{}
	pid := os.Getpid()
	jfrPath := fmt.Sprintf("/tmp/asprof-%d-%d.jfr", pid, pid)

	pCh := make(chan *profilingLoop)

	profiler.On("CopyLib", pid).Return(nil).Once()

	// expect the profiler to be executed with the correct arguments to start it
	profiler.On("Execute", []string{
		"-f",
		jfrPath,
		"-o", "jfr",
		"-e", "cpu",
		"-i", "1000000",
		"start",
		"--timeout", "0",
		strconv.Itoa(pid),
	}).Run(func(args mock.Arguments) {
		// wait for the profiling loop to be created
		p := <-pCh

		// create the jfr file
		f, err := os.Create(p.jfrFile)
		require.NoError(t, err)
		defer f.Close()
	}).Return("", "", nil).Once()

	// expect the profiler to be executed with the correct arguments to stop it
	profiler.On("Execute", []string{
		"stop",
		"-o", "jfr",
		strconv.Itoa(pid),
	}).Return("", "", nil).Once()

	p := newTestProfilingLoop(t, profiler, appendable)
	pCh <- p

	// wait for the profiling loop to finish
	require.NoError(t, p.Close())

	// expect the profiler to clean up the jfr file
	_, err := os.Stat(p.jfrFile)
	require.True(t, os.IsNotExist(err))

	profiler.AssertExpectations(t)
	appendable.AssertExpectations(t)
}

func TestProfilingLoop_GenerateCommand(t *testing.T) {
	logger := log.NewNopLogger()
	pid := 12345
	jfrFile := "/tmp/test.jfr"

	tests := []struct {
		name     string
		cfg      ProfilingConfig
		expected []string
	}{
		{
			name: "default CPU profiling",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				CPU:        true,
				Event:      "itimer",
				SampleRate: 100,
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"-e", "itimer", "-i", "10000000",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "CPU disabled with custom event",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				CPU:        false,
				Event:      "wall",
				SampleRate: 100,
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "per_thread mode",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				CPU:        true,
				Event:      "cpu",
				PerThread:  true,
				SampleRate: 100,
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"-e", "cpu", "-t", "-i", "10000000",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "all events enabled",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				All:        true,
				CPU:        true,
				Event:      "itimer",
				SampleRate: 100,
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"--all", "-e", "itimer", "-i", "10000000",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "memory profiling options",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				CPU:        true,
				Event:      "itimer",
				SampleRate: 100,
				Alloc:      "512k",
				Live:       true,
				NativeMem:  "1m",
				NoFree:     true,
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"-e", "itimer", "-i", "10000000",
				"--alloc", "512k", "--live",
				"--nativemem", "1m", "--nofree",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "lock profiling options",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				CPU:        true,
				Event:      "itimer",
				SampleRate: 100,
				Lock:       "10ms",
				NativeLock: "5ms",
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"-e", "itimer", "-i", "10000000",
				"--lock", "10ms", "--nativelock", "5ms",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "filters and stack options",
			cfg: ProfilingConfig{
				Interval:    60 * time.Second,
				CPU:         true,
				Event:       "itimer",
				SampleRate:  100,
				Include:     []string{"com.example.*", "org.test.*"},
				Exclude:     []string{"*Unsafe.park*"},
				JStackDepth: 1024,
				CStack:      "dwarf",
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"-e", "itimer", "-i", "10000000",
				"-I", "com.example.*", "-I", "org.test.*",
				"-X", "*Unsafe.park*",
				"-j", "1024", "--cstack", "dwarf",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "advanced CPU options",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				CPU:        true,
				Event:      "itimer",
				SampleRate: 100,
				Wall:       "100ms",
				AllUser:    true,
				Filter:     "120-127",
				Sched:      true,
				TargetCPU:  2,
				RecordCPU:  true,
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"-e", "itimer", "-i", "10000000",
				"--wall", "100ms", "--all-user",
				"--filter", "120-127", "--sched",
				"--target-cpu", "2", "--record-cpu",
				"start", "--timeout", "60", "12345",
			},
		},
		{
			name: "JFR options",
			cfg: ProfilingConfig{
				Interval:   60 * time.Second,
				CPU:        true,
				Event:      "itimer",
				SampleRate: 100,
				LogLevel:   "DEBUG",
				Features:   []string{"stats", "vtable"},
				Trace:      []string{"my.pkg.Method:50ms"},
				JFRSync:    "profile",
				Clock:      "monotonic",
			},
			expected: []string{
				"-f", jfrFile, "-o", "jfr",
				"-e", "itimer", "-i", "10000000",
				"-L", "DEBUG",
				"-F", "stats,vtable",
				"--trace", "my.pkg.Method:50ms",
				"--jfrsync", "profile",
				"--clock", "monotonic",
				"start", "--timeout", "60", "12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &profilingLoop{
				logger:  logger,
				pid:     pid,
				jfrFile: jfrFile,
				cfg:     tt.cfg,
			}

			result := p.generateCommand()
			require.Equal(t, tt.expected, result)
		})
	}
}

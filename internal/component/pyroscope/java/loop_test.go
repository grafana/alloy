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

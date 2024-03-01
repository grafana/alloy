//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/pyroscope"
	"github.com/grafana/agent/internal/util"
	ebpfspy "github.com/grafana/pyroscope/ebpf"
	"github.com/grafana/pyroscope/ebpf/pprof"
	"github.com/grafana/pyroscope/ebpf/sd"
	"github.com/grafana/river"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type mockSession struct {
	options      ebpfspy.SessionOptions
	collectError error
	collected    int
	data         [][]string
	dataTarget   *sd.Target
}

func (m *mockSession) Start() error {
	return nil
}

func (m *mockSession) Stop() {

}

func (m *mockSession) Update(options ebpfspy.SessionOptions) error {
	m.options = options
	return nil
}

func (m *mockSession) UpdateTargets(_ sd.TargetsOptions) {

}

func (m *mockSession) CollectProfiles(f pprof.CollectProfilesCallback) error {
	m.collected++
	if m.collectError != nil {
		return m.collectError
	}
	for _, stack := range m.data {
		f(
			pprof.ProfileSample{
				Target:      m.dataTarget,
				Pid:         0,
				SampleType:  pprof.SampleTypeCpu,
				Aggregation: pprof.SampleAggregation(false),
				Stack:       stack,
				Value:       1,
				Value2:      0,
			})
	}
	return nil
}

func (m *mockSession) DebugInfo() interface{} {
	return nil
}

func TestShutdownOnError(t *testing.T) {
	logger := util.TestFlowLogger(t)
	ms := newMetrics(nil)
	targetFinder, err := sd.NewTargetFinder(os.DirFS("/foo"), logger, sd.TargetsOptions{
		ContainerCacheSize: 1024,
	})
	require.NoError(t, err)
	session := &mockSession{}
	arguments := defaultArguments()
	arguments.CollectInterval = time.Millisecond * 100
	c := newTestComponent(
		component.Options{
			Logger:        logger,
			Registerer:    prometheus.NewRegistry(),
			OnStateChange: func(e component.Exports) {},
		},
		arguments,
		session,
		targetFinder,
		ms,
	)

	session.collectError = fmt.Errorf("mocked error collecting profiles")
	err = c.Run(context.TODO())
	require.Error(t, err)
}

func TestContextShutdown(t *testing.T) {
	logger := util.TestFlowLogger(t)
	ms := newMetrics(nil)
	targetFinder, err := sd.NewTargetFinder(os.DirFS("/foo"), logger, sd.TargetsOptions{
		ContainerCacheSize: 1024,
	})
	require.NoError(t, err)
	session := &mockSession{}
	arguments := defaultArguments()
	arguments.CollectInterval = time.Millisecond * 100
	c := newTestComponent(
		component.Options{
			Logger:        logger,
			Registerer:    prometheus.NewRegistry(),
			OnStateChange: func(e component.Exports) {},
		},
		arguments,
		session,
		targetFinder,
		ms,
	)

	session.data = [][]string{
		{"a", "b", "c"},
		{"q", "w", "e"},
	}
	session.dataTarget = sd.NewTarget("cid", 0, map[string]string{"service_name": "foo"})
	var g run.Group
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*1))
	defer cancel()
	g.Add(func() error {
		err = c.Run(ctx)
		require.NoError(t, err)
		return nil
	}, func(err error) {

	})
	g.Add(func() error {
		time.Sleep(time.Millisecond * 300)
		arguments.SampleRate = 4242
		err := c.Update(arguments)
		require.NoError(t, err)
		return nil
	}, func(err error) {

	})
	err = g.Run()
	require.NoError(t, err)
	require.Greater(t, session.collected, 5)
	require.Equal(t, session.options.SampleRate, 4242)
}

func TestUnmarshalConfig(t *testing.T) {
	var arg Arguments
	err := river.Unmarshal([]byte(`targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
collect_interval = "3s"
sample_rate = 239
pid_cache_size = 1000
build_id_cache_size = 2000
same_file_cache_size = 3000
container_id_cache_size = 4000
cache_rounds = 4
collect_user_profile = true
collect_kernel_profile = false`), &arg)
	require.NoError(t, err)
	require.Empty(t, arg.ForwardTo)
	require.Equal(t, time.Second*3, arg.CollectInterval)
	require.Equal(t, 239, arg.SampleRate)
	require.Equal(t, 1000, arg.PidCacheSize)
	require.Equal(t, 2000, arg.BuildIDCacheSize)
	require.Equal(t, 3000, arg.SameFileCacheSize)
	require.Equal(t, 4000, arg.ContainerIDCacheSize)
	require.Equal(t, 4, arg.CacheRounds)
	require.Equal(t, true, arg.CollectUserProfile)
	require.Equal(t, false, arg.CollectKernelProfile)
}

func TestUnmarshalBadConfig(t *testing.T) {
	var arg Arguments
	err := river.Unmarshal([]byte(`targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
collect_interval = 3s"
sample_rate = 239
pid_cache_size = 1000
build_id_cache_size = 2000
same_file_cache_size = 3000
container_id_cache_size = 4000
cache_rounds = 4
collect_user_profile = true
collect_kernel_profile = false`), &arg)
	require.Error(t, err)
}

func newTestComponent(opts component.Options, args Arguments, session *mockSession, targetFinder sd.TargetFinder, ms *metrics) *Component {
	flowAppendable := pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer)
	res := &Component{
		options:      opts,
		metrics:      ms,
		appendable:   flowAppendable,
		args:         args,
		targetFinder: targetFinder,
		session:      session,
		argsUpdate:   make(chan Arguments),
	}
	res.metrics.targetsActive.Set(float64(len(res.targetFinder.DebugInfo())))
	return res
}

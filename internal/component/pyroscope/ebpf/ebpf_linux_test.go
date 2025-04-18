//go:build (linux && arm64) || (linux && amd64)

package ebpf

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	ebpfspy "github.com/grafana/pyroscope/ebpf"
	"github.com/grafana/pyroscope/ebpf/pprof"
	"github.com/grafana/pyroscope/ebpf/sd"
	"github.com/grafana/pyroscope/ebpf/symtab"
	"github.com/grafana/pyroscope/ebpf/symtab/elf"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

type mockSession struct {
	options         ebpfspy.SessionOptions
	collectCallback func() error
	collected       int
	data            [][]string
	dataTarget      *sd.Target
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
	if m.collectCallback != nil {
		return m.collectCallback()
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
	return ebpfspy.SessionDebugInfo{
		ElfCache: symtab.ElfCacheDebugInfo{
			BuildIDCache: symtab.GCacheDebugInfo[elf.SymTabDebugInfo]{},
			SameFileCache: symtab.GCacheDebugInfo[elf.SymTabDebugInfo]{
				LRUSize:      10,
				RoundSize:    10,
				CurrentRound: 1,
				LRUDump: []elf.SymTabDebugInfo{
					{
						Name:          "X",
						Size:          123,
						MiniDebugInfo: false,
						LastUsedRound: 1,
					},
				},
			},
		},
		PidCache: symtab.GCacheDebugInfo[symtab.ProcTableDebugInfo]{
			LRUSize:      10,
			RoundSize:    10,
			CurrentRound: 1,
			LRUDump: []symtab.ProcTableDebugInfo{
				{
					Pid:  666,
					Size: 123,
				},
			},
		},
		Arch:   "my-arch",
		Kernel: "my-kernel",
	}
}

func TestTargetUpdatesWithLongCollection(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	ms := newMetrics(nil)
	targetFinder, err := sd.NewTargetFinder(os.DirFS("/foo"), logger, sd.TargetsOptions{
		ContainerCacheSize: 1024,
	})
	require.NoError(t, err)
	session := &mockSession{}
	arguments := NewDefaultArguments()
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

	collection := make(chan struct{})

	// simulate a long collection
	session.collectCallback = func() error {
		<-collection
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		err = c.Run(ctx)
		require.NoError(t, err)
		wg.Done()
	}()

	// now schedule 3 updates
	c.Update(arguments)
	c.Update(arguments)
	c.Update(arguments)
	argX := NewDefaultArguments()
	argX.SampleRate = 1234
	c.Update(argX)

	// unblock the collection
	close(collection)

	// wait for the session to be updated
	require.Eventually(t, func() bool {
		return session.options.SampleRate == 1234
	}, time.Second*1, time.Millisecond*10)

	cancel()
	wg.Wait()
}

func TestReportingCollectError(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	ms := newMetrics(nil)
	targetFinder, err := sd.NewTargetFinder(os.DirFS("/foo"), logger, sd.TargetsOptions{
		ContainerCacheSize: 1024,
	})
	require.NoError(t, err)
	session := &mockSession{}
	arguments := NewDefaultArguments()
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

	session.collectCallback = func() error { return fmt.Errorf("mocked error collecting profiles") }
	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		err = c.Run(ctx)
		require.NoError(t, err)
		wg.Done()
	}()

	// expect the component to be unhealthy
	require.Eventually(t, func() bool {
		if c.CurrentHealth().Health == component.HealthTypeUnhealthy {
			require.Equal(t, c.CurrentHealth().Message, "ebpf session collectProfiles mocked error collecting profiles")
			return true
		}
		return false
	}, time.Second*1, time.Millisecond*10)

	// the component should still be handling update requests
	err = c.Update(arguments)
	require.NoError(t, err)

	cancel()
	wg.Wait()
}

func TestContextShutdown(t *testing.T) {
	logger := util.TestAlloyLogger(t)
	ms := newMetrics(nil)
	targetFinder, err := sd.NewTargetFinder(os.DirFS("/foo"), logger, sd.TargetsOptions{
		ContainerCacheSize: 1024,
	})
	require.NoError(t, err)
	session := &mockSession{}
	arguments := NewDefaultArguments()
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
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Second*1))
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
	for _, tt := range []struct {
		name        string
		in          string
		expected    func() Arguments
		expectedErr string
	}{
		{
			name: "required-params-only",
			in: `
targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
`,
			expected: func() Arguments {
				x := NewDefaultArguments()
				x.Targets = []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"container_id": "cid",
						"service_name": "foo",
					}),
				}
				x.ForwardTo = []pyroscope.Appendable{}
				return x
			},
		},
		{
			name: "full-config",
			in: `
targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
collect_interval = "3s"
sample_rate = 239
pid_cache_size = 1000
build_id_cache_size = 2000
same_file_cache_size = 3000
container_id_cache_size = 4000
cache_rounds = 4
collect_user_profile = true
collect_kernel_profile = false`,
			expected: func() Arguments {
				x := NewDefaultArguments()
				x.Targets = []discovery.Target{
					discovery.NewTargetFromMap(map[string]string{
						"container_id": "cid",
						"service_name": "foo",
					}),
				}
				x.ForwardTo = []pyroscope.Appendable{}
				x.CollectInterval = time.Second * 3
				x.SampleRate = 239
				x.PidCacheSize = 1000
				x.BuildIDCacheSize = 2000
				x.SameFileCacheSize = 3000
				x.ContainerIDCacheSize = 4000
				x.CacheRounds = 4
				x.CollectUserProfile = true
				x.CollectKernelProfile = false
				return x
			},
		},
		{
			name: "syntax-problem",
			in: `
targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
collect_interval = 3s"
`,
			expectedErr: "4:21: expected TERMINATOR, got IDENT (and 1 more diagnostics)",
		},
		{
			name: "incorrect-map-sizes",
			in: `
targets = [{"service_name" = "foo", "container_id"= "cid"}]
forward_to = []
symbols_map_size = -1
pid_map_size = 0
`,
			expectedErr: "symbols_map_size must be greater than 0\npid_map_size must be greater than 0",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			arg := Arguments{}
			if tt.expectedErr != "" {
				err := syntax.Unmarshal([]byte(tt.in), &arg)
				require.Error(t, err)
				require.Equal(t, tt.expectedErr, err.Error())
				return
			}
			require.NoError(t, syntax.Unmarshal([]byte(tt.in), &arg))
			require.Equal(t, tt.expected(), arg)
		})
	}
}

type mockTargetFinder struct {
	sd.TargetFinder
}

func (m *mockTargetFinder) DebugInfo() []map[string]string {
	return []map[string]string{
		{"__container_id__": "foo", "__name__": "process_cpu", "container": "kube-proxy"},
		{"__container_id__": "baz", "__name__": "process_cpu", "container": "kube-proxy"},
	}
}

func TestDebugInfo(t *testing.T) {
	c := &Component{
		session:      &mockSession{},
		targetFinder: &mockTargetFinder{},
	}

	c.updateDebugInfo()
	di := c.DebugInfo()

	v, err := syntax.Marshal(di)
	require.NoError(t, err)

	require.Equal(t, `targets = [{
	__container_id__ = "foo",
	__name__         = "process_cpu",
	container        = "kube-proxy",
}, {
	__container_id__ = "baz",
	__name__         = "process_cpu",
	container        = "kube-proxy",
}]
session = {
	elf_cache = {
		build_id_cache = {
			lru_size      = 0,
			round_size    = 0,
			current_round = 0,
			lru_dump      = [],
			round_dump    = [],
		},
		same_file_cache = {
			lru_size      = 10,
			round_size    = 10,
			current_round = 1,
			lru_dump      = [{
				name            = "X",
				symbol_count    = 123,
				file            = "",
				mini_debug_info = false,
				last_used_round = 1,
			}],
			round_dump = [],
		},
	},
	pid_cache = {
		lru_size      = 10,
		round_size    = 10,
		current_round = 1,
		lru_dump      = [{
			elfs = {},
			size            = 123,
			pid             = 666,
			last_used_round = 0,
		}],
		round_dump = [],
	},
	arch   = "my-arch",
	kernel = "my-kernel",
}`, string(v))
}

func newTestComponent(opts component.Options, args Arguments, session *mockSession, targetFinder sd.TargetFinder, ms *metrics) *Component {
	alloyAppendable := pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer)
	res := &Component{
		options:      opts,
		metrics:      ms,
		appendable:   alloyAppendable,
		args:         args,
		targetFinder: targetFinder,
		session:      session,
		argsUpdate:   make(chan Arguments),
	}
	res.metrics.targetsActive.Set(float64(len(res.targetFinder.DebugInfo())))
	return res
}

//go:build linux

package cadvisor

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/euank/go-kmsg-parser/kmsgparser"
	"github.com/google/cadvisor/lib/container"
	"github.com/google/cadvisor/lib/metrics"
	info "github.com/google/cadvisor/lib/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"
	"k8s.io/utils/clock"

	"github.com/grafana/alloy/internal/util"
)

func TestParseOOMs(t *testing.T) {
	t0 := time.Unix(1000, 0)
	lines := []string{
		"some unrelated kernel message",
		// Linux 5.0+ format.
		"stress invoked oom-killer: gfp_mask=0xcc0(GFP_KERNEL), order=0, oom_score_adj=0",
		"CPU: 3 PID: 1234 Comm: stress Not tainted 6.8.0-45-generic",
		"oom-kill:constraint=CONSTRAINT_MEMCG,nodemask=(null),cpuset=docker-abc.scope,mems_allowed=0,oom_memcg=/system.slice/docker-abc.scope,task_memcg=/system.slice/docker-abc.scope,task=stress,pid=1234,uid=0",
		"Memory cgroup out of memory: Killed process 1234 (stress) total-vm:600000kB",
		// Pre-5.0 format.
		"java invoked oom-killer: gfp_mask=0x24000c0, order=0, oom_score_adj=0",
		"Task in /kubepods/burstable/pod1/ctr1 killed as a result of limit of /kubepods/burstable/pod1",
		"Memory cgroup out of memory: Killed process 42 (java) score 1000 or sacrifice child",
		// A second OOM in the first container.
		"stress invoked oom-killer: gfp_mask=0xcc0(GFP_KERNEL), order=0, oom_score_adj=0",
		"oom-kill:constraint=CONSTRAINT_MEMCG,nodemask=(null),cpuset=docker-abc.scope,mems_allowed=0,oom_memcg=/system.slice/docker-abc.scope,task_memcg=/system.slice/docker-abc.scope,task=stress,pid=1240,uid=0",
		// A report cut short when the stream closes is not counted.
		"other invoked oom-killer: gfp_mask=0xcc0(GFP_KERNEL), order=0, oom_score_adj=0",
	}

	msgs := make(chan kmsgparser.Message, len(lines))
	for i, line := range lines {
		msgs <- kmsgparser.Message{Message: line, Timestamp: t0.Add(time.Duration(i) * time.Second)}
	}
	close(msgs)

	var got []oomEvent
	parseOOMs(msgs, func(ev oomEvent) { got = append(got, ev) })

	require.Equal(t, []oomEvent{
		{ContainerName: "/system.slice/docker-abc.scope", Timestamp: t0.Add(3 * time.Second)},
		{ContainerName: "/kubepods/burstable/pod1/ctr1", Timestamp: t0.Add(7 * time.Second)},
		{ContainerName: "/system.slice/docker-abc.scope", Timestamp: t0.Add(9 * time.Second)},
	}, got)
}

// fakeInfoProvider returns the same cached stats on every call, like the
// cAdvisor manager does.
type fakeInfoProvider struct {
	infos map[string]*info.ContainerInfo
}

func (f *fakeInfoProvider) GetRequestedContainersInfo(string, info.RequestOptions) (map[string]*info.ContainerInfo, error) {
	out := make(map[string]*info.ContainerInfo, len(f.infos))
	for k, v := range f.infos {
		ci := *v
		ci.Stats = append([]*info.ContainerStats(nil), v.Stats...)
		out[k] = &ci
	}
	return out, nil
}

func (f *fakeInfoProvider) GetVersionInfo() (*info.VersionInfo, error) {
	return &info.VersionInfo{}, nil
}

func (f *fakeInfoProvider) GetMachineInfo() (*info.MachineInfo, error) {
	return &info.MachineInfo{}, nil
}

func newFakeInfoProvider(names ...string) *fakeInfoProvider {
	f := &fakeInfoProvider{infos: map[string]*info.ContainerInfo{}}
	for _, name := range names {
		f.infos[name] = &info.ContainerInfo{
			ContainerReference: info.ContainerReference{Name: name},
			Stats:              []*info.ContainerStats{{Timestamp: time.Now()}},
		}
	}
	return f
}

func TestOOMInfoProvider(t *testing.T) {
	fake := newFakeInfoProvider("/a", "/b")
	counter := newOOMCounter()
	counter.inc("/a")
	counter.inc("/a")
	counter.inc("/not-a-container")

	p := &oomInfoProvider{containerInfoProvider: fake, counter: counter}
	infos, err := p.GetRequestedContainersInfo("/", info.RequestOptions{})
	require.NoError(t, err)

	require.Equal(t, uint64(2), infos["/a"].Stats[0].OOMEvents)
	require.Equal(t, uint64(0), infos["/b"].Stats[0].OOMEvents)

	// The cached stats are not modified.
	require.Equal(t, uint64(0), fake.infos["/a"].Stats[0].OOMEvents)
}

func TestOOMInfoProvider_PrometheusCollector(t *testing.T) {
	fake := newFakeInfoProvider("/system.slice/docker-abc.scope", "/system.slice/docker-def.scope")
	counter := newOOMCounter()
	counter.inc("/system.slice/docker-abc.scope")

	col := metrics.NewPrometheusCollector(
		&oomInfoProvider{containerInfoProvider: fake, counter: counter},
		metrics.BaseContainerLabels(nil),
		container.MetricSet{container.OOMMetrics: struct{}{}},
		clock.RealClock{},
		info.RequestOptions{},
	)
	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(col))

	require.Equal(t, map[string]float64{
		"/system.slice/docker-abc.scope": 1,
		"/system.slice/docker-def.scope": 0,
	}, gatherOOMEvents(t, reg))

	counter.inc("/system.slice/docker-abc.scope")
	counter.inc("/system.slice/docker-def.scope")

	require.Equal(t, map[string]float64{
		"/system.slice/docker-abc.scope": 2,
		"/system.slice/docker-def.scope": 1,
	}, gatherOOMEvents(t, reg))
}

// gatherOOMEvents returns container_oom_events_total by container id.
func gatherOOMEvents(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)

	out := map[string]float64{}
	for _, mf := range families {
		if mf.GetName() != "container_oom_events_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "id" {
					out[lp.GetValue()] = m.GetCounter().GetValue()
				}
			}
		}
	}
	return out
}

func TestRunWithOOMWatcher(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	var srcDone atomic.Bool
	src := func(ctx context.Context, emit func(oomEvent)) error {
		defer srcDone.Store(true)
		emit(oomEvent{ContainerName: "/a"})
		emit(oomEvent{ContainerName: "/a"})
		emit(oomEvent{ContainerName: "/b"})
		<-ctx.Done()
		return nil
	}

	counter := newOOMCounter()
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan struct{})
	errc := make(chan error, 1)
	go func() {
		errc <- runWithOOMWatcher(ctx, util.TestAlloyLogger(t).Slog(), src, counter, func() error {
			// The watcher has exited before the manager is stopped.
			assert.True(t, srcDone.Load())
			close(stopped)
			return errors.New("stop error")
		})
	}()

	require.Eventually(t, func() bool {
		return counter.get("/a") == 2 && counter.get("/b") == 1
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case err := <-errc:
		require.EqualError(t, err, "stop error")
	case <-time.After(5 * time.Second):
		t.Fatal("runWithOOMWatcher did not return after the context was canceled")
	}
	<-stopped
}

func TestRunWithOOMWatcher_SourceFails(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	src := func(context.Context, func(oomEvent)) error {
		return errors.New("permission denied")
	}

	ctx, cancel := context.WithCancel(t.Context())
	errc := make(chan error, 1)
	go func() {
		errc <- runWithOOMWatcher(ctx, util.TestAlloyLogger(t).Slog(), src, newOOMCounter(), func() error { return nil })
	}()

	// A failing source doesn't stop the integration.
	select {
	case err := <-errc:
		t.Fatalf("runWithOOMWatcher returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	require.NoError(t, <-errc)
}

func TestRunWithOOMWatcher_NilSource(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, runWithOOMWatcher(ctx, util.TestAlloyLogger(t).Slog(), nil, newOOMCounter(), func() error { return nil }))
}

// TestKmsgOOMSource_Stops checks that the real kernel log source releases
// /dev/kmsg and its goroutines when the context is canceled. It needs read
// access to /dev/kmsg, so it is skipped otherwise.
func TestKmsgOOMSource_Stops(t *testing.T) {
	f, err := os.Open("/dev/kmsg")
	if err != nil {
		t.Skipf("can't read /dev/kmsg: %v", err)
	}
	_ = f.Close()

	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithCancel(t.Context())
	errc := make(chan error, 1)
	go func() {
		errc <- kmsgOOMSource(util.TestAlloyLogger(t).Slog())(ctx, func(oomEvent) {})
	}()

	// Give the source time to open /dev/kmsg and read the existing ring buffer.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errc:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("kmsg source did not stop after the context was canceled")
	}
}

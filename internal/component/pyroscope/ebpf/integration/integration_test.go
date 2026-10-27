//go:build linux && (amd64 || arm64)

package integration

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf"
	"github.com/grafana/alloy/internal/component/pyroscope/testutil"
	"github.com/grafana/alloy/internal/component/pyroscope/util/test"
	pyroutil "github.com/grafana/alloy/internal/component/pyroscope/util/test/container"
	"github.com/grafana/alloy/internal/component/pyroscope/util/testlog"
)

const perlScript = `sub leaf_perl { my $s = 0; $s += $_ * $_ for 1..200000; return $s }
sub outer_perl { while (1) { leaf_perl() } }
outer_perl();`

// Reproduces https://github.com/grafana/opentelemetry-ebpf-profiler/issues/92.
func TestPyroscopeEbpfPerlIntegration(t *testing.T) {
	test.SkipUnlessPyroscopeJob(t)
	if os.Geteuid() != 0 {
		t.Skip("pyroscope.ebpf needs root")
	}
	wg := sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := testlog.TestLogger(t)
	_, pyroscopeEndpoint := pyroutil.StartPyroscopeContainer(t, ctx, l)

	_, perlPid := pyroutil.StartPerlContainer(t, ctx, l, perlScript)

	reg := prometheus.NewRegistry()
	writeReceiver, writeComponent, err := testutil.CreateWriteComponent(l, reg, pyroscopeEndpoint)
	require.NoError(t, err)

	args := ebpf.NewDefaultArguments()
	args.ForwardTo = []pyroscope.Appendable{writeReceiver}
	args.CollectInterval = 5 * time.Second
	args.SymbCachePath = t.TempDir()
	args.Targets = []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{
			"__process_pid__": strconv.Itoa(perlPid),
			"service_name":    "ebpf-perl",
		}),
	}
	ebpfComponent, err := ebpf.New(l.With("component", "pyroscope.ebpf"), reg, "test-ebpf", args)
	require.NoError(t, err)

	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = writeComponent.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		_ = ebpfComponent.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return samplesSent(reg, "ebpf-perl") > 0
	}, 90*time.Second, time.Second, "pyroscope.ebpf sent no perl samples")
	t.Logf("pyroscope.ebpf sent %v perl samples", samplesSent(reg, "ebpf-perl"))

	// Root first; "*" matches exactly one frame (__libc_start_call_main, unsymbolized).
	expected := []string{"_start", "__libc_start_main", "*", "main", "perl_run", "Perl_runops_standard"}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		res, err := test.Query(pyroscopeEndpoint, &querierv1.SelectMergeProfileRequest{
			ProfileTypeID: `process_cpu:cpu:nanoseconds:cpu:nanoseconds`,
			LabelSelector: `{service_name="ebpf-perl"}`,
			Start:         time.Now().Add(-time.Hour).UnixMilli(),
			End:           time.Now().UnixMilli(),
		})
		if !assert.NoError(c, err) {
			return
		}
		seen := stacks(res)
		assert.Truef(c, slices.ContainsFunc(seen, func(st []string) bool {
			return hasPrefix(st, expected) && hasPerlFrames(st[len(expected):])
		}), "no stack with prefix %v followed by outer_perl and leaf_perl; got %d stacks: %v", expected, len(seen), seen)
	}, 90*time.Second, time.Second)
}

func hasPerlFrames(st []string) bool {
	outer := slices.IndexFunc(st, func(f string) bool { return strings.HasSuffix(f, "::outer_perl") })
	leaf := slices.IndexFunc(st, func(f string) bool { return strings.HasSuffix(f, "::leaf_perl") })
	return outer >= 0 && leaf > outer
}

func stacks(p *profile.Profile) [][]string {
	res := make([][]string, 0, len(p.Sample))
	for _, s := range p.Sample {
		st := make([]string, 0, len(s.Location))
		for i := len(s.Location) - 1; i >= 0; i-- {
			name := ""
			if l := s.Location[i]; len(l.Line) > 0 {
				name = l.Line[0].Function.Name
			}
			st = append(st, name)
		}
		res = append(res, st)
	}
	return res
}

func hasPrefix(stack, prefix []string) bool {
	if len(stack) < len(prefix) {
		return false
	}
	for i, want := range prefix {
		if want != "*" && stack[i] != want {
			return false
		}
	}
	return true
}

func samplesSent(reg *prometheus.Registry, serviceName string) float64 {
	mfs, _ := reg.Gather()
	for _, mf := range mfs {
		if mf.GetName() != "pyroscope_ebpf_pprof_samples_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "service_name" && l.GetValue() == serviceName {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

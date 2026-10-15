//go:build linux && (amd64 || arm64)

package integration

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/prometheus/client_golang/prometheus"
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

// Reproduces https://github.com/grafana/opentelemetry-ebpf-profiler/issues/88.
func TestPyroscopeEbpfPerlIntegration(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("GITHUB_JOB") != "test_pyroscope" {
		t.Skip("Skipping Pyroscope eBPF integration test in GitHub Actions (job name is not test_pyroscope)")
	}
	if os.Geteuid() != 0 {
		t.Skip("pyroscope.ebpf needs root")
	}
	wg := sync.WaitGroup{}
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := testlog.TestLogger(t)
	_, pyroscopeEndpoint := pyroutil.StartPyroscopeContainer(t, ctx, l)

	perl := exec.CommandContext(ctx, "perl", "-e", perlScript)
	require.NoError(t, perl.Start())
	t.Cleanup(func() {
		_ = perl.Process.Kill()
		_ = perl.Wait()
	})

	reg := prometheus.NewRegistry()
	writeReceiver, writeComponent, err := testutil.CreateWriteComponent(l, reg, pyroscopeEndpoint)
	require.NoError(t, err)

	args := ebpf.NewDefaultArguments()
	args.ForwardTo = []pyroscope.Appendable{writeReceiver}
	args.CollectInterval = 5 * time.Second
	args.SymbCachePath = t.TempDir()
	args.Targets = []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{
			"__process_pid__": strconv.Itoa(perl.Process.Pid),
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

	// Root first; "*" matches exactly one frame (__libc_start_call_main, unsymbolized).
	expected := []string{"_start", "__libc_start_main", "*", "main", "perl_run", "Perl_runops_standard"}
	var seen [][]string
	require.Eventually(t, func() bool {
		res, err := test.Query(pyroscopeEndpoint, &querierv1.SelectMergeProfileRequest{
			ProfileTypeID: `process_cpu:cpu:nanoseconds:cpu:nanoseconds`,
			LabelSelector: `{service_name="ebpf-perl"}`,
			Start:         time.Now().Add(-time.Hour).UnixMilli(),
			End:           time.Now().UnixMilli(),
		})
		if err != nil {
			t.Logf("Error querying endpoint: %v", err)
			return false
		}
		seen = stacks(res)
		for _, st := range seen {
			if hasPrefix(st, expected) && len(st) > len(expected) {
				return true
			}
		}
		return false
	}, 90*time.Second, time.Second, "no perl stack with prefix %v (perl frames on top); got %d stacks: %v", expected, len(seen), seen)
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

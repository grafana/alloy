//go:build linux && (amd64 || arm64)

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/java"
	"github.com/grafana/alloy/internal/component/pyroscope/testutil"
	"github.com/grafana/alloy/internal/component/pyroscope/util/test"
	pyroutil "github.com/grafana/alloy/internal/component/pyroscope/util/test/container"
	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestPyroscopeJavaIntegration(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("GITHUB_JOB") != "test_pyroscope" {
		t.Skip("Skipping Pyroscope Java integration test in GitHub Actions (job name is not test_pyroscope)")
	}
	wg := sync.WaitGroup{}
	defer func() {
		wg.Wait()
	}()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	l := log.NewSyncLogger(log.NewLogfmtLogger(os.Stderr))
	l = log.WithPrefix(l,
		"test", t.Name(),
		"ts", log.DefaultTimestampUTC,
	)

	_, pyroscopeEndpoint := pyroutil.StartPyroscopeContainer(t, ctx, l)

	_, javaEndpoint, pid := pyroutil.StartJavaApplicationContainer(t, ctx, l)

	t.Logf("Pyroscope endpoint: %s", pyroscopeEndpoint)
	t.Logf("Java application endpoint: %s", javaEndpoint)
	t.Logf("Java process PID in container: %d", pid)

	reg := prometheus.NewRegistry()

	writeReceiver, writeComponent, err := testutil.
		CreateWriteComponent(l, reg, pyroscopeEndpoint)
	require.NoError(t, err, "Failed to create write component")

	args := java.DefaultArguments()
	args.ForwardTo = []pyroscope.Appendable{writeReceiver}
	args.ProfilingConfig.Interval = time.Second
	args.Targets = []discovery.Target{
		discovery.NewTargetFromMap(map[string]string{
			java.LabelProcessID: fmt.Sprintf("%d", pid),
			"service_name":      "spring-petclinic",
		}),
	}
	javaComponent, err := java.New(
		log.With(l, "component", "pyroscope.java"),
		reg,
		"test-java",
		args,
	)
	require.NoError(t, err, "Failed to create java component")

	wg.Add(3)
	go func() {
		defer wg.Done()
		_ = writeComponent.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		_ = javaComponent.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			burn(javaEndpoint)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	require.Eventually(t, func() bool {
		req := &querierv1.SelectMergeProfileRequest{
			ProfileTypeID: `process_cpu:cpu:nanoseconds:cpu:nanoseconds`,
			LabelSelector: `{service_name="spring-petclinic"}`,
			Start:         time.Now().Add(-time.Hour).UnixMilli(),
			End:           time.Now().UnixMilli(),
		}
		res, err := test.Query(pyroscopeEndpoint, req)
		if err != nil {
			t.Logf("Error querying endpoint: %v", err)
			return false
		}
		ss := res.String()
		if !strings.Contains(ss, `org/springframework/samples/petclinic/web/VetController.showVetList`) {
			return false
		}
		if !strings.Contains(ss, `libjvm.so.JavaThread::thread_main_inner`) {
			return false
		}
		return true
	}, 90*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		req := &querierv1.SelectMergeProfileRequest{
			ProfileTypeID: `memory:alloc_in_new_tlab_bytes:bytes:space:bytes`,
			LabelSelector: `{service_name="spring-petclinic"}`,
			Start:         time.Now().Add(-time.Hour).UnixMilli(),
			End:           time.Now().UnixMilli(),
		}
		res, err := test.Query(pyroscopeEndpoint, req)
		if err != nil {
			t.Logf("Error querying endpoint: %v", err)
			return false
		}
		ss := res.String()
		if !strings.Contains(ss, `org/springframework/samples/petclinic/web/VetController.showVetList`) {
			return false
		}
		if strings.Contains(ss, `libjvm.so.JavaThread::thread_main_inner`) {
			return false
		}
		return true
	}, 90*time.Second, 100*time.Millisecond)
	cancel()
}

func burn(url string) {
	_, _ = http.DefaultClient.Get(url + "/")
	_, _ = http.DefaultClient.Get(url + "/owners/find")
	_, _ = http.DefaultClient.Get(url + "/vets")
	_, _ = http.DefaultClient.Get(url + "/oups")
}

//go:build !race

package docker

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	dt "github.com/grafana/alloy/internal/component/loki/source/docker/internal/dockertarget"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const targetRestartInterval = 20 * time.Millisecond

func Test(t *testing.T) {
	// Use host that works on all platforms (including Windows).
	var cfg = `
		host       = "tcp://127.0.0.1:9375"
		targets    = []
		forward_to = []
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.source.docker")
	require.NoError(t, err)

	go func() {
		err := ctrl.Run(context.Background(), args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Minute))
}

func TestDuplicateTargets(t *testing.T) {
	// Use host that works on all platforms (including Windows).
	var cfg = `
		host       = "tcp://127.0.0.1:9376"
		targets    = [
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8080"},
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8081"},
		]
		forward_to = []
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "loki.source.docker")
	require.NoError(t, err)

	go func() {
		err := ctrl.Run(context.Background(), args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Minute))

	cmp, err := New(component.Options{
		ID:         "loki.source.docker.test",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		DataPath:   t.TempDir(),
	}, args)
	require.NoError(t, err)

	require.Len(t, cmp.manager.tasks, 1)
	require.Equal(t, cmp.manager.tasks[0].target.LabelsStr(), "{__meta_docker_container_id=\"foo\", __meta_docker_port_private=\"8080\"}")

	var newCfg = `
		host       = "tcp://127.0.0.1:9376"
		targets    = [
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8081"},
			{__meta_docker_container_id = "foo", __meta_docker_port_private = "8080"},
		]
		forward_to = []
	`
	err = syntax.Unmarshal([]byte(newCfg), &args)
	require.NoError(t, err)
	cmp.Update(args)
	require.Len(t, cmp.manager.tasks, 1)
	// Although the order of the targets changed, the filtered target stays the same.
	require.Equal(t, cmp.manager.tasks[0].target.LabelsStr(), "{__meta_docker_container_id=\"foo\", __meta_docker_port_private=\"8080\"}")
}

func TestRestart(t *testing.T) {
	runningState := true
	client := clientMock{
		logLine: "2024-05-02T13:11:55.879889Z caller=module_service.go:114 msg=\"module stopped\" module=distributor",
		running: func() bool { return runningState },
	}
	expectedLogLine := "caller=module_service.go:114 msg=\"module stopped\" module=distributor"

	tailer, entryHandler := setupTailer(t, client)
	go tailer.Run(context.Background())

	// The container is already running, expect log lines.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		logLines := entryHandler.Received()
		if assert.NotEmpty(c, logLines) {
			assert.Equal(c, expectedLogLine, logLines[0].Line)
		}
	}, time.Second, 20*time.Millisecond, "Expected log lines were not found within the time limit.")

	// Stops the container.
	runningState = false
	time.Sleep(targetRestartInterval + 10*time.Millisecond) // Sleep for a duration greater than targetRestartInterval to make sure it stops sending log lines.
	entryHandler.Clear()
	time.Sleep(targetRestartInterval + 10*time.Millisecond)
	assert.Empty(t, entryHandler.Received()) // No log lines because the container was not running.

	// Restart the container and expect log lines.
	runningState = true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		logLines := entryHandler.Received()
		if assert.NotEmpty(c, logLines) {
			assert.Equal(c, expectedLogLine, logLines[0].Line)
		}
	}, time.Second, 20*time.Millisecond, "Expected log lines were not found within the time limit after restart.")
}

func TestTargetNeverStarted(t *testing.T) {
	runningState := false
	client := clientMock{
		logLine: "2024-05-02T13:11:55.879889Z caller=module_service.go:114 msg=\"module stopped\" module=distributor",
		running: func() bool { return runningState },
	}

	tailer, _ := setupTailer(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	go tailer.Run(ctx)

	time.Sleep(20 * time.Millisecond)

	require.NotPanics(t, func() { cancel() })
}

func setupTailer(t *testing.T, client clientMock) (tailer *tailer, entryHandler *fake.Client) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	entryHandler = fake.NewClient(func() {})

	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	require.NoError(t, err)

	tgt, err := dt.NewTarget(
		dt.NewMetrics(prometheus.NewRegistry()),
		logger,
		entryHandler,
		ps,
		"flog",
		model.LabelSet{"job": "docker"},
		[]*relabel.Config{},
		client,
	)
	require.NoError(t, err)
	tailerTask := &tailerTask{
		options: &options{
			client:                client,
			targetRestartInterval: targetRestartInterval,
		},
		target: tgt,
	}
	return newTailer(logger, tailerTask), entryHandler
}

type clientMock struct {
	client.APIClient
	logLine string
	running func() bool
}

func (mock clientMock) ContainerInspect(ctx context.Context, c string) (types.ContainerJSON, error) {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: c,
			State: &types.ContainerState{
				Running: mock.running(),
			},
		},
		Config: &container.Config{Tty: true},
	}, nil
}

func (mock clientMock) ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(mock.logLine)), nil
}

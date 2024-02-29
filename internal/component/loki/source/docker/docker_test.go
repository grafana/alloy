//go:build !race

package docker

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/flow/componenttest"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/river"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	// Use host that works on all platforms (including Windows).
	var cfg = `
		host       = "tcp://127.0.0.1:9375"
		targets    = []
		forward_to = []
	`

	var args Arguments
	err := river.Unmarshal([]byte(cfg), &args)
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
	err := river.Unmarshal([]byte(cfg), &args)
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
		Logger:     util.TestFlowLogger(t),
		Registerer: prometheus.NewRegistry(),
		DataPath:   t.TempDir(),
	}, args)
	require.NoError(t, err)

	require.Len(t, cmp.manager.tasks, 1)
}

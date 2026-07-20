package docker

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestComponent(t *testing.T) {
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
		err := ctrl.Run(t.Context(), args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Minute))
}

func TestComponentDuplicateTargets(t *testing.T) {
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
		err := ctrl.Run(t.Context(), args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Minute))

	cmp, err := New(component.Options{
		ID:         "loki.source.docker.test",
		Logger:     logging.NewSlogNop(),
		Registerer: prometheus.NewRegistry(),
		DataPath:   t.TempDir(),
	}, args)
	require.NoError(t, err)

	require.Equal(t, 1, cmp.scheduler.Len())
	for s := range cmp.scheduler.Sources() {
		ss := s.(*tailer)
		require.Equal(t, "{__meta_docker_container_id=\"foo\", __meta_docker_port_private=\"8080\"}", ss.labelsStr)
	}

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
	// Although the order of the targets changed, the filtered target stays the same.
	require.Equal(t, 1, cmp.scheduler.Len())
	for s := range cmp.scheduler.Sources() {
		ss := s.(*tailer)
		require.Equal(t, "{__meta_docker_container_id=\"foo\", __meta_docker_port_private=\"8080\"}", ss.labelsStr)
	}
}

func TestComponentUpdatesLabelsOfRunningTailer(t *testing.T) {
	// Same container ID throughout: the component must apply label changes to
	// the tailer it is already running instead of ignoring them until restart.
	cfg := func(env, service string) Arguments {
		var args Arguments
		err := syntax.Unmarshal([]byte(`
			host       = "tcp://127.0.0.1:9381"
			targets    = [
				{__meta_docker_container_id = "abc123", service = "`+service+`"},
			]
			labels     = {"env" = "`+env+`"}
			forward_to = []
		`), &args)
		require.NoError(t, err)
		return args
	}

	cmp, err := New(component.Options{
		ID:         "loki.source.docker.test",
		Logger:     logging.NewSlogNop(),
		Registerer: prometheus.NewRegistry(),
		DataPath:   t.TempDir(),
	}, cfg("staging", "api"))
	require.NoError(t, err)

	streamLabelsOf := func() model.LabelSet {
		for s := range cmp.scheduler.Sources() {
			return s.(*tailer).curStreamLabels.Load().stdout
		}
		return nil
	}

	require.Equal(t, model.LabelValue("staging"), streamLabelsOf()["env"])
	require.Equal(t, model.LabelValue("api"), streamLabelsOf()["service"])

	// Change both the component's own `labels` argument and a target label
	// arriving from discovery. Neither changes the container ID.
	require.NoError(t, cmp.Update(cfg("production", "gateway")))

	require.Equal(t, 1, cmp.scheduler.Len(), "the tailer must not be restarted")
	require.Equal(t, model.LabelValue("production"), streamLabelsOf()["env"],
		"a change to the `labels` argument must reach the running tailer")
	require.Equal(t, model.LabelValue("gateway"), streamLabelsOf()["service"],
		"a changed target label must reach the running tailer")
}

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
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
	const expectedLabels = `{__meta_docker_container_id="foo", __meta_docker_port_private="8080"}`

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
		require.Equal(t, expectedLabels, ss.positionsLabels)
		require.Equal(t, expectedLabels, ss.labels.String())
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
		require.Equal(t, expectedLabels, ss.positionsLabels)
		require.Equal(t, expectedLabels, ss.labels.String())
	}
}

func TestUpdate(t *testing.T) {
	server := newStreamingDockerServer(t)
	receiver := loki.NewLogsReceiver(loki.WithChannel(make(chan loki.Entry, 2)))
	cfg := func(env, service, release string) Arguments {
		var args Arguments
		err := syntax.Unmarshal([]byte(fmt.Sprintf(`
			host       = %q
			targets    = [
				{__meta_docker_container_id = "abc123", service = "`+service+`"},
			]
			labels     = {"env" = "`+env+`"}
			forward_to = []
		`, server.URL)), &args)
		require.NoError(t, err)
		args.ForwardTo = []loki.LogsReceiver{receiver}
		rule := alloy_relabel.DefaultRelabelConfig
		rule.TargetLabel = "release"
		rule.Replacement = release
		args.RelabelRules = alloy_relabel.Rules{&rule}
		return args
	}

	cmp, err := New(component.Options{
		ID:         "loki.source.docker.test",
		Logger:     logging.NewSlogNop(),
		Registerer: prometheus.NewRegistry(),
		DataPath:   t.TempDir(),
	}, cfg("staging", "api", "canary"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	runDone := make(chan error, 1)
	go func() { runDone <- cmp.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-runDone)
	})

	server.Send("2024-01-01T00:00:00.000000000Z before\n")
	before := receiveDockerEntry(t, receiver)
	require.Equal(t, "before", before.Line)
	beforeLabels, err := json.Marshal(before.Labels)
	require.NoError(t, err)
	require.JSONEq(t, `{"env":"staging","service":"api","release":"canary"}`, string(beforeLabels))

	require.NoError(t, cmp.Update(cfg("production", "gateway", "stable")))

	server.Send("2024-01-01T00:00:01.000000000Z after\n")
	after := receiveDockerEntry(t, receiver)
	require.Equal(t, "after", after.Line)
	afterLabels, err := json.Marshal(after.Labels)
	require.NoError(t, err)
	require.JSONEq(t, `{"env":"production","service":"gateway","release":"stable"}`, string(afterLabels))

	retainedLabels, err := json.Marshal(before.Labels)
	require.NoError(t, err)
	require.JSONEq(t, `{"env":"staging","service":"api","release":"canary"}`, string(retainedLabels))
	require.Equal(t, int32(1), server.logRequests.Load(), "updating labels must not restart the Docker log stream")
}

type streamingDockerServer struct {
	*httptest.Server
	lines       chan string
	logRequests atomic.Int32
}

func newStreamingDockerServer(t *testing.T) *streamingDockerServer {
	t.Helper()
	server := &streamingDockerServer{lines: make(chan string, 2)}
	server.Server = httptest.NewServer(http.HandlerFunc(server.serveHTTP))
	t.Cleanup(server.Close)
	return server
}

func (s *streamingDockerServer) Send(line string) {
	s.lines <- line
}

func (s *streamingDockerServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/json"):
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(container.InspectResponse{
			ID:     "abc123",
			State:  &container.State{Running: true},
			Config: &container.Config{Tty: true},
		})

	case strings.HasSuffix(r.URL.Path, "/logs"):
		s.logRequests.Add(1)
		flusher, _ := w.(http.Flusher)
		for {
			select {
			case line := <-s.lines:
				_, _ = fmt.Fprint(w, line)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}

	default:
		http.NotFound(w, r)
	}
}

func receiveDockerEntry(t *testing.T, receiver loki.LogsReceiver) loki.Entry {
	t.Helper()
	select {
	case entry := <-receiver.Chan():
		return entry
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Docker log entry")
		return loki.Entry{}
	}
}

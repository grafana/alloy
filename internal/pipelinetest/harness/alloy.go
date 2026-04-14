package harness

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	alloyruntime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

type Options struct {
	Config          string
	LogsEntryPoints []string
}

// NewAlloy creates and starts an in-process Alloy runtime for pipeline tests.
// It injects the pipelinetest source and sink components into the provided
// config and wires the source to the configured entry points.
func NewAlloy(t *testing.T, opts Options) *Alloy {
	t.Helper()

	injectedComponents := func(opts Options) string {
		return `
			pipelinetest.source "in" {
				forward_to {
					logs = [` + strings.Join(opts.LogsEntryPoints, ", ") + `]
				}
			}

			pipelinetest.sink "out" {}
		`
	}

	require.NotEmpty(t, opts.LogsEntryPoints, "LogsEntryPoints must not be empty")

	logger, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	ctrl, err := alloyruntime.New(alloyruntime.Options{
		Logger:       logger,
		DataPath:     t.TempDir(),
		MinStability: featuregate.StabilityExperimental,
		Services:     defaultServices(logger),
	})
	require.NoError(t, err)

	ctx, cancel := t.Context(), func() {}
	ctx, cancel = context.WithCancel(ctx)

	a := &Alloy{
		t:      t,
		ctrl:   ctrl,
		cancel: cancel,
	}

	a.wg.Go(func() {
		ctrl.Run(ctx)
	})

	source, err := alloyruntime.ParseSource(t.Name(), []byte(injectedComponents(opts)+"\n"+opts.Config))
	require.NoError(t, err)

	err = ctrl.LoadSource(source, nil, "")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return ctrl.LoadComplete()
	}, 2*time.Second, 50*time.Millisecond)

	a.sink = mustComponent[*Sink](t, a, "pipelinetest.sink.out")
	a.source = mustComponent[*Source](t, a, "pipelinetest.source.in")

	t.Cleanup(func() {
		a.cancel()
		a.wg.Wait()
	})

	return a
}

type Alloy struct {
	t      *testing.T
	ctrl   *alloyruntime.Runtime
	cancel func()
	wg     sync.WaitGroup

	source *Source
	sink   *Sink
}

func (a *Alloy) SendEntries(entries ...loki.Entry) {
	for _, e := range entries {
		a.source.lokiFanout.Send(context.Background(), e)
	}
}

// Assert evaluates the provided assertions against the current snapshot
// until they all pass or the assertion timeout is reached.
func (a *Alloy) Assert(assertions ...Assertion) {
	a.t.Helper()

	require.EventuallyWithT(a.t, func(c *assert.CollectT) {
		s := a.sink.snapshot()
		for _, assertion := range assertions {
			require.NoError(c, assertion(s))
		}
	}, time.Second, 50*time.Millisecond)
}

func mustComponent[T any](t *testing.T, a *Alloy, id string) T {
	t.Helper()

	info, err := a.ctrl.GetComponent(component.ParseID(id), component.InfoOptions{})
	require.NoError(t, err)

	typed, ok := info.Component.(T)
	require.Truef(t, ok, "component %q has type %T, want %T", id, info.Component, *new(T))
	return typed
}

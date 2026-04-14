package harness

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloyruntime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

const injected = `pipelinetest.sink "out" {}`

// NewAlloy creates and starts an in-process Alloy runtime for pipeline tests.
// It adds the pipelinetest sink component to cfg so tests can assert on
// the resulting output while the rest of the pipeline is defined by cfg.
func NewAlloy(t *testing.T, cfg string) *Alloy {
	t.Helper()

	logger, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	ctrl, err := alloyruntime.New(alloyruntime.Options{
		Logger:       logger,
		DataPath:     t.TempDir(),
		MinStability: featuregate.StabilityExperimental,
		Services:     defaultServices(logger),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	a := &Alloy{
		t:      t,
		ctrl:   ctrl,
		cancel: cancel,
	}

	a.wg.Go(func() {
		ctrl.Run(ctx)
	})

	source, err := alloyruntime.ParseSource(t.Name(), []byte(injected+"\n"+cfg))
	require.NoError(t, err)

	err = ctrl.LoadSource(source, nil, "")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return ctrl.LoadComplete()
	}, 2*time.Second, 50*time.Millisecond)

	a.sink = MustComponent[*Sink](t, a, "pipelinetest.sink.out")

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

	sink *Sink
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

func MustComponent[T any](t *testing.T, a *Alloy, id string) T {
	t.Helper()

	info, err := a.ctrl.GetComponent(component.ParseID(id), component.InfoOptions{})
	require.NoError(t, err)

	typed, ok := info.Component.(T)
	require.Truef(t, ok, "component %q has type %T, want %T", id, info.Component, *new(T))
	return typed
}

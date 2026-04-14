package harness

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloyruntime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"
)

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

	source, err := alloyruntime.ParseSource(t.Name(), []byte(cfg))
	require.NoError(t, err)

	err = ctrl.LoadSource(source, nil, "")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return ctrl.LoadComplete()
	}, 2*time.Second, 50*time.Millisecond)

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
}

func (a *Alloy) Component(id string) (component.Component, error) {
	info, err := a.ctrl.GetComponent(component.ParseID(id), component.InfoOptions{})
	if err != nil {
		return nil, err
	}
	if info == nil || info.Component == nil {
		return nil, fmt.Errorf("component %q is not available", id)
	}
	return info.Component, nil
}

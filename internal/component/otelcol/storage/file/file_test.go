package file_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/storage/file"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	extstorage "go.opentelemetry.io/collector/extension/xextension/storage"
)

func TestExtension(t *testing.T) {
	ctx := componenttest.TestContext(t)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	ctrl := newTestComponent(t, ctx)

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	startedComponent, err := ctrl.GetComponent()
	require.NoError(t, err, "no component added in controller.")

	err = waitHealthy(ctx, startedComponent.(component.HealthComponent), time.Second)
	require.NoError(t, err, "timed out waiting for the component to be healthy")

	// Confirm the exports are correct.
	exports := ctrl.Exports().(extension.Exports)
	require.NotNil(t, exports.Handler)

	// Confirm the extension satisfies the interface.
	ext, ok := exports.Handler.Extension.(extstorage.Extension)
	require.True(t, ok, "extension is not of type extstorage.Extension")
	require.NotNil(t, ext)

	// Do a basic test of the extension client.
	cl, err := ext.GetClient(ctx, otelcomponent.KindReceiver, otelcomponent.MustNewID("test"), "")
	require.NoError(t, err)
	require.NotNil(t, cl)

	b, err := cl.Get(ctx, "test")
	require.NoError(t, err)
	require.Nil(t, b)

	err = cl.Set(ctx, "test", []byte("test"))
	require.NoError(t, err)

	b, err = cl.Get(ctx, "test")
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, "test", string(b))

	require.NoError(t, cl.Close(ctx), "failed to close client")
}

// newTestComponent brings up and runs the test component.
func newTestComponent(t *testing.T, ctx context.Context) *componenttest.Controller {
	t.Helper()
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.storage.file")
	require.NoError(t, err)

	args := file.Arguments{}
	args.SetToDefault()

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	return ctrl
}

func waitHealthy(ctx context.Context, c component.HealthComponent, timeout time.Duration) error {
	healthChannel := make(chan bool)

	go func() {
		for {
			healthz := c.CurrentHealth().Health
			if healthz == component.HealthTypeHealthy {
				healthChannel <- true
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(timeout):
				return
			default:
			}
		}
	}()

	// Wait for channel to be written to or timeout to occur.
	select {
	case <-healthChannel:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context timed out")
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for the component to be healthy")
	}
}

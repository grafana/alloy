package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/auth"
	"github.com/grafana/agent/internal/flow/componenttest"
	"github.com/grafana/agent/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func TestAuth(t *testing.T) {
	var (
		waitCreated = util.NewWaitTrigger()
		onCreated   = func() {
			waitCreated.Trigger()
		}
	)

	// Create and start our Flow component. We then wait for it to export a
	// consumer that we can send data to.
	te := newTestEnvironment(t, onCreated)
	te.Start(fakeAuthArgs{})

	require.NoError(t, waitCreated.Wait(time.Second), "extension never created")
}

type testEnvironment struct {
	t *testing.T

	Controller *componenttest.Controller
}

func newTestEnvironment(t *testing.T, onCreated func()) *testEnvironment {
	t.Helper()

	reg := component.Registration{
		Name:    "testcomponent",
		Args:    fakeAuthArgs{},
		Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := otelextension.NewFactory(
				"testcomponent",
				func() otelcomponent.Config { return nil },
				func(
					_ context.Context,
					_ otelextension.CreateSettings,
					_ otelcomponent.Config,
				) (otelcomponent.Component, error) {

					onCreated()
					return nil, nil
				}, otelcomponent.StabilityLevelUndefined,
			)

			return auth.New(opts, factory, args.(auth.Arguments))
		},
	}

	return &testEnvironment{
		t:          t,
		Controller: componenttest.NewControllerFromReg(util.TestLogger(t), reg),
	}
}

func (te *testEnvironment) Start(args component.Arguments) {
	go func() {
		ctx := componenttest.TestContext(te.t)
		err := te.Controller.Run(ctx, args)
		require.NoError(te.t, err, "failed to run component")
	}()
}

type fakeAuthArgs struct {
}

var _ auth.Arguments = fakeAuthArgs{}

func (fa fakeAuthArgs) Convert() (otelcomponent.Config, error) {
	return &struct{}{}, nil
}

func (fa fakeAuthArgs) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

func (fa fakeAuthArgs) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

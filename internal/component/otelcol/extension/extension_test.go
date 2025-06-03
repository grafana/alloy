package extension_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
)

func TestExtension(t *testing.T) {
	var (
		waitCreated = util.NewWaitTrigger()
		onCreated   = func() {
			waitCreated.Trigger()
		}
	)

	// Create and start our Alloy component. We then wait for it to export a
	// consumer that we can send data to.
	te := newTestEnvironment(t, onCreated)
	te.Start(fakeExtensionArgs{})

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
		Args:    fakeExtensionArgs{},
		Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := otelextension.NewFactory(
				otelcomponent.MustNewType("testcomponent"),
				func() otelcomponent.Config { return nil },
				func(
					_ context.Context,
					_ otelextension.Settings,
					_ otelcomponent.Config,
				) (otelextension.Extension, error) {

					onCreated()
					return nil, nil
				}, otelcomponent.StabilityLevelUndefined,
			)

			return extension.New(opts, factory, args.(extension.Arguments))
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

type fakeExtensionArgs struct {
}

var _ extension.Arguments = fakeExtensionArgs{}

func (fakeExtensionArgs) ExportsHandler() bool {
	return false
}

func (fa fakeExtensionArgs) Convert(_ component.Options) (otelcomponent.Config, error) {
	return &struct{}{}, nil
}

func (fa fakeExtensionArgs) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (fa fakeExtensionArgs) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (fa fakeExtensionArgs) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	var dma otelcolCfg.DebugMetricsArguments
	dma.SetToDefault()
	return dma
}

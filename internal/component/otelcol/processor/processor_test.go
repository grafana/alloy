package processor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	otelprocessor "go.opentelemetry.io/collector/processor"
)

func TestProcessor(t *testing.T) {
	ctx := componenttest.TestContext(t)

	// Create an instance of a fake OpenTelemetry Collector processor which our
	// Alloy component will wrap around. Our fake processor will immediately
	// forward data to the connected consumer once one is made available to it.
	var (
		consumer otelconsumer.Traces

		waitConsumerTrigger = util.NewWaitTrigger()
		onTracesConsumer    = func(t otelconsumer.Traces) {
			consumer = t
			waitConsumerTrigger.Trigger()
		}

		waitTracesTrigger = util.NewWaitTrigger()
		nextConsumer      = &fakeconsumer.Consumer{
			ConsumeTracesFunc: func(context.Context, ptrace.Traces) error {
				waitTracesTrigger.Trigger()
				return nil
			},
		}

		// Our fake processor will wait for a consumer to be registered and then
		// pass along data directly to it.
		innerProcessor = &fakeProcessor{
			ConsumeTracesFunc: func(ctx context.Context, td ptrace.Traces) error {
				require.NoError(t, waitConsumerTrigger.Wait(time.Second), "no next consumer registered")
				return consumer.ConsumeTraces(ctx, td)
			},
		}
	)

	// Create and start our Alloy component. We then wait for it to export a
	// consumer that we can send data to.
	te := newTestEnvironment(t, innerProcessor, onTracesConsumer)
	te.Start(fakeProcessorArgs{
		Output: &otelcol.ConsumerArguments{
			Metrics: []otelcol.Consumer{nextConsumer},
			Logs:    []otelcol.Consumer{nextConsumer},
			Traces:  []otelcol.Consumer{nextConsumer},
		},
	})

	require.NoError(t, te.Controller.WaitExports(1*time.Second), "test component did not generate exports")
	ce := te.Controller.Exports().(otelcol.ConsumerExports)

	// Create a test set of traces and send it to our consumer in the background.
	// We then wait for our channel to receive the traces, indicating that
	// everything was wired up correctly.
	go func() {
		var err error

		for {
			err = ce.Input.ConsumeTraces(ctx, ptrace.NewTraces())

			if errors.Is(err, pipeline.ErrSignalNotSupported) {
				// Our component may not have been fully initialized yet. Wait a little
				// bit before trying again.
				time.Sleep(100 * time.Millisecond)
				continue
			}

			require.NoError(t, err)
			break
		}
	}()

	require.NoError(t, waitTracesTrigger.Wait(time.Second), "consumer did not get invoked")
}

type testEnvironment struct {
	t *testing.T

	Controller *componenttest.Controller
}

func newTestEnvironment(
	t *testing.T,
	fp otelprocessor.Traces,
	onTracesConsumer func(t otelconsumer.Traces),
) *testEnvironment {

	t.Helper()

	reg := component.Registration{
		Name:    "testcomponent",
		Args:    fakeProcessorArgs{},
		Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			// Create a factory which always returns our instance of fakeProcessor
			// defined above.
			factory := otelprocessor.NewFactory(
				otelcomponent.MustNewType("testcomponent"),
				func() otelcomponent.Config {
					res, err := fakeProcessorArgs{}.Convert()
					require.NoError(t, err)
					return res
				},
				otelprocessor.WithTraces(func(
					_ context.Context,
					_ otelprocessor.Settings,
					_ otelcomponent.Config,
					t otelconsumer.Traces,
				) (otelprocessor.Traces, error) {

					onTracesConsumer(t)
					return fp, nil
				}, otelcomponent.StabilityLevelUndefined),
			)

			return processor.New(opts, factory, args.(processor.Arguments))
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

type fakeProcessorArgs struct {
	Output *otelcol.ConsumerArguments
}

var _ processor.Arguments = fakeProcessorArgs{}

func (fa fakeProcessorArgs) Convert() (otelcomponent.Config, error) {
	return &struct{}{}, nil
}

func (fa fakeProcessorArgs) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (fa fakeProcessorArgs) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (fa fakeProcessorArgs) NextConsumers() *otelcol.ConsumerArguments {
	return fa.Output
}

func (fa fakeProcessorArgs) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	var dma otelcolCfg.DebugMetricsArguments
	dma.SetToDefault()
	return dma
}

type fakeProcessor struct {
	StartFunc         func(ctx context.Context, host otelcomponent.Host) error
	ShutdownFunc      func(ctx context.Context) error
	CapabilitiesFunc  func() otelconsumer.Capabilities
	ConsumeTracesFunc func(ctx context.Context, td ptrace.Traces) error
}

var _ otelprocessor.Traces = (*fakeProcessor)(nil)

func (fe *fakeProcessor) Start(ctx context.Context, host otelcomponent.Host) error {
	if fe.StartFunc != nil {
		return fe.StartFunc(ctx, host)
	}
	return nil
}

func (fe *fakeProcessor) Shutdown(ctx context.Context) error {
	if fe.ShutdownFunc != nil {
		return fe.ShutdownFunc(ctx)
	}
	return nil
}

func (fe *fakeProcessor) Capabilities() otelconsumer.Capabilities {
	if fe.CapabilitiesFunc != nil {
		return fe.CapabilitiesFunc()
	}
	return otelconsumer.Capabilities{}
}

func (fe *fakeProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if fe.ConsumeTracesFunc != nil {
		return fe.ConsumeTracesFunc(ctx, td)
	}
	return nil
}

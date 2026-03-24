package receiver_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	otelreceiver "go.opentelemetry.io/collector/receiver"
)

func TestReceiver(t *testing.T) {
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
	)

	// Create and start our Alloy component. We then wait for it to export a
	// consumer that we can send data to.
	te := newTestEnvironment(t, onTracesConsumer)
	te.Start(fakeReceiverArgs{
		Output: &otelcol.ConsumerArguments{
			Metrics: []otelcol.Consumer{nextConsumer},
			Logs:    []otelcol.Consumer{nextConsumer},
			Traces:  []otelcol.Consumer{nextConsumer},
		},
	})

	require.NoError(t, waitConsumerTrigger.Wait(time.Second), "no traces consumer sent")

	err := consumer.ConsumeTraces(t.Context(), ptrace.NewTraces())
	require.NoError(t, err)

	require.NoError(t, waitTracesTrigger.Wait(time.Second), "consumer did not get invoked")
}

func TestReceiverNotStarted(t *testing.T) {
	var (
		waitConsumerTrigger = util.NewWaitTrigger()
		onTracesConsumer    = func(t otelconsumer.Traces) {
			waitConsumerTrigger.Trigger()
		}
	)
	te := newTestEnvironment(t, onTracesConsumer)
	te.Start(fakeReceiverArgs{
		Output: &otelcol.ConsumerArguments{},
	})

	// Check that no trace receiver was started because it's not needed by the output.
	require.ErrorContains(t, waitConsumerTrigger.Wait(time.Second), "context deadline exceeded")
}

func TestReceiverUpdate(t *testing.T) {
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
	)

	te := newTestEnvironment(t, onTracesConsumer)
	te.Start(fakeReceiverArgs{
		Output: &otelcol.ConsumerArguments{},
	})

	// Check that no trace receiver was started because it's not needed by the output.
	require.ErrorContains(t, waitConsumerTrigger.Wait(time.Second), "context deadline exceeded")

	te.Controller.Update(fakeReceiverArgs{
		Output: &otelcol.ConsumerArguments{
			Traces: []otelcol.Consumer{nextConsumer},
		},
	})

	// Now the trace receiver is started.
	require.NoError(t, waitConsumerTrigger.Wait(time.Second), "no traces consumer sent")

	err := consumer.ConsumeTraces(t.Context(), ptrace.NewTraces())
	require.NoError(t, err)

	require.NoError(t, waitTracesTrigger.Wait(time.Second), "consumer did not get invoked")

	waitConsumerTrigger = util.NewWaitTrigger()

	// Remove the trace receiver.
	te.Controller.Update(fakeReceiverArgs{
		Output: &otelcol.ConsumerArguments{},
	})

	// Check that after the update no trace receiver is started.
	require.ErrorContains(t, waitConsumerTrigger.Wait(time.Second), "context deadline exceeded")
}

type testEnvironment struct {
	t *testing.T

	Controller *componenttest.Controller
}

func newTestEnvironment(t *testing.T, onTracesConsumer func(t otelconsumer.Traces)) *testEnvironment {
	t.Helper()

	reg := component.Registration{
		Name:    "testcomponent",
		Args:    fakeReceiverArgs{},
		Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			// Create a factory which always returns our instance of fakeReceiver
			// defined above.
			factory := otelreceiver.NewFactory(
				otelcomponent.MustNewType("testcomponent"),
				func() otelcomponent.Config { return nil },
				otelreceiver.WithTraces(func(
					_ context.Context,
					_ otelreceiver.Settings,
					_ otelcomponent.Config,
					t otelconsumer.Traces,
				) (otelreceiver.Traces, error) {

					onTracesConsumer(t)
					return nil, nil
				}, otelcomponent.StabilityLevelUndefined),
			)

			return receiver.New(opts, factory, args.(receiver.Arguments))
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

type fakeReceiverArgs struct {
	Output *otelcol.ConsumerArguments
}

var _ receiver.Arguments = fakeReceiverArgs{}

func (fa fakeReceiverArgs) Convert() (otelcomponent.Config, error) {
	return &struct{}{}, nil
}

func (fa fakeReceiverArgs) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (fa fakeReceiverArgs) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (fa fakeReceiverArgs) NextConsumers() *otelcol.ConsumerArguments {
	return fa.Output
}

func (fa fakeReceiverArgs) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	var args otelcolCfg.DebugMetricsArguments
	args.SetToDefault()
	return args
}

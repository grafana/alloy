package processor_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	otelprocessor "go.opentelemetry.io/collector/processor"
	"go.uber.org/atomic"
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

// TestProcessor_StabilityValidation checks that Update rejects Arguments
// that implement otelcol.StabilityValidator and require a higher stability
// level than the component was configured with. New calls Update
// internally, so this covers both initial build and later config changes.
func TestProcessor_StabilityValidation(t *testing.T) {
	ctx := componenttest.TestContext(t)

	reg := component.Registration{
		Name:    "testcomponent",
		Args:    fakeStabilityArgs{},
		Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := otelprocessor.NewFactory(
				otelcomponent.MustNewType("testcomponent"),
				func() otelcomponent.Config { return &struct{}{} },
				otelprocessor.WithTraces(func(
					_ context.Context,
					_ otelprocessor.Settings,
					_ otelcomponent.Config,
					_ otelconsumer.Traces,
				) (otelprocessor.Traces, error) {

					return &fakeProcessor{}, nil
				}, otelcomponent.StabilityLevelUndefined),
			)
			return processor.New(opts, factory, args.(processor.Arguments))
		},
	}

	atGA := func(opts component.Options) component.Options {
		opts.MinStability = featuregate.StabilityGenerallyAvailable
		return opts
	}
	atExperimental := func(opts component.Options) component.Options {
		opts.MinStability = featuregate.StabilityExperimental
		return opts
	}

	args := fakeStabilityArgs{
		fakeProcessorArgs:   fakeProcessorArgs{Output: &otelcol.ConsumerArguments{}},
		RequireExperimental: true,
	}

	t.Run("rejected below the required stability level", func(t *testing.T) {
		controller := componenttest.NewControllerFromReg(util.TestLogger(t), reg)
		err := controller.Run(ctx, args, atGA)
		require.ErrorContains(t, err, "experimental")
	})

	t.Run("allowed at the required stability level", func(t *testing.T) {
		controller := componenttest.NewControllerFromReg(util.TestLogger(t), reg)
		go func() {
			_ = controller.Run(ctx, args, atExperimental)
		}()
		require.NoError(t, controller.WaitRunning(time.Second))
	})
}

// TestProcessor_UpdateDoesNotWaitOnDataPath models a config reload that adds a
// new component downstream of a running processor and also updates the
// processor. The new component is paused until every Update of the reload has
// returned. If Update waits for data that is blocked on it, the reload never
// finishes. See https://github.com/grafana/alloy/issues/7762.
func TestProcessor_UpdateDoesNotWaitOnDataPath(t *testing.T) {
	const timeout = 5 * time.Second

	tests := []struct {
		name string
		// blockData makes data of the first processor instance block on the
		// paused downstream component. It runs before the processor starts, and
		// returns a function that runs after it starts.
		blockData func(t *testing.T, env *reloadTestEnvironment) func(ce otelcol.ConsumerExports)
	}{
		{
			// Update swaps the consumers, which must not wait for data that is in
			// flight through the processor.
			name: "data in flight",
			blockData: func(t *testing.T, env *reloadTestEnvironment) func(ce otelcol.ConsumerExports) {
				inFlight := util.NewWaitTrigger()
				env.onConsume = func(int) { inFlight.Trigger() }
				return func(ce otelcol.ConsumerExports) {
					go func() { _ = ce.Input.ConsumeTraces(componenttest.TestContext(t), ptrace.NewTraces()) }()
					require.NoError(t, inFlight.Wait(timeout), "data did not reach the processor")
				}
			},
		},
		{
			// Update stops the old instance, which flushes data while it shuts
			// down, as the batch and tail sampling processors do.
			name: "data flushed on shutdown",
			blockData: func(_ *testing.T, env *reloadTestEnvironment) func(ce otelcol.ConsumerExports) {
				env.onShutdown = func(ctx context.Context, generation int, next otelconsumer.Traces) error {
					if generation != 1 {
						return nil
					}
					return next.ConsumeTraces(ctx, ptrace.NewTraces())
				}
				return func(otelcol.ConsumerExports) {}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newReloadTestEnvironment(t)
			afterStart := tc.blockData(t, env)
			ce := env.start(t, timeout)
			afterStart(ce)

			updateDone := make(chan error, 1)
			go func() { updateDone <- env.controller.Update(env.args()) }()
			select {
			case err := <-updateDone:
				require.NoError(t, err)
			case <-time.After(timeout):
				require.FailNow(t, "Update did not return: it waits on the data path")
			}

			// The end of the reload: the new downstream component starts. The
			// blocked data is delivered and the new processor instance starts.
			env.downstream.Resume()
			require.Eventually(t, func() bool { return env.delivered.Load() == 1 },
				timeout, 10*time.Millisecond, "the blocked data was not delivered")
			require.Eventually(t, func() bool { return env.started.Load() == 2 },
				timeout, 10*time.Millisecond, "the new processor instance did not start")

			// New data goes through the new instance.
			require.NoError(t, ce.Input.ConsumeTraces(t.Context(), ptrace.NewTraces()))
			require.Equal(t, int32(1), env.consumedByGeneration2.Load())
		})
	}
}

// reloadTestEnvironment runs a processor whose output is a paused lazyconsumer.
// The lazyconsumer stands for a new component that a reload adds: the test
// resumes it to mark the end of the reload. Each Update of the processor
// creates a new instance of the fake processor, with an increasing generation
// number.
type reloadTestEnvironment struct {
	controller *componenttest.Controller
	downstream *lazyconsumer.Consumer

	// Optional hooks for the fake processor instances. Set them before start.
	onConsume  func(generation int)
	onShutdown func(ctx context.Context, generation int, next otelconsumer.Traces) error

	generation            atomic.Int32
	started               atomic.Int32 // Instances started.
	delivered             atomic.Int32 // Calls that reached the downstream component.
	consumedByGeneration2 atomic.Int32
}

func newReloadTestEnvironment(t *testing.T) *reloadTestEnvironment {
	env := &reloadTestEnvironment{}

	env.downstream = lazyconsumer.NewPaused(componenttest.TestContext(t), "downstream")
	env.downstream.SetConsumers(&fakeconsumer.Consumer{
		ConsumeTracesFunc: func(context.Context, ptrace.Traces) error {
			env.delivered.Inc()
			return nil
		},
	}, nil, nil)

	reg := component.Registration{
		Name:    "testcomponent",
		Args:    fakeProcessorArgs{},
		Exports: otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			factory := otelprocessor.NewFactory(
				otelcomponent.MustNewType("testcomponent"),
				func() otelcomponent.Config { return &struct{}{} },
				otelprocessor.WithTraces(func(
					_ context.Context,
					_ otelprocessor.Settings,
					_ otelcomponent.Config,
					next otelconsumer.Traces,
				) (otelprocessor.Traces, error) {

					return env.newInstance(int(env.generation.Inc()), next), nil
				}, otelcomponent.StabilityLevelUndefined),
			)
			return processor.New(opts, factory, args.(processor.Arguments))
		},
	}
	env.controller = componenttest.NewControllerFromReg(util.TestLogger(t), reg)
	return env
}

func (env *reloadTestEnvironment) args() fakeProcessorArgs {
	return fakeProcessorArgs{Output: &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{env.downstream}}}
}

// start runs the processor and waits for its first instance to start.
func (env *reloadTestEnvironment) start(t *testing.T, timeout time.Duration) otelcol.ConsumerExports {
	componenttest.RunInBackground(t, func(ctx context.Context) error {
		return env.controller.Run(ctx, env.args())
	})
	// Don't leave data blocked when a test fails. Cleanups run in reverse
	// order, so this runs before the processor is stopped.
	t.Cleanup(env.downstream.Resume)

	require.NoError(t, env.controller.WaitExports(timeout), "processor did not export its input")
	require.Eventually(t, func() bool { return env.started.Load() == 1 },
		timeout, 10*time.Millisecond, "the first processor instance did not start")
	return env.controller.Exports().(otelcol.ConsumerExports)
}

func (env *reloadTestEnvironment) newInstance(generation int, next otelconsumer.Traces) *fakeProcessor {
	return &fakeProcessor{
		StartFunc: func(context.Context, otelcomponent.Host) error {
			env.started.Inc()
			return nil
		},
		ShutdownFunc: func(ctx context.Context) error {
			if env.onShutdown != nil {
				return env.onShutdown(ctx, generation, next)
			}
			return nil
		},
		ConsumeTracesFunc: func(ctx context.Context, td ptrace.Traces) error {
			if generation == 2 {
				env.consumedByGeneration2.Inc()
			}
			if env.onConsume != nil {
				env.onConsume(generation)
			}
			return next.ConsumeTraces(ctx, td)
		},
	}
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

// fakeStabilityArgs adds a RequireExperimental toggle to fakeProcessorArgs so
// tests can exercise the otelcol.StabilityValidator hook in Processor.Update.
type fakeStabilityArgs struct {
	fakeProcessorArgs
	RequireExperimental bool
}

var _ otelcol.StabilityValidator = fakeStabilityArgs{}

func (fa fakeStabilityArgs) ValidateStabilityLevel(minStability featuregate.Stability) error {
	if fa.RequireExperimental && !minStability.Permits(featuregate.StabilityExperimental) {
		return fmt.Errorf("this feature is experimental and requires setting the stability.level flag to experimental")
	}
	return nil
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

package alloyengine

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/grafana/alloy/flowcmd"
	"github.com/spf13/cobra"
)

var _ extension.Extension = (*alloyEngineExtension)(nil)

type state int

var (
	stateNotStarted   state = 0
	stateRunning      state = 2
	stateShuttingDown state = 3
	stateTerminated   state = 4
	stateRunError     state = 5
)

func (s state) String() string {
	switch s {
	case stateNotStarted:
		return "not_started"
	case stateRunning:
		return "running"
	case stateShuttingDown:
		return "shutting_down"
	case stateTerminated:
		return "terminated"
	case stateRunError:
		return "run_error"
	}
	return fmt.Sprintf("unknown_state_%d", s)
}

// alloyEngineExtension implements the alloyengine extension.
type alloyEngineExtension struct {
	config            *Config
	settings          component.TelemetrySettings
	runExited         chan struct{}
	runCommandFactory func() *cobra.Command

	stateMutex sync.Mutex
	state      state
	runCancel  context.CancelFunc
}

// newAlloyEngineExtension creates a new alloyEngine extension instance.
func newAlloyEngineExtension(config *Config, settings component.TelemetrySettings) *alloyEngineExtension {
	return &alloyEngineExtension{
		config:            config,
		settings:          settings,
		state:             stateNotStarted,
		runCommandFactory: flowcmd.RunCommand,
	}
}

func (e *alloyEngineExtension) isUp() int64 {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	if e.state == stateRunning {
		return 1
	}
	return 0
}

// Start is called when the extension is started.
func (e *alloyEngineExtension) Start(ctx context.Context, host component.Host) error {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	switch e.state {
	case stateNotStarted:
		break
	default:
		return fmt.Errorf("cannot start alloyengine extension in current state: %s", e.state.String())
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	e.runCancel = runCancel
	e.runExited = make(chan struct{})

	runCommand := e.runCommandFactory()
	runCommand.SetArgs([]string{e.config.AlloyConfig.File})
	err := runCommand.ParseFlags(e.config.flagsAsSlice())
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if e.settings.MeterProvider != nil {
		meter := e.settings.MeterProvider.Meter("github.com/grafana/alloy/extension/alloyengine")
		upGauge, err := meter.Int64ObservableGauge(
			"alloyengine_up",
			metric.WithDescription("1 if the Alloy engine state is running, 0 otherwise"),
			metric.WithUnit("1"),
		)
		if err != nil {
			e.settings.Logger.Warn("failed to create alloyengine_up gauge", zap.Error(err))
		} else {
			_, err := meter.RegisterCallback(
				func(ctx context.Context, o metric.Observer) error {
					o.ObserveInt64(upGauge, e.isUp())
					return nil
				},
				upGauge,
			)
			if err != nil {
				e.settings.Logger.Warn("failed to register alloyengine_up callback", zap.Error(err))
			}
		}
	}

	go func() {
		defer close(e.runExited)

		err := e.runWithBackoffRetry(runCommand, runCtx)

		e.stateMutex.Lock()
		previousState := e.state
		e.state = stateTerminated
		e.stateMutex.Unlock()

		if err == nil {
			e.settings.Logger.Debug("run command exited successfully without error")
		} else if previousState == stateShuttingDown {
			e.settings.Logger.Warn("run command exited with an error during shutdown", zap.Error(err))
		}
	}()

	return nil
}

func (e *alloyEngineExtension) runWithBackoffRetry(runCommand *cobra.Command, ctx context.Context) error {
	var lastError error
	baseDelay := 1 * time.Second
	i := 1

	for {
		// TODO: how can we avoid this? we want to identify when running is true without having to wait for a response here
		// this way we're gonna have flapping states between running and run_error even if run always returns an error
		e.state = stateRunning
		e.settings.Logger.Info("alloyengine extension started successfully")
		err := runCommand.ExecuteContext(ctx)

		if err == nil {
			return nil
		}

		lastError = err
		e.stateMutex.Lock()
		e.state = stateRunError
		e.stateMutex.Unlock()

		// exponential backoff until 15s
		delay := 15 * time.Second
		if i < 4 {
			delay = time.Duration(math.Pow(2, float64(i))) * baseDelay
		}

		e.settings.Logger.Warn("run command failed, will retry", zap.Error(err), zap.Duration("retry_delay", delay), zap.Int("attempt", i))

		i++

		select {
		case <-time.After(delay):
			// continue to next iteration
		case <-ctx.Done():
			return lastError
		}
	}
}

// Shutdown is called when the extension is being stopped.
func (e *alloyEngineExtension) Shutdown(ctx context.Context) error {
	e.stateMutex.Lock()
	switch e.state {
	case stateRunning:
		e.settings.Logger.Info("alloyengine extension shutting down")
		e.state = stateShuttingDown
		// guaranteed to be non-nil since we are in stateRunning
		e.runCancel()
		// unlock so that the run goroutine can transition to terminated
		e.stateMutex.Unlock()

		select {
		case <-e.runExited:
			e.settings.Logger.Info("alloyengine extension shut down successfully")
		case <-ctx.Done():
			e.settings.Logger.Warn("alloyengine extension shutdown interrupted by context", zap.Error(ctx.Err()))
		}
		return nil
	case stateNotStarted:
		e.settings.Logger.Info("alloyengine extension shutdown completed (not started)")
		e.stateMutex.Unlock()
		return nil
	default:
		e.settings.Logger.Warn("alloyengine extension shutdown in current state is a no-op", zap.String("state", e.state.String()))
		e.stateMutex.Unlock()
		return nil
	}
}

// Ready returns nil when the extension is ready to process data.
func (e *alloyEngineExtension) Ready() error {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	switch e.state {
	case stateRunning:
		return nil
	default:
		return fmt.Errorf("alloyengine extension not ready in current state: %s", e.state.String())
	}
}

// NotReady returns an error when the extension is not ready to process data.
func (e *alloyEngineExtension) NotReady() error {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	switch e.state {
	case stateRunning:
		return nil
	default:
		return fmt.Errorf("alloyengine extension not ready in current state: %s", e.state.String())
	}
}

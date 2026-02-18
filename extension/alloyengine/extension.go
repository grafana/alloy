package alloyengine

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/grafana/alloy/flowcmd"
	"github.com/grafana/alloy/internal/readyctx"
	"github.com/spf13/cobra"
)

var _ extension.Extension = (*alloyEngineExtension)(nil)

type state int

var (
	stateNotStarted   state = 0
	stateStarting     state = 1
	stateRunning      state = 2
	stateShuttingDown state = 3
	stateTerminated   state = 4
	stateRunError     state = 5
)

func (e *alloyEngineExtension) setState(newState state) {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	oldState := e.state
	e.state = newState
	if oldState != newState {
		e.settings.Logger.Info("alloyengine extension state changed", zap.String("from", oldState.String()), zap.String("to", newState.String()))
	}
}

func (e *alloyEngineExtension) getState() state {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	return e.state
}

func (s state) String() string {
	switch s {
	case stateNotStarted:
		return "not_started"
	case stateStarting:
		return "starting"
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

func (e *alloyEngineExtension) Start(ctx context.Context, host component.Host) error {
	currentState := e.getState()
	switch currentState {
	case stateNotStarted:
		break
	default:
		return fmt.Errorf("cannot start alloyengine extension in current state: %s", currentState)
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	e.runCancel = runCancel
	e.runExited = make(chan struct{})

	runCtx = readyctx.WithOnReady(runCtx, func() {
		e.setState(stateRunning)
	})

	runCommand := e.runCommandFactory()
	runCommand.SetArgs([]string{e.config.AlloyConfig.File})
	err := runCommand.ParseFlags(e.config.flagsAsSlice())
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	e.setState(stateStarting)

	go func() {
		defer close(e.runExited)

		err := e.runWithBackoffRetry(runCommand, runCtx)

		previousState := currentState
		e.setState(stateTerminated)

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
		err := runCommand.ExecuteContext(ctx)

		if err == nil {
			return nil
		}

		lastError = err
		e.setState(stateRunError)

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
	currentState := e.getState()
	switch currentState {
	case stateStarting, stateRunning, stateRunError:
		e.settings.Logger.Info("alloyengine extension shutting down")
		e.setState(stateShuttingDown)
		// guaranteed to be non-nil since we are in stateRunning
		e.runCancel()

		select {
		case <-e.runExited:
			e.settings.Logger.Info("alloyengine extension shut down successfully")
		case <-ctx.Done():
			e.settings.Logger.Warn("alloyengine extension shutdown interrupted by context", zap.Error(ctx.Err()))
		}
		return nil
	case stateNotStarted:
		e.settings.Logger.Info("alloyengine extension shutdown completed (not started)")
		return nil
	default:
		e.settings.Logger.Warn("alloyengine extension shutdown in current state is a no-op", zap.String("state", e.state.String()))
		return nil
	}
}

// Ready returns nil when the extension is ready to process data.
func (e *alloyEngineExtension) Ready() error {
	currentState := e.getState()
	switch currentState {
	case stateStarting, stateRunning, stateRunError:
		return nil
	default:
		return fmt.Errorf("alloyengine extension not ready in current state: %s", currentState.String())
	}
}

// NotReady returns an error when the extension is not ready to process data.
func (e *alloyEngineExtension) NotReady() error {
	currentState := e.getState()
	switch currentState {
	case stateStarting, stateRunning, stateRunError:
		return nil
	default:
		return fmt.Errorf("alloyengine extension not ready in current state: %s", currentState.String())
	}
}

package alloyflow

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/grafana/alloy/flowcmd"
	"github.com/spf13/cobra"
)

var _ extension.Extension = (*alloyFlowExtension)(nil)

type state int

var (
	stateNotStarted   state = 0
	stateRunning      state = 1
	stateShuttingDown state = 2
	stateTerminated   state = 3
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
	}
	return fmt.Sprintf("unknown_state_%d", s)
}

// alloyFlowExtension implements the alloyflow extension.
type alloyFlowExtension struct {
	config            *Config
	settings          component.TelemetrySettings
	runExited         chan struct{}
	runCommandFactory func() *cobra.Command

	stateMutex sync.Mutex
	state      state
	runCancel  context.CancelFunc
}

// newAlloyFlowExtension creates a new alloyflow extension instance.
func newAlloyFlowExtension(config *Config, settings component.TelemetrySettings) *alloyFlowExtension {
	return &alloyFlowExtension{
		config:            config,
		settings:          settings,
		state:             stateNotStarted,
		runCommandFactory: flowcmd.RunCommand,
	}
}

// Start is called when the extension is started.
func (e *alloyFlowExtension) Start(ctx context.Context, host component.Host) error {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	switch e.state {
	case stateNotStarted:
		break
	default:
		return fmt.Errorf("cannot start alloyflow extension in current state: %s", e.state.String())
	}

	runCommand := e.runCommandFactory()
	runCommand.SetArgs([]string{e.config.ConfigPath})
	err := runCommand.ParseFlags(e.config.flagsAsSlice())
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	e.runCancel = runCancel
	e.runExited = make(chan struct{})

	go func() {
		defer close(e.runExited)
		err := runCommand.ExecuteContext(runCtx)

		e.stateMutex.Lock()
		previousState := e.state
		e.state = stateTerminated
		e.stateMutex.Unlock()

		if err == nil {
			e.settings.Logger.Debug("run command exited successfully without error")
		} else if previousState == stateShuttingDown {
			e.settings.Logger.Warn("run command exited with an error during shutdown", zap.Error(err))
		} else {
			e.settings.Logger.Error("run command exited unexpectedly with an error", zap.Error(err))
		}
	}()

	e.state = stateRunning
	e.settings.Logger.Info("alloyflow extension started successfully")
	return nil
}

// Shutdown is called when the extension is being stopped.
func (e *alloyFlowExtension) Shutdown(ctx context.Context) error {
	e.stateMutex.Lock()
	switch e.state {
	case stateRunning:
		e.settings.Logger.Info("alloyflow extension shutting down")
		e.state = stateShuttingDown
		// guaranteed to be non-nil since we are in stateRunning
		e.runCancel()
		// unlock so that the run goroutine can transition to terminated
		e.stateMutex.Unlock()

		select {
		case <-e.runExited:
			e.settings.Logger.Info("alloyflow extension shut down successfully")
		case <-ctx.Done():
			e.settings.Logger.Warn("alloyflow extension shutdown interrupted by context", zap.Error(ctx.Err()))
		}
		return nil
	case stateNotStarted:
		e.settings.Logger.Info("alloyflow extension shutdown completed (not started)")
		e.stateMutex.Unlock()
		return nil
	default:
		e.settings.Logger.Warn("alloyflow extension shutdown in current state is a no-op", zap.String("state", e.state.String()))
		e.stateMutex.Unlock()
		return nil
	}
}

// Ready returns nil when the extension is ready to process data.
func (e *alloyFlowExtension) Ready() error {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	switch e.state {
	case stateRunning:
		return nil
	default:
		return fmt.Errorf("alloyflow extension not ready in current state: %s", e.state.String())
	}
}

// NotReady returns an error when the extension is not ready to process data.
func (e *alloyFlowExtension) NotReady() error {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()

	switch e.state {
	case stateRunning:
		return nil
	default:
		return fmt.Errorf("alloyflow extension not ready in current state: %s", e.state.String())
	}
}

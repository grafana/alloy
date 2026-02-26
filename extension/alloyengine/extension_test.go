package alloyengine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/readyctx"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

// defaultTestConfig returns a default test configuration.
func defaultTestConfig() *Config {
	return &Config{
		AlloyConfig: AlloyConfig{
			File: "testdata/config.alloy",
		},
		Flags: map[string]string{},
	}
}

// newTestExtension creates an extension with injectable runCommandFactory and a nop logger.
func newTestExtension(t *testing.T, factory func() *cobra.Command, config *Config) *alloyEngineExtension {
	t.Helper()
	e := newAlloyEngineExtension(config, component.TelemetrySettings{Logger: zap.NewNop()})
	e.runCommandFactory = factory
	return e
}

// blockingCommand returns a cobra command that blocks until the context is cancelled, then returns nil.
func blockingCommand() *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			if fn, ok := readyctx.OnReadyFromContext(cmd.Context()); ok && fn != nil {
				fn()
			}
			<-cmd.Context().Done()
			return nil
		},
	}
}

// blockingCommandWithoutReady blocks until context cancellation but never calls the ready callback.
func blockingCommandWithoutReady() *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			<-cmd.Context().Done()
			return nil
		},
	}
}

// shutdownErrorCommand blocks until context cancellation, then returns the provided error.
func shutdownErrorCommand(err error) *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			if fn, ok := readyctx.OnReadyFromContext(cmd.Context()); ok && fn != nil {
				fn()
			}
			<-cmd.Context().Done()
			return err
		},
	}
}

// retryTrackingState holds state for tracking retry attempts across command instances.
type retryTrackingState struct {
	attempts    int
	succeededAt int
	failCount   int
	err         error
}

func newRetryTrackingCommand(failCount int, err error) (func() *cobra.Command, *retryTrackingState) {
	state := &retryTrackingState{
		failCount: failCount,
		err:       err,
	}
	factory := func() *cobra.Command {
		return &cobra.Command{
			RunE: func(cmd *cobra.Command, args []string) error {
				state.attempts++
				if state.attempts <= state.failCount {
					return state.err
				}

				state.succeededAt = state.attempts
				return nil
			},
		}
	}
	return factory, state
}

func TestConfig_MissingPath(t *testing.T) {
	t.Helper()
	cfg := &Config{
		AlloyConfig: AlloyConfig{
			File: "",
		},
		Flags: map[string]string{},
	}
	require.Error(t, cfg.Validate())
}

func TestLifecycle_SuccessfulStartAndShutdown(t *testing.T) {
	e := newTestExtension(t, blockingCommand, defaultTestConfig())

	ctx := context.Background()
	host := componenttest.NewNopHost()

	require.NoError(t, e.Start(ctx, host))
	require.Eventually(t, func() bool { return e.getState() == stateRunning }, 1*time.Second, 25*time.Millisecond, "extension did not reach stateRunning")
	require.NoError(t, e.Ready())
	require.NoError(t, e.NotReady())

	// Perform graceful shutdown with timeout to avoid hanging tests.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, e.Shutdown(shutdownCtx))

	// Verify the run goroutine has exited and state is terminated.
	require.Eventually(t, func() bool {
		select {
		case <-e.runExited:
			return true
		default:
			return false
		}
	}, 1*time.Second, 25*time.Millisecond, "run command did not exit in time")
	require.Equal(t, stateTerminated, e.getState())
}

func TestLifecycle_StartTwiceFails(t *testing.T) {
	e := newTestExtension(t, blockingCommand, defaultTestConfig())
	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))
	err := e.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestLifecycle_NotReadyWhenNotStarted(t *testing.T) {
	e := newTestExtension(t, blockingCommand, defaultTestConfig())
	require.Error(t, e.Ready())
	require.Error(t, e.NotReady())
}

func TestLifecycle_StayInStartingWhenReadyNotCalled(t *testing.T) {
	e := newTestExtension(t, blockingCommandWithoutReady, defaultTestConfig())
	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	// Give the run goroutine time to start and block (without calling ready).
	time.Sleep(50 * time.Millisecond)

	// We should still be in stateStarting since the ready callback was never invoked.
	require.Equal(t, stateStarting, e.getState())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)
	require.NoError(t, e.Shutdown(shutdownCtx))
}

func TestLifecycle_ShutdownWithRunCommandError(t *testing.T) {
	expected := errors.New("shutdown error")
	e := newTestExtension(t, func() *cobra.Command { return shutdownErrorCommand(expected) }, defaultTestConfig())

	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool { return e.getState() == stateRunning }, 1*time.Second, 25*time.Millisecond, "extension did not reach stateRunning")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, e.Shutdown(shutdownCtx))

	// The internal goroutine should have transitioned to terminated even on error during shutdown.
	require.Eventually(t, func() bool {
		select {
		case <-e.runExited:
			return true
		default:
			return false
		}
	}, 1*time.Second, 25*time.Millisecond, "run command did not exit in time")
	require.Equal(t, stateTerminated, e.getState())
}

func TestLifecycle_RunSucceedsAfterRetries(t *testing.T) {
	testErr := errors.New("temporary failure")
	factory, state := newRetryTrackingCommand(2, testErr) // Fail 2 times, succeed on 3rd attempt
	cfg := defaultTestConfig()
	e := newTestExtension(t, factory, cfg)

	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	// Wait for the command to eventually exit
	require.Eventually(t, func() bool {
		select {
		case <-e.runExited:
			return true
		default:
			return false
		}
	}, 10*time.Second, 100*time.Millisecond, "extension did not exit in time")

	// Verify it succeeded after 3 attempts (2 failures + 1 success)
	require.Equal(t, 3, state.attempts)
	require.Equal(t, 3, state.succeededAt)
	require.Equal(t, stateTerminated, e.getState())
}

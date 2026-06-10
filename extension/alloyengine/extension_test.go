package alloyengine

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/readyctx"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func shutdownExtensionWithTestTimeout(e *alloyEngineExtension) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.Shutdown(ctx)
}

func requireRunExited(t *testing.T, e *alloyEngineExtension, wait, tick time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-e.runExited:
			return true
		default:
			return false
		}
	}, wait, tick, "run command did not exit in time")
}

func requireShutdownAndTerminated(t *testing.T, e *alloyEngineExtension) {
	t.Helper()
	require.NoError(t, shutdownExtensionWithTestTimeout(e))
	requireRunExited(t, e, time.Second, 25*time.Millisecond)
	require.Equal(t, stateTerminated, e.getState())
}

// defaultTestConfig returns a default test configuration.
func defaultTestConfig() *Config {
	return &Config{
		AlloyConfig: AlloyConfig{
			Inline: InlineAlloyConfig{
				Content: "logging { level = \"debug\" }",
			},
		},
		Flags: map[string]string{},
	}
}

// newTestExtension creates an extension with injectable runCommandFactory and a nop logger.
func newTestExtension(t *testing.T, factory func(modulePath string, configs map[string][]byte) *cobra.Command, config *Config) *alloyEngineExtension {
	t.Helper()
	e := newAlloyEngineExtension(config, component.TelemetrySettings{Logger: zap.NewNop()})
	e.runCommandFactory = factory
	t.Cleanup(func() { _ = shutdownExtensionWithTestTimeout(e) })
	return e
}

// blockingCommand returns a cobra command that blocks until the context is cancelled, then returns nil.
func blockingCommand(_ string, _ map[string][]byte) *cobra.Command {
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
func blockingCommandWithoutReady(_ string, _ map[string][]byte) *cobra.Command {
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

func newRetryTrackingCommand(failCount int, err error) (func(string, map[string][]byte) *cobra.Command, *retryTrackingState) {
	state := &retryTrackingState{
		failCount: failCount,
		err:       err,
	}
	factory := func(_ string, _ map[string][]byte) *cobra.Command {
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

func TestConfig_MissingContent(t *testing.T) {
	t.Helper()
	cfg := &Config{
		AlloyConfig: AlloyConfig{},
		Flags:       map[string]string{},
	}
	require.EqualError(t, cfg.Validate(), "either config.file or config.inline.content must be set")
}

func TestLifecycle_StartPassesInlineConfigToFactory(t *testing.T) {
	const content = "logging { level = \"debug\" }"

	var (
		gotModulePath string
		gotConfigs    map[string][]byte
	)
	factory := func(modulePath string, configs map[string][]byte) *cobra.Command {
		gotModulePath = modulePath
		gotConfigs = configs
		return blockingCommand(modulePath, configs)
	}
	e := newTestExtension(t, factory, &Config{
		AlloyConfig: AlloyConfig{
			Inline: InlineAlloyConfig{
				Content: content,
			},
		},
		Flags: map[string]string{},
	})

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, cwd, gotModulePath)
	require.Equal(t, content, string(gotConfigs["config.alloy"]))

	requireShutdownAndTerminated(t, e)
}

func TestLifecycle_SuccessfulStartAndShutdown(t *testing.T) {
	e := newTestExtension(t, blockingCommand, defaultTestConfig())

	host := componenttest.NewNopHost()

	require.NoError(t, e.Start(t.Context(), host))
	require.Eventually(t, func() bool { return e.getState() == stateRunning }, 1*time.Second, 25*time.Millisecond, "extension did not reach stateRunning")
	require.NoError(t, e.Ready())
	require.NoError(t, e.NotReady())

	requireShutdownAndTerminated(t, e)
}

func TestLifecycle_StartTwiceFails(t *testing.T) {
	e := newTestExtension(t, blockingCommand, defaultTestConfig())
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	err := e.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	requireShutdownAndTerminated(t, e)
}

func TestLifecycle_SecondInstanceFailsWhileFirstRunning(t *testing.T) {
	ext1 := newTestExtension(t, blockingCommand, defaultTestConfig())
	ext2 := newTestExtension(t, blockingCommand, defaultTestConfig())

	require.NoError(t, ext1.Start(t.Context(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool { return ext1.getState() == stateRunning }, 1*time.Second, 25*time.Millisecond, "first extension did not reach stateRunning")

	err := ext2.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	require.Contains(t, err.Error(), "only one alloyengine extension can be active per process")
}

func TestLifecycle_NotReadyWhenNotStarted(t *testing.T) {
	e := newTestExtension(t, blockingCommand, defaultTestConfig())
	require.Error(t, e.Ready())
	require.Error(t, e.NotReady())
}

func TestLifecycle_StayInStartingWhenReadyNotCalled(t *testing.T) {
	e := newTestExtension(t, blockingCommandWithoutReady, defaultTestConfig())
	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))

	// Give the run goroutine time to start and block (without calling ready).
	time.Sleep(50 * time.Millisecond)

	// We should still be in stateStarting since the ready callback was never invoked.
	require.Equal(t, stateStarting, e.getState())

	requireShutdownAndTerminated(t, e)
}

func TestLifecycle_ShutdownWithRunCommandError(t *testing.T) {
	expected := errors.New("shutdown error")
	e := newTestExtension(t, func(_ string, _ map[string][]byte) *cobra.Command { return shutdownErrorCommand(expected) }, defaultTestConfig())

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))
	require.Eventually(t, func() bool { return e.getState() == stateRunning }, 1*time.Second, 25*time.Millisecond, "extension did not reach stateRunning")

	requireShutdownAndTerminated(t, e)
}

func TestLifecycle_RunSucceedsAfterRetries(t *testing.T) {
	testErr := errors.New("temporary failure")
	factory, state := newRetryTrackingCommand(2, testErr) // Fail 2 times, succeed on 3rd attempt
	cfg := defaultTestConfig()
	e := newTestExtension(t, factory, cfg)

	require.NoError(t, e.Start(t.Context(), componenttest.NewNopHost()))

	requireRunExited(t, e, 10*time.Second, 100*time.Millisecond)

	// Verify it succeeded after 3 attempts (2 failures + 1 success)
	require.Equal(t, 3, state.attempts)
	require.Equal(t, 3, state.succeededAt)
	require.Equal(t, stateTerminated, e.getState())
}

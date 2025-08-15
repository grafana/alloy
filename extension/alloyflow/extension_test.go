package alloyflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

// newTestExtension creates an extension with injectable runCommandFactory and a nop logger.
func newTestExtension(t *testing.T, factory func() *cobra.Command) *alloyFlowExtension {
	t.Helper()
	cfg := &Config{ConfigPath: "testdata/config.alloy", Flags: map[string]string{}}
	e := newAlloyFlowExtension(cfg, component.TelemetrySettings{Logger: zap.NewNop()})
	e.runCommandFactory = factory
	return e
}

// blockingCommand returns a cobra command that blocks until the context is cancelled, then returns nil.
func blockingCommand() *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			<-cmd.Context().Done()
			return nil
		},
	}
}

// errorCommand returns a cobra command that immediately returns the provided error.
func errorCommand(err error) *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return err
		},
	}
}

// shutdownErrorCommand blocks until context cancellation, then returns the provided error.
func shutdownErrorCommand(err error) *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			<-cmd.Context().Done()
			return err
		},
	}
}

func TestLifecycle_SuccessfulStartAndShutdown(t *testing.T) {
	e := newTestExtension(t, blockingCommand)

	ctx := context.Background()
	host := componenttest.NewNopHost()

	require.NoError(t, e.Start(ctx, host))
	require.Equal(t, stateRunning, e.state)
	require.NoError(t, e.Ready())
	require.NoError(t, e.NotReady())

	// Perform graceful shutdown with timeout to avoid hanging tests.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, e.Shutdown(shutdownCtx))

	// Verify the run goroutine has exited and state is terminated.
	select {
	case <-e.runExited:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("run command did not exit in time")
	}
	require.Equal(t, stateTerminated, e.state)
}

func TestStartTwiceFails(t *testing.T) {
	e := newTestExtension(t, blockingCommand)
	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))
	err := e.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestReadyWhenNotStarted(t *testing.T) {
	e := newTestExtension(t, blockingCommand)
	require.Error(t, e.Ready())
	require.Error(t, e.NotReady())
}

func TestRunCommandUnexpectedError(t *testing.T) {
	expected := errors.New("boom")
	e := newTestExtension(t, func() *cobra.Command { return errorCommand(expected) })

	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	// Wait for the command to exit and extension to transition to terminated.
	select {
	case <-e.runExited:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("run command did not exit in time")
	}

	require.Equal(t, stateTerminated, e.state)
	require.Error(t, e.Ready())

	// Shutdown after termination should be a no-op and not error.
	require.NoError(t, e.Shutdown(context.Background()))
}

func TestShutdownWithRunCommandError(t *testing.T) {
	expected := errors.New("shutdown error")
	e := newTestExtension(t, func() *cobra.Command { return shutdownErrorCommand(expected) })

	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, e.Shutdown(shutdownCtx))

	// The internal goroutine should have transitioned to terminated even on error during shutdown.
	select {
	case <-e.runExited:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("run command did not exit in time")
	}
	require.Equal(t, stateTerminated, e.state)
}

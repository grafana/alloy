package componenttest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestContext returns a context which cancels itself when t finishes.
func TestContext(t testing.TB) context.Context {
	// nolint:usetesting
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

// stopTimeout is how long the stop function returned by RunInBackground waits
// for run to return.
const stopTimeout = 30 * time.Second

// RunInBackground calls run in a new goroutine, such as a component's or a
// Controller's Run method. The returned stop function cancels run's context
// and waits for it to return, failing the test if run returns an error or
// doesn't return in time. stop is also registered as a test cleanup, so tests
// only need to call it to stop run early.
//
// Call stop before checking for goroutine leaks: canceling the context alone
// doesn't wait for the component to finish shutting down.
func RunInBackground(t testing.TB, run func(ctx context.Context) error) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(TestContext(t))
	runErr := make(chan error, 1)
	go func() { runErr <- run(ctx) }()

	stop = sync.OnceFunc(func() {
		cancel()
		select {
		case err := <-runErr:
			require.NoError(t, err)
		case <-time.After(stopTimeout):
			require.FailNow(t, "timed out waiting for Run to exit")
		}
	})
	t.Cleanup(stop)
	return stop
}

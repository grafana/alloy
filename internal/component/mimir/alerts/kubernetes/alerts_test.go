package alerts

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax"
)

func TestAlloyConfigs(t *testing.T) {
	var testCases = []struct {
		name                  string
		config                string
		expectedErrorContains string
	}{
		{
			name: "basic working config",
			config: `
	address = "GRAFANA_CLOUD_METRICS_URL"
	global_config = ""
	basic_auth {
		username = "GRAFANA_CLOUD_USER"
		password = "GRAFANA_CLOUD_API_KEY"
	}`,
		},
		{
			name: "invalid http config",
			config: `
	address = "GRAFANA_CLOUD_METRICS_URL"
	global_config = ""
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"`,
			expectedErrorContains: `at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.config), &args)
			if tc.expectedErrorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorContains)
			}
		})
	}
}

type fakeLifecycle struct {
	updateCalled    atomic.Bool
	startupCalled   atomic.Bool
	restartCalled   atomic.Bool
	shutdownCalled  atomic.Bool
	syncStateCalled atomic.Bool

	startupErr error
	restartErr error
}

func (f *fakeLifecycle) LifecycleUpdate(Arguments) {
	f.updateCalled.Store(true)
}

func (f *fakeLifecycle) Startup(context.Context) error {
	f.startupCalled.Store(true)
	return f.startupErr
}

func (f *fakeLifecycle) Restart(context.Context) error {
	f.restartCalled.Store(true)
	return f.restartErr
}

func (f *fakeLifecycle) Shutdown() {
	f.shutdownCalled.Store(true)
}

func (f *fakeLifecycle) SyncState() {
	f.syncStateCalled.Store(true)
}

type fakeHealthReporter struct {
	mtx sync.Mutex
	err error
}

func (f *fakeHealthReporter) ReportUnhealthy(err error) {
	f.mtx.Lock()
	f.err = err
	f.mtx.Unlock()
}

func (f *fakeHealthReporter) ReportHealthy() {
	f.mtx.Lock()
	f.err = nil
	f.mtx.Unlock()
}

func (f *fakeHealthReporter) getErr() error {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	return f.err
}

func newComponentForTesting(t *testing.T, reg prometheus.Registerer, logger log.Logger) *Component {
	opts := component.Options{
		ID:         "mimir.alerts.kubernetes",
		Logger:     logger,
		Registerer: reg,
	}

	args := Arguments{Address: "http://localhost:8080/"}
	args.SetToDefault()

	c, err := newNoInit(opts, args)
	require.NoError(t, err)
	return c
}

func TestIterationHandlesUpdate(t *testing.T) {
	t.Run("error during restart", func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		logger := log.NewNopLogger()

		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}
		state.restartErr = errors.New("expected test error")

		newArgs := Arguments{}
		newArgs.SetToDefault()
		newArgs.Address = "http://localhost:8080/"

		var wg sync.WaitGroup
		wg.Add(1)

		c := newComponentForTesting(t, reg, logger)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), state, health))
		}()

		require.NoError(t, c.Update(newArgs))
		wg.Wait()

		require.Error(t, health.getErr())
		require.True(t, state.restartCalled.Load())
	})

	t.Run("success", func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		logger := log.NewNopLogger()

		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}

		newArgs := Arguments{}
		newArgs.SetToDefault()
		newArgs.Address = "http://localhost:8080/"

		var wg sync.WaitGroup
		wg.Add(1)

		c := newComponentForTesting(t, reg, logger)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), state, health))
		}()

		require.NoError(t, c.Update(newArgs))
		wg.Wait()

		require.NoError(t, health.getErr())
		require.True(t, state.restartCalled.Load())
	})
}

func TestIterationHandlesContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	reg := prometheus.NewPedanticRegistry()
	logger := log.NewNopLogger()

	health := &fakeHealthReporter{}
	state := &fakeLifecycle{}

	c := newComponentForTesting(t, reg, logger)
	go func() {
		require.ErrorIs(t, c.iteration(ctx, state, health), errShutdown)
	}()

	cancel()
	require.NoError(t, health.getErr())
}

package rules

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/cluster"
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
	basic_auth {
		username = "GRAFANA_CLOUD_USER"
		password = "GRAFANA_CLOUD_API_KEY"
	}
	external_labels = {"label1" = "value1"}`,
		},
		{
			name: "invalid http config",
			config: `
	address = "GRAFANA_CLOUD_METRICS_URL"
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"`,
			expectedErrorContains: `at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured`,
		},
		{
			name: "query matchers valid",
			config: `
	address = "GRAFANA_CLOUD_METRICS_URL"
	extra_query_matchers {
		matcher {
			name = "job"
			match_type = "!="
			value = "bar"
		}
		matcher {
			name = "namespace"
			match_type = "="
			value = "all"
		}
		matcher {
			name = "namespace"
			match_type = "!~"
			value = ".+"
		}
		matcher {
			name = "cluster"
			match_type = "=~"
			value = "prod-.*"
		}
	}
`,
		},
		{
			name: "query matchers empty",
			config: `
	address = "GRAFANA_CLOUD_METRICS_URL"
	extra_query_matchers {}`,
		},
		{
			name: "query matchers invalid",
			config: `
	address = "GRAFANA_CLOUD_METRICS_URL"
	extra_query_matchers {
		matcher {
			name = "job"
			match_type = "!!"
			value = "bar"
		}
	}`,
			expectedErrorContains: `invalid match type`,
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

type fakeCluster struct{}

func (f fakeCluster) Lookup(shard.Key, int, shard.Op) ([]peer.Peer, error) {
	return nil, nil
}

func (f fakeCluster) Peers() []peer.Peer {
	return nil
}

func (f fakeCluster) Ready() bool {
	return true
}

type fakeLeadership struct {
	leader    bool
	changed   bool
	updateErr error
}

func (f *fakeLeadership) update() (bool, error) {
	return f.changed, f.updateErr
}

func (f *fakeLeadership) isLeader() bool {
	return f.leader
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

func (f *fakeLifecycle) update(Arguments) {
	f.updateCalled.Store(true)
}

func (f *fakeLifecycle) startup(context.Context) error {
	f.startupCalled.Store(true)
	return f.startupErr
}

func (f *fakeLifecycle) restart(context.Context) error {
	f.restartCalled.Store(true)
	return f.restartErr
}

func (f *fakeLifecycle) shutdown() {
	f.shutdownCalled.Store(true)
}

func (f *fakeLifecycle) syncState() {
	f.syncStateCalled.Store(true)
}

type fakeHealthReporter struct {
	mtx sync.Mutex
	err error
}

func (f *fakeHealthReporter) reportUnhealthy(err error) {
	f.mtx.Lock()
	f.err = err
	f.mtx.Unlock()
}

func (f *fakeHealthReporter) reportHealthy() {
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
		ID:         "mimir.rules.kubernetes",
		Logger:     logger,
		Registerer: reg,
		GetServiceData: func(name string) (any, error) {
			if name == cluster.ServiceName {
				return &fakeCluster{}, nil
			}

			panic(fmt.Sprintf("unexpected service name %s", name))
		},
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

		leader := &fakeLeadership{}
		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}
		state.restartErr = errors.New("expected test error")

		newArgs := Arguments{Address: "http://localhost:8080/"}
		newArgs.SetToDefault()

		var wg sync.WaitGroup
		wg.Add(1)

		c := newComponentForTesting(t, reg, logger)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), leader, state, health))
		}()

		require.NoError(t, c.Update(newArgs))
		wg.Wait()

		require.Error(t, health.getErr())
		require.True(t, state.restartCalled.Load())
	})

	t.Run("success", func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		logger := log.NewNopLogger()

		leader := &fakeLeadership{}
		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}

		newArgs := Arguments{Address: "http://localhost:8080/"}
		newArgs.SetToDefault()

		var wg sync.WaitGroup
		wg.Add(1)

		c := newComponentForTesting(t, reg, logger)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), leader, state, health))
		}()

		require.NoError(t, c.Update(newArgs))
		wg.Wait()

		require.NoError(t, health.getErr())
		require.True(t, state.restartCalled.Load())
	})
}

func TestIterationHandlesClusterChange(t *testing.T) {
	t.Run("error during leader check", func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		logger := log.NewNopLogger()

		leader := &fakeLeadership{}
		leader.updateErr = errors.New("expected test error")
		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}

		c := newComponentForTesting(t, reg, logger)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), leader, state, health))
		}()

		c.NotifyClusterChange()
		wg.Wait()

		require.Error(t, health.getErr())
		require.False(t, state.restartCalled.Load())
	})

	t.Run("leader not changed", func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		logger := log.NewNopLogger()

		leader := &fakeLeadership{}
		leader.changed = false
		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}

		c := newComponentForTesting(t, reg, logger)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), leader, state, health))
		}()

		c.NotifyClusterChange()
		wg.Wait()

		require.NoError(t, health.getErr())
		require.False(t, state.restartCalled.Load())
	})

	t.Run("error during restart", func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		logger := log.NewNopLogger()

		leader := &fakeLeadership{}
		leader.changed = true
		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}
		state.restartErr = errors.New("expected test error")

		c := newComponentForTesting(t, reg, logger)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), leader, state, health))
		}()

		c.NotifyClusterChange()
		wg.Wait()

		require.Error(t, health.getErr())
		require.True(t, state.restartCalled.Load())
	})

	t.Run("success", func(t *testing.T) {
		reg := prometheus.NewPedanticRegistry()
		logger := log.NewNopLogger()

		leader := &fakeLeadership{}
		leader.changed = true
		health := &fakeHealthReporter{}
		state := &fakeLifecycle{}

		c := newComponentForTesting(t, reg, logger)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, c.iteration(t.Context(), leader, state, health))
		}()

		c.NotifyClusterChange()
		wg.Wait()

		require.NoError(t, health.getErr())
		require.True(t, state.restartCalled.Load())
	})
}

func TestIterationHandlesContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	reg := prometheus.NewPedanticRegistry()
	logger := log.NewNopLogger()

	leader := &fakeLeadership{}
	health := &fakeHealthReporter{}
	state := &fakeLifecycle{}

	c := newComponentForTesting(t, reg, logger)
	go func() {
		require.ErrorIs(t, c.iteration(ctx, leader, state, health), errShutdown)
	}()

	cancel()
	require.NoError(t, health.getErr())
}

func TestIterationHandlesTick(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	logger := log.NewNopLogger()

	leader := &fakeLeadership{}
	health := &fakeHealthReporter{}
	state := &fakeLifecycle{}

	c := newComponentForTesting(t, reg, logger)
	c.ticker.Reset(time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, c.iteration(t.Context(), leader, state, health))
	}()

	wg.Wait()

	require.NoError(t, health.getErr())
	require.True(t, state.syncStateCalled.Load())
}

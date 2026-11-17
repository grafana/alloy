package rules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promFake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"

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

func (f fakeCluster) Enabled() bool {
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

func newComponentForTesting(t *testing.T, reg prometheus.Registerer, logger *slog.Logger) *Component {
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
		logger := slog.New(slog.DiscardHandler)

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
		logger := slog.New(slog.DiscardHandler)

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
		logger := slog.New(slog.DiscardHandler)

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
		logger := slog.New(slog.DiscardHandler)

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
		logger := slog.New(slog.DiscardHandler)

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
		logger := slog.New(slog.DiscardHandler)

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
	logger := slog.New(slog.DiscardHandler)

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
	logger := slog.New(slog.DiscardHandler)

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

// TestStartupWhenMimirIsDown verifies that the component starts in degraded mode when Mimir is unreachable
// during startup, and that it recovers to healthy once Mimir becomes reachable.
func TestStartupWhenMimirIsDown(t *testing.T) {
	// Fake clientsets don't drive the streaming watch-list bookmark protocol,
	// so disable it for this test to avoid WaitForCacheSync hanging on the
	// initial events bookmark.
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	reg := prometheus.NewPedanticRegistry()
	logger := slog.New(slog.DiscardHandler)

	leader := &fakeLeadership{}
	health := &fakeHealthReporter{}

	c := newComponentForTesting(t, reg, logger)

	// Wire fake k8s and prometheus-operator clients so startup() can build
	// informers.
	c.k8sClient = kubeFake.NewClientset()
	c.promClient = promFake.NewSimpleClientset()

	// Make the first few ListRules calls fail with a recoverable error,
	// mimicking Mimir being unreachable while Alloy is starting up.
	mimirClient := newFakeMimirClient()
	mimirClient.listFailures = 6
	c.mimirClient = mimirClient

	c.namespaceSelector = labels.Everything()
	c.ruleSelector = labels.Everything()
	c.leader = &fakeLeadership{leader: true}

	require.NoError(t, c.startup(t.Context()), "startup must succeed even when the initial Mimir sync fails")
	t.Cleanup(c.shutdown)

	// Verify that the eventProcessor is running and consuming all erroring calls from our fake Mimir client.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, 0, mimirClient.remainingListFailures(), "event loop should retry the queued sync until it reaches max retries")
	}, 5*time.Second, 10*time.Millisecond)

	// Verify that component reports as unhealthy since the state sync with Mimir failed.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, component.HealthTypeUnhealthy, c.CurrentHealth().Health, "component should report as unhealthy since Mimir is unreachable")
	}, 5*time.Second, 10*time.Millisecond)

	// Trigger state re-sync.
	c.ticker.Reset(time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, c.iteration(t.Context(), leader, c, health))
	}()
	wg.Wait()

	// Verify that the component report as healthy once the state re-sync is complete.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, component.HealthTypeHealthy, c.CurrentHealth().Health, "component should report as healthy once Mimir is reachable")
	}, 5*time.Second, 10*time.Millisecond)
}

// deleteCountingMimirClient records how many rule groups were deleted from Mimir.
type deleteCountingMimirClient struct {
	*fakeMimirClient
	deletes atomic.Int32
}

func (m *deleteCountingMimirClient) DeleteRuleGroup(ctx context.Context, namespace, group string) error {
	m.deletes.Inc()
	return m.fakeMimirClient.DeleteRuleGroup(ctx, namespace, group)
}

// TestStartupDoesNotDeleteRulesBeforeCacheSync verifies that the initial reconcile after
// startup only runs once the Kubernetes informer caches are filled. If it ran earlier, the
// PrometheusRules would look missing and their rule groups would be deleted from Mimir.
func TestStartupDoesNotDeleteRulesBeforeCacheSync(t *testing.T) {
	// Fake clientsets don't drive the streaming watch-list bookmark protocol,
	// so disable it for this test to avoid WaitForCacheSync hanging on the
	// initial events bookmark.
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	reg := prometheus.NewPedanticRegistry()
	logger := slog.New(slog.DiscardHandler)

	c := newComponentForTesting(t, reg, logger)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace",
			UID:  types.UID("33f8860c-bd06-4c0d-a0b1-a114d6b9937b"),
		},
	}
	rule := &promv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
			UID:       types.UID("64aab764-c95e-4ee9-a932-cd63ba57e6cf"),
		},
		Spec: promv1.PrometheusRuleSpec{
			Groups: []promv1.RuleGroup{
				{
					Name: "group",
					Rules: []promv1.Rule{
						{
							Alert: "alert",
							Expr:  intstr.FromString("expr"),
						},
					},
				},
			},
		},
	}

	c.k8sClient = kubeFake.NewClientset(ns)
	promClient := promFake.NewSimpleClientset(rule)
	// Slow down the initial list of PrometheusRules so that the rule cache is still empty
	// for a while after startup begins.
	promClient.PrependReactor("list", "prometheusrules", func(k8sTesting.Action) (bool, runtime.Object, error) {
		time.Sleep(200 * time.Millisecond)
		return false, nil, nil
	})
	c.promClient = promClient

	// Mimir already holds the rule group from a previous run.
	mimirClient := &deleteCountingMimirClient{fakeMimirClient: newFakeMimirClient()}
	groups, err := convertCRDRuleGroupToRuleGroup(rule.Spec)
	require.NoError(t, err)
	mimirNs := mimirNamespaceForRuleCRD(c.args.MimirNameSpacePrefix, c.args.MimirNamespaceSeparator, rule)
	require.NoError(t, mimirClient.CreateRuleGroup(t.Context(), mimirNs, groups[0]))
	c.mimirClient = mimirClient

	c.namespaceSelector = labels.Everything()
	c.ruleSelector = labels.Everything()
	c.leader = &fakeLeadership{leader: true}

	require.NoError(t, c.startup(t.Context()))
	t.Cleanup(c.shutdown)

	// Wait for the initial sync and the informer events to be processed.
	require.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, component.HealthTypeHealthy, c.CurrentHealth().Health)
		assert.Zero(ct, c.eventProcessor.queue.Len())
	}, 5*time.Second, 10*time.Millisecond)

	assert.Zero(t, mimirClient.deletes.Load(), "no rule group should be deleted from Mimir")
	rules, err := mimirClient.ListRules(t.Context(), "")
	require.NoError(t, err)
	assert.Len(t, rules[mimirNs], 1)
}

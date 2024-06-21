package rules

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/shard"
	"github.com/grafana/alloy/internal/component"
	commonK8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/featuregate"
	lokiClient "github.com/grafana/alloy/internal/loki/client"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/instrument"
	promListers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	_ "k8s.io/component-base/metrics/prometheus/workqueue"
	controller "sigs.k8s.io/controller-runtime"

	promExternalVersions "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	promVersioned "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
)

const (
	configurationUpdate = "configuration-update"
	clusterUpdate       = "cluster-update"
)

var (
	errShutdown = errors.New("component is shutting down")
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.rules.kubernetes",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   nil,
		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			return New(o, c.(Arguments))
		},
	})
}

type Component struct {
	log  log.Logger
	opts component.Options
	args Arguments

	lokiClient   lokiClient.Interface
	k8sClient    kubernetes.Interface
	promClient   promVersioned.Interface
	ruleLister   promListers.PrometheusRuleLister
	ruleInformer cache.SharedIndexInformer

	namespaceLister   coreListers.NamespaceLister
	namespaceInformer cache.SharedIndexInformer
	informerStopChan  chan struct{}
	ticker            *time.Ticker

	queue         workqueue.RateLimitingInterface
	configUpdates chan ConfigUpdate

	namespaceSelector labels.Selector
	ruleSelector      labels.Selector

	currentState commonK8s.RuleGroupsByNamespace

	leader         leadership
	clusterUpdates chan struct{}

	metrics   *metrics
	healthMut sync.RWMutex
	health    component.Health
}

type metrics struct {
	configUpdatesTotal prometheus.Counter
	clusterUpdatesTotal prometheus.Counter

	eventsTotal   *prometheus.CounterVec
	eventsFailed  *prometheus.CounterVec
	eventsRetried *prometheus.CounterVec

	lokiClientTiming *prometheus.HistogramVec
}

func newMetrics() *metrics {
	return &metrics{
		configUpdatesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: "loki_rules",
			Name:      "config_updates_total",
			Help:      "Total number of times the configuration has been updated.",
		}),
		eventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "loki_rules",
			Name:      "events_total",
			Help:      "Total number of events processed, partitioned by event type.",
		}, []string{"type"}),
		eventsFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "loki_rules",
			Name:      "events_failed_total",
			Help:      "Total number of events that failed to be processed, even after retries, partitioned by event type.",
		}, []string{"type"}),
		eventsRetried: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "loki_rules",
			Name:      "events_retried_total",
			Help:      "Total number of retries across all events, partitioned by event type.",
		}, []string{"type"}),
		lokiClientTiming: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "loki_rules",
			Name:      "loki_client_request_duration_seconds",
			Help:      "Duration of requests to the Loki API.",
			Buckets:   instrument.DefBuckets,
		}, instrument.HistogramCollectorBuckets),
	}
}

func (m *metrics) register(r prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{
		m.configUpdatesTotal,
		m.clusterUpdatesTotal,
		m.eventsTotal,
		m.eventsFailed,
		m.eventsRetried,
		m.lokiClientTiming,
	} {
		if err := r.Register(c); err != nil {
			return err
		}
	}

	return nil
}

type ConfigUpdate struct {
	args Arguments
}

var _ component.Component = (*Component)(nil)
var _ component.DebugComponent = (*Component)(nil)
var _ component.HealthComponent = (*Component)(nil)
var _ cluster.Component = (*Component)(nil)

// New creates a new Component and initializes required clients based on the provided configuration.
func New(o component.Options, args Arguments) (*Component, error) {
	c, err := newNoInit(o, args)
	if err != nil {
		return nil, err
	}

	err = c.init()
	if err != nil {
		return nil, fmt.Errorf("initializing component failed: %w", err)
	}

	return c, nil
}

func newNoInit(o component.Options, args Arguments) (*Component, error) {
	m := newMetrics()
	if err := m.register(o.Registerer); err != nil {
		return nil, fmt.Errorf("registering metrics failed: %w", err)
	}

	clusterSvc, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("getting cluster service failed: %w", err)
	}

	c := &Component{
		log:            o.Logger,
		opts:           o,
		args:           args,
		leader:         newComponentLeadership(o.ID, o.Logger, clusterSvc.(cluster.Cluster)),
		configUpdates:  make(chan ConfigUpdate),
		clusterUpdates: make(chan struct{}, 1),
		ticker:         time.NewTicker(args.SyncInterval),
		metrics:        m,
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	c.startupWithRetries(ctx, c.leader, c, c)

	for {
		// iteration only returns a sentinel error to indicate shutdown, otherwise it handles
		// any errors encountered itself by logging and marking the component as unhealthy.
		err := c.iteration(ctx, c.leader, c, c)
		if errors.Is(err, errShutdown) {
			break
		} else if err != nil {
			level.Error(c.log).Log("msg", "unexpected error from iteration loop; this is a bug", "err", err)
			c.reportUnhealthy(err)
		}
	}

	return nil
}

func (c *Component) Update(newConfig component.Arguments) error {
	c.configUpdates <- ConfigUpdate{
		args: newConfig.(Arguments),
	}
	return nil
}

func (c *Component) NotifyClusterChange() {
	// NOTE that we use cluster updates and ownership of a particular key to implement our
	// own leadership election. Once per-component scheduling is introduced to Alloy, this
	// leadership election logic should be removed in favor of per-component scheduling.
	select {
	case c.clusterUpdates <- struct{}{}:
	default: // update already scheduled
	}
}

func (c *Component) startupWithRetries(ctx context.Context, leader leadership, state lifecycle, health healthReporter) {
	startupBackoff := backoff.New(
		ctx,
		backoff.Config{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 10 * time.Second,
			MaxRetries: 0, // infinite retries
		},
	)
	for {
		// Repeatedly check if we are the leader and attempt to start the component
		_, err := leader.update()
		if err != nil {
			level.Error(c.log).Log("msg", "checking leadership during starting failed, will retry", "err", err)
			health.reportUnhealthy(err)
		} else if err := state.startup(ctx); err != nil {
			level.Error(c.log).Log("msg", "starting up component failed, will retry", "err", err)
			health.reportUnhealthy(err)
		} else {
			break
		}
		startupBackoff.Wait()
	}
}

func (c *Component) iteration(ctx context.Context, leader leadership, state lifecycle, health healthReporter) error {
	select {
	case update := <-c.configUpdates:
		c.metrics.configUpdatesTotal.Inc()
		state.update(update.args)

		if err := state.restart(ctx); err != nil {
			level.Error(c.log).Log("msg", "restarting component failed", "trigger", configurationUpdate, "err", err)
			health.reportUnhealthy(err)
		}
	case <-c.clusterUpdates:
		c.metrics.clusterUpdatesTotal.Inc()

		changed, err := leader.update()
		if err != nil {
			level.Error(c.log).Log("msg", "checking leadership failed", "trigger", clusterUpdate, "err", err)
			health.reportUnhealthy(err)
		} else if changed {
			if err := state.restart(ctx); err != nil {
				level.Error(c.log).Log("msg", "restarting component failed", "trigger", clusterUpdate, "err", err)
				health.reportUnhealthy(err)
			}
		}
	case <-ctx.Done():
		state.shutdown()
		return errShutdown
	case <-c.ticker.C:
		state.syncState()
	}

	return nil
}

// update updates the Arguments used to create new Kubernetes or Loki clients
// when restarting the component in response to configuration or cluster updates.
func (c *Component) update(args Arguments) {
	c.args = args
}

// restart stops any existing event processor and starts a new one. This method is
// a shortcut for calling shutdown, init, and startup in sequence.
func (c *Component) restart(ctx context.Context) error {
	c.shutdown()
	if err := c.init(); err != nil {
		return err
	}

	return c.startup(ctx)
}

// startup launches the informers and starts the event loop if this instance is
// the leader. If it is not the leader, startup does nothing.
func (c *Component) startup(ctx context.Context) error {
	if !c.leader.isLeader() {
		level.Info(c.log).Log("msg", "skipping startup because we are not the leader")
		return nil
	}

	cfg := workqueue.RateLimitingQueueConfig{Name: "loki.rules.kubernetes"}
	c.queue = workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), cfg)
	c.informerStopChan = make(chan struct{})

	if err := c.startNamespaceInformer(); err != nil {
		return err
	}
	if err := c.startRuleInformer(); err != nil {
		return err
	}
	if err := c.syncLoki(ctx); err != nil {
		return err
	}
	go c.eventLoop(ctx)
	return nil
}

// shutdown stops processing new events and waits for currently queued ones to be
// processed.
func (c *Component) shutdown() {
	close(c.informerStopChan)
	c.queue.ShutDownWithDrain()
}

// syncState asks the eventProcessor to sync rule state from the Loki Ruler. It does
// not block waiting for state to be synced.
func (c *Component) syncState(ctx context.Context) {
	if err := c.syncLoki(ctx); err != nil {
		return err
	}
}

func (c *Component) init() error {
	level.Info(c.log).Log("msg", "initializing with new configuration")

	// TODO: allow overriding some stuff in RestConfig and k8s client options?
	restConfig, err := controller.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get k8s config: %w", err)
	}

	c.k8sClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	c.promClient, err = promVersioned.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create prometheus operator client: %w", err)
	}

	httpClient := c.args.HTTPClientConfig.Convert()

	c.lokiClient, err = lokiClient.New(c.log, lokiClient.Config{
		ID:               c.args.TenantID,
		Address:          c.args.Address,
		UseLegacyRoutes:  c.args.UseLegacyRoutes,
		HTTPClientConfig: *httpClient,
	}, c.metrics.lokiClientTiming)
	if err != nil {
		return err
	}

	c.ticker.Reset(c.args.SyncInterval)

	c.namespaceSelector, err = commonK8s.ConvertSelectorToListOptions(c.args.RuleNamespaceSelector)
	if err != nil {
		return err
	}

	c.ruleSelector, err = commonK8s.ConvertSelectorToListOptions(c.args.RuleSelector)
	if err != nil {
		return err
	}

	return nil
}

func (c *Component) startNamespaceInformer() error {
	factory := informers.NewSharedInformerFactoryWithOptions(
		c.k8sClient,
		24*time.Hour,
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = c.namespaceSelector.String()
		}),
	)

	namespaces := factory.Core().V1().Namespaces()
	c.namespaceLister = namespaces.Lister()
	c.namespaceInformer = namespaces.Informer()
	_, err := c.namespaceInformer.AddEventHandler(commonK8s.NewQueuedEventHandler(c.log, c.queue))
	if err != nil {
		return err
	}

	factory.Start(c.informerStopChan)
	factory.WaitForCacheSync(c.informerStopChan)
	return nil
}

func (c *Component) startRuleInformer() error {
	factory := promExternalVersions.NewSharedInformerFactoryWithOptions(
		c.promClient,
		24*time.Hour,
		promExternalVersions.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = c.ruleSelector.String()
		}),
	)

	promRules := factory.Monitoring().V1().PrometheusRules()
	c.ruleLister = promRules.Lister()
	c.ruleInformer = promRules.Informer()
	_, err := c.ruleInformer.AddEventHandler(commonK8s.NewQueuedEventHandler(c.log, c.queue))
	if err != nil {
		return err
	}

	factory.Start(c.informerStopChan)
	factory.WaitForCacheSync(c.informerStopChan)
	return nil
}

// healthReporter encapsulates the logic for marking a component as healthy or
// not healthy to make testing portions of the Component easier.
type healthReporter interface {
	// reportUnhealthy marks the owning component as unhealthy
	reportUnhealthy(err error)
	// reportHealthy marks the owning component as healthy
	reportHealthy()
}

// lifecycle encapsulates state transitions and mutable state to make testing
// portions of the Component easier.
type lifecycle interface {
	// update updates the Arguments used for configuring the Component.
	update(args Arguments)

	// startup starts processing events from Kubernetes object changes.
	startup(ctx context.Context) error

	// restart stops the component if running and then starts it again.
	restart(ctx context.Context) error

	// shutdown stops the component, blocking until existing events are processed.
	shutdown()

	// syncState requests that Loki ruler state be synced independent of any
	// changes made to Kubernetes objects.
	syncState()
}

// leadership encapsulates the logic for checking if this instance of the Component
// is the leader among all instances to avoid conflicting updates of the Loki API.
type leadership interface {
	// update checks if this component instance is still the leader, stores the result,
	// and returns true if the leadership status has changed since the last time update
	// was called.
	update() (bool, error)

	// isLeader returns true if this component instance is the leader, false otherwise.
	isLeader() bool
}


// componentLeadership implements leadership based on checking ownership of a specific
// key using a cluster.Cluster service.
type componentLeadership struct {
	id      string
	logger  log.Logger
	cluster cluster.Cluster
	leader  atomic.Bool
}

func newComponentLeadership(id string, logger log.Logger, cluster cluster.Cluster) *componentLeadership {
	return &componentLeadership{
		id:      id,
		logger:  logger,
		cluster: cluster,
	}
}

func (l *componentLeadership) update() (bool, error) {
	peers, err := l.cluster.Lookup(shard.StringKey(l.id), 1, shard.OpReadWrite)
	if err != nil {
		return false, fmt.Errorf("unable to determine leader for %s: %w", l.id, err)
	}

	if len(peers) != 1 {
		return false, fmt.Errorf("unexpected peers from leadership check: %+v", peers)
	}

	isLeader := peers[0].Self
	level.Info(l.logger).Log("msg", "checked leadership of component", "is_leader", isLeader)
	return l.leader.Swap(isLeader) != isLeader, nil
}

func (l *componentLeadership) isLeader() bool {
	return l.leader.Load()
}
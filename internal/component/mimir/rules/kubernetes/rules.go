package rules

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/grafana/alloy/internal/component"
	commonK8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/featuregate"
	mimirClient "github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/ckit/shard"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/instrument"
	promListers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
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

func init() {
	component.Register(component.Registration{
		Name:      "mimir.rules.kubernetes",
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

	mimirClient       mimirClient.Interface
	k8sClient         kubernetes.Interface
	promClient        promVersioned.Interface
	namespaceSelector labels.Selector
	ruleSelector      labels.Selector

	cluster  cluster.Cluster
	isLeader atomic.Bool

	eventProcessor *eventProcessor
	configUpdates  chan ConfigUpdate
	clusterUpdates chan struct{}
	ticker         *time.Ticker

	metrics   *metrics
	healthMut sync.RWMutex
	health    component.Health
}

type metrics struct {
	configUpdatesTotal  prometheus.Counter
	clusterUpdatesTotal prometheus.Counter

	eventsTotal   *prometheus.CounterVec
	eventsFailed  *prometheus.CounterVec
	eventsRetried *prometheus.CounterVec

	mimirClientTiming *prometheus.HistogramVec
}

func (m *metrics) Register(r prometheus.Registerer) error {
	r.MustRegister(
		m.configUpdatesTotal,
		m.clusterUpdatesTotal,
		m.eventsTotal,
		m.eventsFailed,
		m.eventsRetried,
		m.mimirClientTiming,
	)
	return nil
}

func newMetrics() *metrics {
	return &metrics{
		configUpdatesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: "mimir_rules",
			Name:      "config_updates_total",
			Help:      "Total number of times the configuration has been updated.",
		}),
		clusterUpdatesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: "mimir_rules",
			Name:      "cluster_updates_total",
			Help:      "Total number of times the cluster has changed.",
		}),
		eventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "mimir_rules",
			Name:      "events_total",
			Help:      "Total number of events processed, partitioned by event type.",
		}, []string{"type"}),
		eventsFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "mimir_rules",
			Name:      "events_failed_total",
			Help:      "Total number of events that failed to be processed, even after retries, partitioned by event type.",
		}, []string{"type"}),
		eventsRetried: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "mimir_rules",
			Name:      "events_retried_total",
			Help:      "Total number of retries across all events, partitioned by event type.",
		}, []string{"type"}),
		mimirClientTiming: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "mimir_rules",
			Name:      "mimir_client_request_duration_seconds",
			Help:      "Duration of requests to the Mimir API.",
			Buckets:   instrument.DefBuckets,
		}, instrument.HistogramCollectorBuckets),
	}
}

type ConfigUpdate struct {
	args Arguments
	err  chan error
}

var _ component.Component = (*Component)(nil)
var _ component.DebugComponent = (*Component)(nil)
var _ component.HealthComponent = (*Component)(nil)
var _ cluster.Component = (*Component)(nil)

func New(o component.Options, args Arguments) (*Component, error) {
	metrics := newMetrics()
	err := metrics.Register(o.Registerer)
	if err != nil {
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
		cluster:        clusterSvc.(cluster.Cluster),
		configUpdates:  make(chan ConfigUpdate),
		clusterUpdates: make(chan struct{}),
		ticker:         time.NewTicker(args.SyncInterval),
		metrics:        metrics,
	}

	err = c.init()
	if err != nil {
		return nil, fmt.Errorf("initializing component failed: %w", err)
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	startupBackoff := backoff.New(
		ctx,
		backoff.Config{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 10 * time.Second,
			MaxRetries: 0, // infinite retries
		},
	)
	for {
		if err := c.startup(ctx); err != nil {
			level.Error(c.log).Log("msg", "starting up component failed", "err", err)
			c.reportUnhealthy(err)
		} else {
			break
		}
		startupBackoff.Wait()
	}

	for {
		select {
		case update := <-c.configUpdates:
			c.metrics.configUpdatesTotal.Inc()
			c.args = update.args

			if err := c.restart(ctx); err != nil {
				level.Error(c.log).Log("msg", "restarting component failed", "trigger", configurationUpdate, "err", err)
				c.reportUnhealthy(err)
				update.err <- err
			} else {
				update.err <- nil
			}
		case <-c.clusterUpdates:
			c.metrics.clusterUpdatesTotal.Inc()

			changed, err := c.leadershipChanged()
			if err != nil {
				level.Error(c.log).Log("msg", "checking leadership failed", "trigger", clusterUpdate, "err", err)
				c.reportUnhealthy(err)
				continue
			}

			if !changed {
				level.Debug(c.log).Log("msg", "skipping restart since leadership has not changed")
				continue
			}

			if err := c.restart(ctx); err != nil {
				level.Error(c.log).Log("msg", "restarting component failed", "trigger", clusterUpdate, "err", err)
				c.reportUnhealthy(err)
			}
		case <-ctx.Done():
			c.shutdown()
			return nil
		case <-c.ticker.C:
			c.periodicSync()
		}
	}
}

func (c *Component) restart(ctx context.Context) error {
	c.shutdown()
	if err := c.init(); err != nil {
		return err
	}

	return c.startup(ctx)
}

// startup launches the informers and starts the event loop.
func (c *Component) startup(ctx context.Context) error {
	if !c.isLeader.Load() {
		level.Info(c.log).Log("msg", "skipping startup because we are not the leader")
		return nil
	}

	cfg := workqueue.RateLimitingQueueConfig{Name: "mimir.rules.kubernetes"}
	queue := workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), cfg)
	informerStopChan := make(chan struct{})

	namespaceLister, err := c.startNamespaceInformer(queue, informerStopChan)
	if err != nil {
		return err
	}

	ruleLister, err := c.startRuleInformer(queue, informerStopChan)
	if err != nil {
		return err
	}

	c.eventProcessor = c.newEventProcessor(queue, informerStopChan, namespaceLister, ruleLister)
	if err = c.eventProcessor.syncMimir(ctx); err != nil {
		return err
	}

	go c.eventProcessor.run(ctx)
	return nil
}

func (c *Component) shutdown() {
	if c.eventProcessor != nil {
		c.eventProcessor.stop()
		c.eventProcessor = nil
	}
}

func (c *Component) periodicSync() {
	if c.eventProcessor != nil {
		c.eventProcessor.enqueueSyncMimir()
	}
}

func (c *Component) Update(newConfig component.Arguments) error {
	errChan := make(chan error)
	c.configUpdates <- ConfigUpdate{
		args: newConfig.(Arguments),
		err:  errChan,
	}
	return <-errChan
}

func (c *Component) NotifyClusterChange() {
	c.clusterUpdates <- struct{}{}
}

// leadershipChanged checks and sets the current leadership status and returns true if it has changed
func (c *Component) leadershipChanged() (bool, error) {
	peers, err := c.cluster.Lookup(shard.StringKey(c.opts.ID), 1, shard.OpReadWrite)
	if err != nil {
		return false, fmt.Errorf("unable to determine leader for %s: %w", c.opts.ID, err)
	}

	if len(peers) != 1 {
		return false, fmt.Errorf("unexpected peers from leadership check: %+v", peers)
	}

	isLeader := peers[0].Self
	level.Info(c.log).Log("msg", "checked leadership of component", "is_leader", isLeader)
	return c.isLeader.Swap(isLeader) != isLeader, nil
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

	c.mimirClient, err = mimirClient.New(c.log, mimirClient.Config{
		ID:                   c.args.TenantID,
		Address:              c.args.Address,
		UseLegacyRoutes:      c.args.UseLegacyRoutes,
		PrometheusHTTPPrefix: c.args.PrometheusHTTPPrefix,
		HTTPClientConfig:     *httpClient,
	}, c.metrics.mimirClientTiming)
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

func (c *Component) startNamespaceInformer(queue workqueue.RateLimitingInterface, stopChan chan struct{}) (coreListers.NamespaceLister, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		c.k8sClient,
		24*time.Hour,
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = c.namespaceSelector.String()
		}),
	)

	namespaces := factory.Core().V1().Namespaces()
	namespaceLister := namespaces.Lister()
	namespaceInformer := namespaces.Informer()
	_, err := namespaceInformer.AddEventHandler(commonK8s.NewQueuedEventHandler(c.log, queue))
	if err != nil {
		return nil, err
	}

	factory.Start(stopChan)
	factory.WaitForCacheSync(stopChan)
	return namespaceLister, nil
}

func (c *Component) startRuleInformer(queue workqueue.RateLimitingInterface, stopChan chan struct{}) (promListers.PrometheusRuleLister, error) {
	factory := promExternalVersions.NewSharedInformerFactoryWithOptions(
		c.promClient,
		24*time.Hour,
		promExternalVersions.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = c.ruleSelector.String()
		}),
	)

	promRules := factory.Monitoring().V1().PrometheusRules()
	ruleLister := promRules.Lister()
	ruleInformer := promRules.Informer()
	_, err := ruleInformer.AddEventHandler(commonK8s.NewQueuedEventHandler(c.log, queue))
	if err != nil {
		return nil, err
	}

	factory.Start(stopChan)
	factory.WaitForCacheSync(stopChan)
	return ruleLister, nil
}

func (c *Component) newEventProcessor(queue workqueue.RateLimitingInterface, stopChan chan struct{}, namespaceLister coreListers.NamespaceLister, ruleLister promListers.PrometheusRuleLister) *eventProcessor {
	return &eventProcessor{
		queue:             queue,
		stopChan:          stopChan,
		health:            c,
		mimirClient:       c.mimirClient,
		namespaceLister:   namespaceLister,
		ruleLister:        ruleLister,
		namespaceSelector: c.namespaceSelector,
		ruleSelector:      c.ruleSelector,
		namespacePrefix:   c.args.MimirNameSpacePrefix,
		metrics:           c.metrics,
		logger:            c.log,
	}
}

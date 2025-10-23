package rules

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	commonK8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/loki/rules/lokiclient"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/instrument"
	promListers "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
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

func init() {
	component.Register(component.Registration{
		Name:      "loki.rules.kubernetes",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   nil,
		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			return NewComponent(o, c.(Arguments))
		},
	})
}

type Component struct {
	log  log.Logger
	opts component.Options
	args Arguments

	lokiClient   lokiclient.Interface
	k8sClient    kubernetes.Interface
	promClient   promVersioned.Interface
	ruleLister   promListers.PrometheusRuleLister
	ruleInformer cache.SharedIndexInformer

	namespaceLister   coreListers.NamespaceLister
	namespaceInformer cache.SharedIndexInformer
	informerStopChan  chan struct{}
	ticker            *time.Ticker

	queue         workqueue.TypedRateLimitingInterface[commonK8s.Event]
	configUpdates chan ConfigUpdate

	namespaceSelector labels.Selector
	ruleSelector      labels.Selector

	currentState commonK8s.PrometheusRuleGroupsByNamespace

	metrics   *metrics
	healthMut sync.RWMutex
	health    component.Health
}

type metrics struct {
	configUpdatesTotal prometheus.Counter

	eventsTotal   *prometheus.CounterVec
	eventsFailed  *prometheus.CounterVec
	eventsRetried *prometheus.CounterVec

	lokiClientTiming *prometheus.HistogramVec
}

func (m *metrics) Register(r prometheus.Registerer) error {
	m.configUpdatesTotal = util.MustRegisterOrGet(r, m.configUpdatesTotal).(prometheus.Counter)
	m.eventsTotal = util.MustRegisterOrGet(r, m.eventsTotal).(*prometheus.CounterVec)
	m.eventsFailed = util.MustRegisterOrGet(r, m.eventsFailed).(*prometheus.CounterVec)
	m.eventsRetried = util.MustRegisterOrGet(r, m.eventsRetried).(*prometheus.CounterVec)
	m.lokiClientTiming = util.MustRegisterOrGet(r, m.lokiClientTiming).(*prometheus.HistogramVec)
	return nil
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

type ConfigUpdate struct {
	args Arguments
	err  chan error
}

var _ component.Component = (*Component)(nil)
var _ component.DebugComponent = (*Component)(nil)
var _ component.HealthComponent = (*Component)(nil)

func NewComponent(o component.Options, args Arguments) (*Component, error) {
	metrics := newMetrics()
	err := metrics.Register(o.Registerer)
	if err != nil {
		return nil, fmt.Errorf("registering metrics failed: %w", err)
	}

	c := &Component{
		log:           o.Logger,
		opts:          o,
		args:          args,
		configUpdates: make(chan ConfigUpdate),
		ticker:        time.NewTicker(args.SyncInterval),
		metrics:       metrics,
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
			c.shutdown()

			c.args = update.args
			err := c.init()
			if err != nil {
				level.Error(c.log).Log("msg", "updating configuration failed", "err", err)
				c.reportUnhealthy(err)
				update.err <- err
				continue
			}

			err = c.startup(ctx)
			if err != nil {
				level.Error(c.log).Log("msg", "updating configuration failed", "err", err)
				c.reportUnhealthy(err)
				update.err <- err
				continue
			}

			update.err <- nil
		case <-ctx.Done():
			c.shutdown()
			return nil
		case <-c.ticker.C:
			c.queue.Add(commonK8s.Event{
				Typ: eventTypeSyncLoki,
			})
		}
	}
}

// startup launches the informers and starts the event loop.
func (c *Component) startup(ctx context.Context) error {
	cfg := workqueue.TypedRateLimitingQueueConfig[commonK8s.Event]{Name: "loki.rules.kubernetes"}
	c.queue = workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[commonK8s.Event](), cfg)
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

func (c *Component) shutdown() {
	close(c.informerStopChan)
	c.queue.ShutDownWithDrain()
}

func (c *Component) Update(newConfig component.Arguments) error {
	errChan := make(chan error)
	c.configUpdates <- ConfigUpdate{
		args: newConfig.(Arguments),
		err:  errChan,
	}
	return <-errChan
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

	c.lokiClient, err = lokiclient.New(c.log, lokiclient.Config{
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

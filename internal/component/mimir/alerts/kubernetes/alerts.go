package alerts

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/go-kit/log"
	alertmgr_cfg "github.com/grafana/alloy/internal/mimir/alertmanager"
	"github.com/grafana/dskit/backoff"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	_ "k8s.io/component-base/metrics/prometheus/workqueue"
	controller "sigs.k8s.io/controller-runtime"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/mimir/util"
	"github.com/grafana/alloy/internal/featuregate"
	mimirClient "github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/internal/runtime/logging/level"

	commonK8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	promExternalVersions "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	promListers_v1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1alpha1"
	promVersioned "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
)

const (
	configurationUpdate = "configuration-update"
)

var (
	errShutdown = errors.New("component is shutting down")
)

func init() {
	component.Register(component.Registration{
		Name:      "mimir.alerts.kubernetes",
		Stability: featuregate.StabilityExperimental,
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

	// The client via which Alertmanager configs are sent to Mimir.
	mimirClient mimirClient.AlertmanagerInterface

	// Signal an update to the Arguments.
	configUpdates chan ConfigUpdate

	ticker *time.Ticker

	healthMut sync.RWMutex
	health    component.Health
	metrics   *metrics

	// Connection to the Kubernetes API.
	k8sClient kubernetes.Interface
	// Selector for "Namespace" k8s resources which to watch.
	namespaceSelector labels.Selector
	// Selector for "AlertmanagerConfig" k8s resources which to watch.
	cfgSelector labels.Selector
	// Matcher strategy for "AlertmanagerConfig" resources.
	matcherStrategy monitoringv1.AlertmanagerConfigMatcherStrategyType
	// A Prometheus Operator client via which "AlertmanagerConfigs" are retrieved from the Kubernetes API.
	promClient promVersioned.Interface
	// The event processor that watches for changes in the Kubernetes API and updates the Mimir Alertmanager configs.
	eventProcessor *eventProcessor
}

type ConfigUpdate struct {
	args Arguments
}

var _ component.Component = (*Component)(nil)
var _ component.HealthComponent = (*Component)(nil)

// TODO: Implement DebugInfo()
// var _ component.DebugComponent = (*Component)(nil)

// New creates a new Component and initializes required clients based on the provided configuration.
// TODO: Add clustering support
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

	c := &Component{
		log:           o.Logger,
		opts:          o,
		args:          args,
		configUpdates: make(chan ConfigUpdate),
		metrics:       m,
		ticker:        time.NewTicker(args.SyncInterval),
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	//TODO: There's a chance that the startup function is stuck retrying forever.
	// What's worse is that a config update wouldn't be able to fix it, since we haven't entered the config update loop yet.
	c.startupWithRetries(ctx, c, c)

	for {
		// iteration only returns a sentinel error to indicate shutdown, otherwise it handles
		// any errors encountered itself by logging and marking the component as unhealthy.
		err := c.iteration(ctx, c, c)
		if errors.Is(err, errShutdown) {
			break
		} else if err != nil {
			level.Error(c.log).Log("msg", "unexpected error from iteration loop; this is a bug", "err", err)
			c.ReportUnhealthy(err)
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

func (c *Component) startupWithRetries(ctx context.Context, state util.Lifecycle[Arguments], health util.HealthReporter) {
	startupBackoff := backoff.New(
		ctx,
		backoff.Config{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 10 * time.Second,
			MaxRetries: 0, // infinite retries
		},
	)
	for {
		if err := state.Startup(ctx); err != nil {
			level.Error(c.log).Log("msg", "starting up component failed, will retry", "err", err)
			health.ReportUnhealthy(err)
		} else {
			break
		}
		startupBackoff.Wait()
	}
}

func (c *Component) iteration(ctx context.Context, state util.Lifecycle[Arguments], health util.HealthReporter) error {
	select {
	case update := <-c.configUpdates:
		c.metrics.configUpdatesTotal.Inc()
		state.LifecycleUpdate(update.args)

		if err := state.Restart(ctx); err != nil {
			level.Error(c.log).Log("msg", "restarting component failed", "trigger", configurationUpdate, "err", err)
			health.ReportUnhealthy(err)
		}
	case <-ctx.Done():
		state.Shutdown()
		return errShutdown
	case <-c.ticker.C:
		// It's useful to sync periodically:
		// * If an event was missed earlier, it can be synced now.
		// * If the connection to Mimir or to the k8s API didn't work last time, it might work now.
		state.SyncState()
	}

	return nil
}

// update updates the Arguments used to create new Kubernetes or Mimir clients
// when restarting the component in response to configuration or cluster updates.
func (c *Component) LifecycleUpdate(args Arguments) {
	c.args = args
}

// restart stops any existing event processor and starts a new one. This method is
// a shortcut for calling shutdown, init, and startup in sequence.
func (c *Component) Restart(ctx context.Context) error {
	c.Shutdown()
	if err := c.init(); err != nil {
		return err
	}

	return c.Startup(ctx)
}

// startup launches the informers and starts the event loop if this instance is
// the leader. If it is not the leader, startup does nothing.
func (c *Component) Startup(ctx context.Context) error {
	cfg := workqueue.TypedRateLimitingQueueConfig[commonK8s.Event]{Name: "mimir.alerts.kubernetes"}
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[commonK8s.Event](), cfg)
	informerStopChan := make(chan struct{})

	namespaceLister, err := c.startNamespaceInformer(queue, informerStopChan)
	if err != nil {
		return err
	}

	cfgLister, err := c.startConfigInformer(queue, informerStopChan)
	if err != nil {
		return err
	}

	baseCfg, err := alertmgr_cfg.Unmarshal([]byte(c.args.GlobalConfig))
	if err != nil {
		return fmt.Errorf("failed to unmarshal global config: %w", err)
	}
	sb := assets.NewStoreBuilder(c.k8sClient.CoreV1(), c.k8sClient.CoreV1())

	c.eventProcessor = c.newEventProcessor(queue, informerStopChan, namespaceLister, cfgLister, *baseCfg, sb)

	go c.eventProcessor.run(ctx)
	return nil
}

// shutdown stops processing new events and waits for currently queued ones to be
// processed. After this method is called eventProcessor is unset and must be recreated.
func (c *Component) Shutdown() {
	if c.eventProcessor != nil {
		c.eventProcessor.stop()
		c.eventProcessor = nil
	}
}

func (c *Component) SyncState() {
	if c.eventProcessor != nil {
		c.eventProcessor.enqueueSyncMimir()
	}
}

func (c *Component) init() error {
	level.Info(c.log).Log("msg", "initializing with configuration")

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

	c.ticker.Reset(c.args.SyncInterval)

	c.mimirClient, err = mimirClient.New(c.log, mimirClient.Config{
		Address:          c.args.Address,
		HTTPClientConfig: *httpClient,
	}, c.metrics.mimirClientTiming)
	if err != nil {
		return err
	}

	c.namespaceSelector, err = commonK8s.ConvertSelectorToListOptions(c.args.AlertmanagerConfigNamespaceSelector)
	if err != nil {
		return err
	}

	c.cfgSelector, err = commonK8s.ConvertSelectorToListOptions(c.args.AlertmanagerConfigSelector)
	if err != nil {
		return err
	}

	c.matcherStrategy = monitoringv1.AlertmanagerConfigMatcherStrategyType(c.args.AlertmanagerConfigMatcherStrategy)
	if c.matcherStrategy != monitoringv1.OnNamespaceConfigMatcherStrategyType &&
		c.matcherStrategy != monitoringv1.NoneConfigMatcherStrategyType {
		return fmt.Errorf("invalid alertmanagerconfig_matcher_strategy: %s", c.args.AlertmanagerConfigMatcherStrategy)
	}

	return nil
}

func (c *Component) startNamespaceInformer(queue workqueue.TypedRateLimitingInterface[commonK8s.Event], stopChan chan struct{}) (coreListers.NamespaceLister, error) {
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

func (c *Component) startConfigInformer(queue workqueue.TypedRateLimitingInterface[commonK8s.Event], stopChan chan struct{}) (promListers_v1alpha1.AlertmanagerConfigLister, error) {
	factory := promExternalVersions.NewSharedInformerFactoryWithOptions(
		c.promClient,
		24*time.Hour,
		promExternalVersions.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.LabelSelector = c.cfgSelector.String()
		}),
	)

	amConfigs := factory.Monitoring().V1alpha1().AlertmanagerConfigs()
	amConfigsLister := amConfigs.Lister()
	amConfigsInformer := amConfigs.Informer()
	_, err := amConfigsInformer.AddEventHandler(commonK8s.NewQueuedEventHandler(c.log, queue))
	if err != nil {
		return nil, err
	}

	factory.Start(stopChan)
	factory.WaitForCacheSync(stopChan)
	return amConfigsLister, nil
}

func (c *Component) newEventProcessor(queue workqueue.TypedRateLimitingInterface[commonK8s.Event], stopChan chan struct{},
	namespaceLister coreListers.NamespaceLister, cfgLister promListers_v1alpha1.AlertmanagerConfigLister,
	baseCfg alertmgr_cfg.Config, sb *assets.StoreBuilder) *eventProcessor {

	// Deep copy to make sure that a change in arguments won't immediately propagate to the event processor.
	templateFiles := make(map[string]string, len(c.args.TemplateFiles))
	maps.Copy(templateFiles, c.args.TemplateFiles)

	return &eventProcessor{
		queue:             queue,
		stopChan:          stopChan,
		health:            c,
		mimirClient:       c.mimirClient,
		namespaceLister:   namespaceLister,
		cfgLister:         cfgLister,
		baseCfg:           baseCfg,
		namespaceSelector: c.namespaceSelector,
		cfgSelector:       c.cfgSelector,
		matcherStrategy:   c.matcherStrategy,
		metrics:           c.metrics,
		logger:            c.log,
		kclient:           c.k8sClient,
		templateFiles:     templateFiles,
		storeBuilder:      sb,
	}
}

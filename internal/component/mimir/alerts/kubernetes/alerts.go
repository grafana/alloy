package alerts

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	_ "k8s.io/component-base/metrics/prometheus/workqueue"
	controller "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

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

	promExternalVersions "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	promListers_v1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1alpha1"
	promVersioned "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
)

const (
	configurationUpdate = "configuration-update"
	// clusterUpdate       = "cluster-update"
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

	mimirClient mimirClient.Interface

	configUpdates chan ConfigUpdate
	// clusterUpdates chan struct{}
	// leader util.Leadership

	healthMut sync.RWMutex
	health    component.Health
	metrics   *metrics

	k8sClient         kubernetes.Interface
	namespaceSelector labels.Selector
	cfgSelector       labels.Selector
	eventProcessor    *eventProcessor
	promClient        promVersioned.Interface
}

type ConfigUpdate struct {
	args Arguments
}

var _ component.Component = (*Component)(nil)
var _ component.DebugComponent = (*Component)(nil)
var _ component.HealthComponent = (*Component)(nil)

// var _ cluster.Component = (*Component)(nil)

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

	// TODO: Add clustering support
	// clusterSvc, err := o.GetServiceData(cluster.ServiceName)
	// if err != nil {
	// 	return nil, fmt.Errorf("getting cluster service failed: %w", err)
	// }

	c := &Component{
		log:  o.Logger,
		opts: o,
		args: args,
		// leader:        util.NewComponentLeadership(o.ID, o.Logger, clusterSvc.(cluster.Cluster)),
		configUpdates: make(chan ConfigUpdate),
		// clusterUpdates: make(chan struct{}, 1),
		metrics: m,
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	// TODO: Add clustering support
	c.startupWithRetries(ctx, c, c)
	// c.startupWithRetries(ctx, c.leader, c, c)

	for {
		// iteration only returns a sentinel error to indicate shutdown, otherwise it handles
		// any errors encountered itself by logging and marking the component as unhealthy.
		// TODO: Add clustering support
		err := c.iteration(ctx, c, c)
		// err := c.iteration(ctx, c.leader, c, c)
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

// TODO: Add clustering support
// func (c *Component) NotifyClusterChange() {
// 	// NOTE that we use cluster updates and ownership of a particular key to implement our
// 	// own leadership election. Once per-component scheduling is introduced to Alloy, this
// 	// leadership election logic should be removed in favor of per-component scheduling.
// 	select {
// 	case c.clusterUpdates <- struct{}{}:
// 	default: // update already scheduled
// 	}
// }

// TODO: Add clustering support
// func (c *Component) startupWithRetries(ctx context.Context, leader util.Leadership, state util.Lifecycle[Arguments], health util.HealthReporter) {
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
		// TODO: Add clustering support
		// Repeatedly check if we are the leader and attempt to start the component
		// _, err := leader.Update()
		// if err != nil {
		// 	level.Error(c.log).Log("msg", "checking leadership during starting failed, will retry", "err", err)
		// 	health.ReportUnhealthy(err)
		// } else
		if err := state.Startup(ctx); err != nil {
			level.Error(c.log).Log("msg", "starting up component failed, will retry", "err", err)
			health.ReportUnhealthy(err)
		} else {
			break
		}
		startupBackoff.Wait()
	}
}

// TODO: Add clustering support
// func (c *Component) iteration(ctx context.Context, leader util.Leadership, state util.Lifecycle[Arguments], health util.HealthReporter) error {
func (c *Component) iteration(ctx context.Context, state util.Lifecycle[Arguments], health util.HealthReporter) error {
	select {
	case update := <-c.configUpdates:
		c.metrics.configUpdatesTotal.Inc()
		state.LifecycleUpdate(update.args)

		if err := state.Restart(ctx); err != nil {
			level.Error(c.log).Log("msg", "restarting component failed", "trigger", configurationUpdate, "err", err)
			health.ReportUnhealthy(err)
		}
	// TODO: Add clustering support
	// case <-c.clusterUpdates:
	// 	c.metrics.clusterUpdatesTotal.Inc()

	// 	changed, err := leader.Update()
	// 	if err != nil {
	// 		level.Error(c.log).Log("msg", "checking leadership failed", "trigger", clusterUpdate, "err", err)
	// 		health.ReportUnhealthy(err)
	// 	} else if changed {
	// 		if err := state.Restart(ctx); err != nil {
	// 			level.Error(c.log).Log("msg", "restarting component failed", "trigger", clusterUpdate, "err", err)
	// 			health.ReportUnhealthy(err)
	// 		}
	// 	}
	case <-ctx.Done():
		state.Shutdown()
		return errShutdown
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
	// TODO: Add clustering support
	// if !c.leader.IsLeader() {
	// 	level.Info(c.log).Log("msg", "skipping startup because we are not the leader")
	// 	return nil
	// }

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

	var baseCfg alertmgr_cfg.Config
	err = yaml.Unmarshal([]byte(c.args.GlobalConfig), &baseCfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal global config: %w", err)
	}

	c.eventProcessor = c.newEventProcessor(queue, informerStopChan, namespaceLister, cfgLister, baseCfg)

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

// syncState asks the eventProcessor to sync rule state from the Mimir Ruler. It does
// not block waiting for state to be synced.
func (c *Component) SyncState() {}

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

	c.mimirClient, err = mimirClient.New(c.log, mimirClient.Config{
		// TODO: Do we need a tenant ID?
		// ID:               c.args.TenantID,
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
	ruleLister := amConfigs.Lister()
	ruleInformer := amConfigs.Informer()
	_, err := ruleInformer.AddEventHandler(commonK8s.NewQueuedEventHandler(c.log, queue))
	if err != nil {
		return nil, err
	}

	factory.Start(stopChan)
	factory.WaitForCacheSync(stopChan)
	return ruleLister, nil
}

func (c *Component) newEventProcessor(queue workqueue.TypedRateLimitingInterface[commonK8s.Event], stopChan chan struct{},
	namespaceLister coreListers.NamespaceLister, cfgLister promListers_v1alpha1.AlertmanagerConfigLister,
	baseCfg alertmgr_cfg.Config) *eventProcessor {

	// Deep copy to make sure that a change in arguments won't immediately propagate to the event processor.
	templateFiles := make(map[string]string, len(c.args.TemplateFiles))
	maps.Copy(templateFiles, c.args.TemplateFiles)

	// TODO: Deep copy maps and slices
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
		metrics:           c.metrics,
		logger:            c.log,
		kclient:           c.k8sClient,
		templateFiles:     templateFiles,
	}
}

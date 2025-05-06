package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	commonK8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/dskit/instrument"
	"github.com/pkg/errors"
	"github.com/prometheus-operator/prometheus-operator/pkg/alertmanager"
	validation_v1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/alertmanager/validation/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	promListers_v1alpha "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1alpha1"
	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	go_k8s "k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/yaml" // Used for CRD compatibility instead of gopkg.in/yaml.v2

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/mimir/util"
	"github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type eventProcessor struct {
	queue    workqueue.TypedRateLimitingInterface[commonK8s.Event]
	stopChan chan struct{}
	health   util.HealthReporter

	mimirClient       client.Interface
	namespaceLister   coreListers.NamespaceLister
	baseCfg           alertmgr_cfg.Config
	cfgLister         promListers_v1alpha.AlertmanagerConfigLister
	namespaceSelector labels.Selector
	cfgSelector       labels.Selector
	templateFiles     map[string]string

	configBuilder alertmanager.ConfigBuilder

	kclient go_k8s.Interface

	metrics *metrics
	logger  log.Logger
}

type metrics struct {
	configUpdatesTotal  prometheus.Counter
	clusterUpdatesTotal prometheus.Counter

	eventsTotal   *prometheus.CounterVec
	eventsFailed  *prometheus.CounterVec
	eventsRetried *prometheus.CounterVec

	mimirClientTiming *prometheus.HistogramVec
}

func newMetrics() *metrics {
	return &metrics{
		configUpdatesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: "mimir_alerts",
			Name:      "config_updates_total",
			Help:      "Total number of times the configuration has been updated.",
		}),
		clusterUpdatesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: "mimir_alerts",
			Name:      "cluster_updates_total",
			Help:      "Total number of times the cluster has changed.",
		}),
		eventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "mimir_alerts",
			Name:      "events_total",
			Help:      "Total number of events processed, partitioned by event type.",
		}, []string{"type"}),
		eventsFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "mimir_alerts",
			Name:      "events_failed_total",
			Help:      "Total number of events that failed to be processed, even after retries, partitioned by event type.",
		}, []string{"type"}),
		eventsRetried: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "mimir_alerts",
			Name:      "events_retried_total",
			Help:      "Total number of retries across all events, partitioned by event type.",
		}, []string{"type"}),
		mimirClientTiming: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "mimir_alerts",
			Name:      "mimir_client_request_duration_seconds",
			Help:      "Duration of requests to the Mimir API.",
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
		m.mimirClientTiming,
	} {
		if err := r.Register(c); err != nil {
			return err
		}
	}

	return nil
}

// run processes events added to the queue until the queue is shutdown.
func (e *eventProcessor) run(ctx context.Context) {
	// Do an initial reconciliation so that Mimir is updated if there are no AlertmanagerConfig CRDs.
	err := e.reconcileState(ctx)
	if err != nil {
		level.Error(e.logger).Log(
			"msg", "failed to do an initial configuration update",
			"err", err,
		)
		e.health.ReportUnhealthy(err)
	} else {
		e.health.ReportHealthy()
	}

	for {
		evt, shutdown := e.queue.Get()
		if shutdown {
			level.Info(e.logger).Log("msg", "shutting down event loop")
			return
		}

		e.metrics.eventsTotal.WithLabelValues(string(evt.Typ)).Inc()
		err := e.processEvent(ctx, evt)

		if err != nil {
			retries := e.queue.NumRequeues(evt)
			if retries < 5 && client.IsRecoverable(err) {
				e.metrics.eventsRetried.WithLabelValues(string(evt.Typ)).Inc()
				e.queue.AddRateLimited(evt)
				level.Error(e.logger).Log(
					"msg", "failed to process event, will retry",
					"retries", fmt.Sprintf("%d/5", retries),
					"err", err,
				)
				continue
			} else {
				e.metrics.eventsFailed.WithLabelValues(string(evt.Typ)).Inc()
				level.Error(e.logger).Log(
					"msg", "failed to process event, unrecoverable error or max retries exceeded",
					"retries", fmt.Sprintf("%d/5", retries),
					"err", err,
				)
				e.health.ReportUnhealthy(err)
			}
		} else {
			e.health.ReportHealthy()
		}

		e.queue.Forget(evt)
	}
}

// stop stops adding new Kubernetes events to the queue and blocks until all existing
// events have been processed by the run loop.
func (e *eventProcessor) stop() {
	close(e.stopChan)
	// Because this method blocks until the queue is empty, it's important that we don't
	// stop the run loop and let it continue to process existing items in the queue.
	e.queue.ShutDownWithDrain()
}

func (e *eventProcessor) processEvent(ctx context.Context, event kubernetes.Event) error {
	defer e.queue.Done(event)
	return e.reconcileState(ctx)
}

func (c *eventProcessor) provisionAlertmanagerConfiguration(ctx context.Context,
	amConfigs map[string]*promv1alpha1.AlertmanagerConfig, store *assets.Store) (*alertmgr_cfg.Config, error) {
	var (
		// TODO: What to set as matcher strategy and version?
		version, err = semver.New("0.28.1")
		// TODO: Should the matching strategy be configurable/
		cfgBuilder = alertmanager.NewConfigBuilder(c.logger, *version, store, monitoringv1.AlertmanagerConfigMatcherStrategy{Type: "OnNamespace"})
	)

	convertedCfg := c.baseCfg.String()
	err = cfgBuilder.InitializeFromRawConfiguration([]byte(convertedCfg))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize from global AlertmangerConfig: %w", err)
	}

	if err := cfgBuilder.AddAlertmanagerConfigs(ctx, amConfigs); err != nil {
		return nil, errors.Wrap(err, "failed to generate Alertmanager configuration")
	}

	generatedConfig, err := cfgBuilder.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration: %w", err)
	}

	var res alertmgr_cfg.Config
	err = yaml.Unmarshal(generatedConfig, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal generated final configuration: %w", err)
	}

	return &res, nil
}

func (e *eventProcessor) reconcileState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cfg, err := e.desiredStateFromKubernetes()
	if err != nil {
		return err
	}

	// TODO: Get CreateAlertmanagerConfigs ot accept a pointer so that we avoid a copy?
	err = e.mimirClient.CreateAlertmanagerConfigs(ctx, *cfg, e.templateFiles)
	if err != nil {
		return err
	}

	return nil
}

// Load AlertmanagerConfig resources from Kubernetes and
// merge them into one together with the global Alertmanager configuration.
func (e *eventProcessor) desiredStateFromKubernetes() (*alertmgr_cfg.Config, error) {
	cfgs, err := e.getKubernetesState()
	if err != nil {
		return nil, err
	}

	amConfigs := make(map[string]*promv1alpha1.AlertmanagerConfig)
	for namespace, configs := range cfgs {
		for _, config := range configs {
			// Validate the AlertmanagerConfig CRDs
			err := validation_v1alpha1.ValidateAlertmanagerConfig(config)
			if err != nil {
				level.Error(e.logger).Log(
					"msg", "got an invalid AlertmanagerConfig CRD from Kubernetes",
					"namespace", namespace,
					"name", config.Name,
					"err", err,
				)
				continue
			}

			id := namespace + `/` + config.Name
			amConfigs[id] = config
		}
	}

	// TODO: Use a different context?
	ctx := context.Background()
	cfg, err := e.provisionAlertmanagerConfiguration(ctx, amConfigs, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to provision Alertmanager configuration: %w", err)
	}

	return cfg, nil
}

// Returns AlertmanagerConfig resources indexed by Kubernetes namespace.
func (e *eventProcessor) getKubernetesState() (map[string][]*promv1alpha1.AlertmanagerConfig, error) {
	namespaces, err := e.namespaceLister.List(e.namespaceSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	out := make(map[string][]*promv1alpha1.AlertmanagerConfig)
	for _, namespace := range namespaces {
		alertmanagerConfigs, err := e.cfgLister.AlertmanagerConfigs(namespace.Name).List(e.cfgSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to list AlertmanagerConfig CRDs: %w", err)
		}
		out[namespace.Name] = append(out[namespace.Name], alertmanagerConfigs...)
	}

	return out, nil
}

// TODO: Do we need to check for managed namespaces like mimir.rules.kubernetes?
// isManagedMimirNamespace returns true if the namespace is managed by Alloy.
// Unmanaged namespaces are left as is by the operator.
// func isManagedMimirNamespace(prefix, namespace string) bool {
// 	prefixPart := regexp.QuoteMeta(prefix)
// 	namespacePart := `.+`
// 	namePart := `.+`
// 	uuidPart := `[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}`
// 	managedNamespaceRegex := regexp.MustCompile(
// 		fmt.Sprintf("^%s/%s/%s/%s$", prefixPart, namespacePart, namePart, uuidPart),
// 	)
// 	return managedNamespaceRegex.MatchString(namespace)
// }

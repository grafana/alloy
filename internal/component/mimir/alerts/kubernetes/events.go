package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	alertmgr_cfg "github.com/grafana/alloy/internal/mimir/alertmanager"
	"github.com/grafana/dskit/instrument"
	"github.com/prometheus-operator/prometheus-operator/pkg/alertmanager"
	validation_v1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/alertmanager/validation/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/assets"
	promListers_v1alpha "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	go_k8s "k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/mimir/util"
	"github.com/grafana/alloy/internal/mimir/client"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type eventProcessor struct {
	queue    workqueue.TypedRateLimitingInterface[kubernetes.Event]
	stopChan chan struct{}
	health   util.HealthReporter

	mimirClient client.AlertmanagerInterface

	namespaceLister   coreListers.NamespaceLister
	cfgLister         promListers_v1alpha.AlertmanagerConfigLister
	namespaceSelector labels.Selector
	cfgSelector       labels.Selector
	kclient           go_k8s.Interface

	baseCfg       alertmgr_cfg.Config
	templateFiles map[string]string

	metrics *metrics
	logger  log.Logger
}

type metrics struct {
	configUpdatesTotal prometheus.Counter

	eventsTotal   *prometheus.CounterVec
	eventsFailed  *prometheus.CounterVec
	eventsRetried *prometheus.CounterVec

	mimirClientTiming *prometheus.HistogramVec
}

// TODO: Write unit tests which check metrics
func newMetrics() *metrics {
	return &metrics{
		configUpdatesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: "mimir_alerts",
			Name:      "config_updates_total",
			Help:      "Total number of times the configuration has been updated.",
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
	// TODO: This stops the informers, but the informers are not created by eventProcessor.
	//       Create and stop the components in the same struct? To make it more clear what owns them.
	close(e.stopChan)
	// Because this method blocks until the queue is empty, it's important that we don't
	// stop the run loop and let it continue to process existing items in the queue.
	e.queue.ShutDownWithDrain()
}

func (e *eventProcessor) processEvent(ctx context.Context, event kubernetes.Event) error {
	defer e.queue.Done(event)
	return e.reconcileState(ctx)
}

func (e *eventProcessor) enqueueSyncMimir() {
	e.queue.Add(kubernetes.Event{
		Typ: util.EventTypeSyncMimir,
	})
}

func (c *eventProcessor) provisionAlertmanagerConfiguration(ctx context.Context,
	amConfigs map[string]*promv1alpha1.AlertmanagerConfig, store *assets.StoreBuilder) (*alertmgr_cfg.Config, error) {

	var (
		// TODO: Make this configurable?
		version, _ = semver.New("0.29.0")
		// TODO: Add an option to get an Alertmanager CRD through k8s informers.
		cfgBuilder = alertmanager.NewConfigBuilder(slog.New(logging.NewSlogGoKitHandler(c.logger)), *version, store, &monitoringv1.Alertmanager{})
	)

	convertedCfg, err := c.baseCfg.String()
	if err != nil {
		return nil, err
	}

	err = cfgBuilder.InitializeFromRawConfiguration([]byte(convertedCfg))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize from global AlertmangerConfig: %w", err)
	}

	if err := cfgBuilder.AddAlertmanagerConfigs(ctx, amConfigs); err != nil {
		return nil, fmt.Errorf("failed to generate Alertmanager configuration: %w", err)
	}

	generatedConfig, err := cfgBuilder.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration: %w", err)
	}

	res, err := alertmgr_cfg.Unmarshal(generatedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal generated final configuration: %w", err)
	}

	return res, nil
}

func (e *eventProcessor) reconcileState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cfg, err := e.desiredStateFromKubernetes(ctx)
	if err != nil {
		return err
	}

	// TODO: Get Mimir's current Alertmanager config and diff it with the one Alloy has.
	//       If it's the same, do nothing. If it's different, update Mimir.
	err = e.mimirClient.CreateAlertmanagerConfigs(ctx, cfg, e.templateFiles)
	if err != nil {
		return err
	}

	return nil
}

// Load AlertmanagerConfig resources from Kubernetes and
// merge them into one together with the global Alertmanager configuration.
func (e *eventProcessor) desiredStateFromKubernetes(ctx context.Context) (*alertmgr_cfg.Config, error) {
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

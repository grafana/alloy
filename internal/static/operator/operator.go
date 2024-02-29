package operator

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dskit "github.com/grafana/dskit/log"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	controller "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/hierarchy"
	promop_v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promop "github.com/prometheus-operator/prometheus-operator/pkg/operator"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Needed for clients.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

// Config controls the configuration of the Operator.
type Config struct {
	LogLevel            dskit.Level
	LogFormat           string
	Labels              promop.Labels
	Controller          controller.Options
	AgentSelector       string
	KubelsetServiceName string

	agentLabelSelector labels.Selector

	// RestConfig used to connect to cluster. One will be generated based on the
	// environment if not set.
	RestConfig *rest.Config

	// TODO(rfratto): extra settings from Prometheus Operator:
	//
	// 1. Reloader container image/requests/limits
	// 2. Namespaces allow/denylist.
	// 3. Namespaces for Prometheus resources.
}

// NewConfig creates a new Config and initializes default values.
// Flags will be registered against f if it is non-nil.
func NewConfig(f *flag.FlagSet) (*Config, error) {
	if f == nil {
		f = flag.NewFlagSet("temp", flag.PanicOnError)
	}

	var c Config
	err := c.registerFlags(f)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) registerFlags(f *flag.FlagSet) error {
	c.LogLevel.RegisterFlags(f)
	f.Var(&c.Labels, "labels", "Labels to add to all created operator resources")
	f.StringVar(&c.AgentSelector, "agent-selector", "", "Label selector to discover GrafanaAgent CRs. Defaults to all GrafanaAgent CRs.")
	var namespace string
	var webhookServerOptions webhook.Options
	f.StringVar(&namespace, "namespace", "", "Namespace to restrict the Operator to.")                                  // nolint:staticcheck
	f.StringVar(&webhookServerOptions.Host, "listen-host", "", "Host to listen on. Empty string means all interfaces.") // nolint:staticcheck
	f.IntVar(&webhookServerOptions.Port, "listen-port", 9443, "Port to listen on.")                                     // nolint:staticcheck
	f.StringVar(&c.Controller.Metrics.BindAddress, "metrics-listen-address", ":8080", "Address to expose Operator metrics on")
	f.StringVar(&c.Controller.HealthProbeBindAddress, "health-listen-address", "", "Address to expose Operator health probes on")

	f.StringVar(&c.KubelsetServiceName, "kubelet-service", "", "Service and Endpoints objects to write kubelets into. Allows for monitoring Kubelet and cAdvisor metrics using a ServiceMonitor. Must be in format \"namespace/name\". If empty, nothing will be created.")

	c.Controller.WebhookServer = webhook.NewServer(webhookServerOptions)

	if namespace != "" {
		c.Controller.Cache.DefaultNamespaces = map[string]cache.Config{}
		c.Controller.Cache.DefaultNamespaces[namespace] = cache.Config{}
	}

	// Custom initial values for the endpoint names.
	c.Controller.ReadinessEndpointName = "/-/ready"
	c.Controller.LivenessEndpointName = "/-/healthy"

	c.Controller.Scheme = runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		core_v1.AddToScheme,
		apps_v1.AddToScheme,
		gragent.AddToScheme,
		promop_v1.AddToScheme,
	} {
		if err := add(c.Controller.Scheme); err != nil {
			return fmt.Errorf("unable to register scheme: %w", err)
		}
	}

	return nil
}

// Operator is the Grafana Agent Operator.
type Operator struct {
	log     log.Logger
	manager manager.Manager

	// New creates reconcilers to reconcile creating the kubelet service (if
	// configured) and Grafana Agent deployments. We store them as
	// lazyReconcilers so tests can update what the underlying reconciler
	// implementation is.

	kubeletReconciler *lazyReconciler // Unused if kubelet service unconfigured
	agentReconciler   *lazyReconciler
}

// New creates a new Operator.
func New(l log.Logger, c *Config) (*Operator, error) {
	var (
		lazyKubeletReconciler, lazyAgentReconciler lazyReconciler
	)

	restConfig := c.RestConfig
	if restConfig == nil {
		restConfig = controller.GetConfigOrDie()
	}
	manager, err := controller.NewManager(restConfig, c.Controller)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	if err := manager.AddReadyzCheck("running", healthz.Ping); err != nil {
		level.Warn(l).Log("msg", "failed to set up 'running' readyz check", "err", err)
	}
	if err := manager.AddHealthzCheck("running", healthz.Ping); err != nil {
		level.Warn(l).Log("msg", "failed to set up 'running' healthz check", "err", err)
	}

	var (
		agentPredicates []predicate.Predicate

		notifier        = hierarchy.NewNotifier(log.With(l, "component", "hierarchy_notifier"), manager.GetClient())
		notifierHandler = notifier.EventHandler()
	)

	// Initialize agentPredicates if an GrafanaAgent selector is configured.
	if c.AgentSelector != "" {
		sel, err := meta_v1.ParseToLabelSelector(c.AgentSelector)
		if err != nil {
			return nil, fmt.Errorf("unable to create predicate for selecting GrafanaAgent CRs: %w", err)
		}
		c.agentLabelSelector, err = meta_v1.LabelSelectorAsSelector(sel)
		if err != nil {
			return nil, fmt.Errorf("unable to create predicate for selecting GrafanaAgent CRs: %w", err)
		}
		selPredicate, err := predicate.LabelSelectorPredicate(*sel)
		if err != nil {
			return nil, fmt.Errorf("unable to create predicate for selecting GrafanaAgent CRs: %w", err)
		}
		agentPredicates = append(agentPredicates, selPredicate)
	}

	if c.KubelsetServiceName != "" {
		parts := strings.Split(c.KubelsetServiceName, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format for kubelet-service %q, must be formatted as \"namespace/name\"", c.KubelsetServiceName)
		}
		kubeletNamespace := parts[0]
		kubeletName := parts[1]

		err := controller.NewControllerManagedBy(manager).
			For(&core_v1.Node{}).
			Owns(&core_v1.Service{}).
			Owns(&core_v1.Endpoints{}).
			Complete(&lazyKubeletReconciler)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubelet controller: %w", err)
		}

		lazyKubeletReconciler.Set(&kubeletReconciler{
			Client: manager.GetClient(),

			kubeletNamespace: kubeletNamespace,
			kubeletName:      kubeletName,
		})
	}

	err = controller.NewControllerManagedBy(manager).
		For(&gragent.GrafanaAgent{}, builder.WithPredicates(agentPredicates...)).
		Owns(&apps_v1.StatefulSet{}).
		Owns(&apps_v1.DaemonSet{}).
		Owns(&apps_v1.Deployment{}).
		Owns(&core_v1.Secret{}).
		Owns(&core_v1.Service{}).
		Watches(&core_v1.Secret{}, notifierHandler).
		Watches(&gragent.LogsInstance{}, notifierHandler).
		Watches(&gragent.PodLogs{}, notifierHandler).
		Watches(&gragent.MetricsInstance{}, notifierHandler).
		Watches(&gragent.Integration{}, notifierHandler).
		Watches(&promop_v1.PodMonitor{}, notifierHandler).
		Watches(&promop_v1.Probe{}, notifierHandler).
		Watches(&promop_v1.ServiceMonitor{}, notifierHandler).
		Watches(&core_v1.Secret{}, notifierHandler).
		Watches(&core_v1.ConfigMap{}, notifierHandler).
		Complete(&lazyAgentReconciler)
	if err != nil {
		return nil, fmt.Errorf("failed to create GrafanaAgent controller: %w", err)
	}

	lazyAgentReconciler.Set(&reconciler{
		Client:   manager.GetClient(),
		scheme:   manager.GetScheme(),
		notifier: notifier,
		config:   c,
	})

	return &Operator{
		log:     l,
		manager: manager,

		kubeletReconciler: &lazyKubeletReconciler,
		agentReconciler:   &lazyAgentReconciler,
	}, nil
}

// Start starts the operator. It will run until ctx is canceled.
func (o *Operator) Start(ctx context.Context) error {
	return o.manager.Start(ctx)
}

type lazyReconciler struct {
	mut   sync.RWMutex
	inner reconcile.Reconciler
}

// Get returns the current reconciler.
func (lr *lazyReconciler) Get() reconcile.Reconciler {
	lr.mut.RLock()
	defer lr.mut.RUnlock()
	return lr.inner
}

// Set updates the current reconciler.
func (lr *lazyReconciler) Set(inner reconcile.Reconciler) {
	lr.mut.Lock()
	defer lr.mut.Unlock()
	lr.inner = inner
}

// Wrap wraps the current reconciler with a middleware.
func (lr *lazyReconciler) Wrap(mw func(next reconcile.Reconciler) reconcile.Reconciler) {
	lr.mut.Lock()
	defer lr.mut.Unlock()
	lr.inner = mw(lr.inner)
}

// Reconcile calls Reconcile against the current reconciler.
func (lr *lazyReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	lr.mut.RLock()
	defer lr.mut.RUnlock()
	if lr.inner == nil {
		return reconcile.Result{}, fmt.Errorf("no reconciler")
	}
	return lr.inner.Reconcile(ctx, req)
}

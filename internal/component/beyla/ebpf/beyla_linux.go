//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/grafana/beyla/pkg/beyla"
	"github.com/grafana/beyla/pkg/components"
	"github.com/grafana/beyla/pkg/export/prom"
	"github.com/grafana/beyla/pkg/kubeflags"
	"github.com/grafana/beyla/pkg/services"
	"github.com/grafana/beyla/pkg/transform"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
)

func init() {
	component.Register(component.Registration{
		Name:      "beyla.ebpf",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	opts      component.Options
	mut       sync.Mutex
	args      Arguments
	reload    chan struct{}
	reg       *prometheus.Registry
	healthMut sync.RWMutex
	health    component.Health
}

var _ component.HealthComponent = (*Component)(nil)

func (args Routes) Convert() *transform.RoutesConfig {
	routes := beyla.DefaultConfig.Routes
	if args.Unmatch != "" {
		routes.Unmatch = transform.UnmatchType(args.Unmatch)
	}
	routes.Patterns = args.Patterns
	routes.IgnorePatterns = args.IgnorePatterns
	routes.IgnoredEvents = transform.IgnoreMode(args.IgnoredEvents)
	return routes
}

func (args Attributes) Convert() beyla.Attributes {
	attrs := beyla.DefaultConfig.Attributes
	if args.Kubernetes.Enable != "" {
		attrs.Kubernetes.Enable = kubeflags.EnableFlag(args.Kubernetes.Enable)
	}
	attrs.Kubernetes.ClusterName = args.Kubernetes.ClusterName
	return attrs
}

func (args Discovery) Convert() (services.DiscoveryConfig, error) {
	srv, err := args.Services.Convert()
	if err != nil {
		return services.DiscoveryConfig{}, err
	}
	excludeSrv, err := args.ExcludeServices.Convert()
	if err != nil {
		return services.DiscoveryConfig{Services: srv}, err
	}
	return services.DiscoveryConfig{
		Services:        srv,
		ExcludeServices: excludeSrv,
	}, nil
}

func (args Services) Convert() (services.DefinitionCriteria, error) {
	var attrs services.DefinitionCriteria
	for _, s := range args {
		ports, err := stringToPortEnum(s.OpenPorts)
		if err != nil {
			return nil, err
		}
		paths, err := stringToRegexpAttr(s.Path)
		if err != nil {
			return nil, err
		}
		kubernetes, err := s.Kubernetes.Convert()
		if err != nil {
			return nil, err
		}
		podLabels := map[string]*services.RegexpAttr{}
		for k, v := range s.Kubernetes.PodLabels {
			label, err := stringToRegexpAttr(v)
			if err != nil {
				return nil, err
			}
			podLabels[k] = &label
		}

		attrs = append(attrs, services.Attributes{
			Name:      s.Name,
			Namespace: s.Namespace,
			OpenPorts: ports,
			Path:      paths,
			Metadata:  kubernetes,
			PodLabels: podLabels,
		})
	}
	return attrs, nil
}

func (args KubernetesService) Convert() (map[string]*services.RegexpAttr, error) {
	metadata := map[string]*services.RegexpAttr{}
	if args.Namespace != "" {
		namespace, err := stringToRegexpAttr(args.Namespace)
		metadata[services.AttrNamespace] = &namespace
		if err != nil {
			return nil, err
		}
	}
	if args.PodName != "" {
		podName, err := stringToRegexpAttr(args.PodName)
		metadata[services.AttrPodName] = &podName
		if err != nil {
			return nil, err
		}
	}
	if args.DeploymentName != "" {
		deploymentName, err := stringToRegexpAttr(args.DeploymentName)
		metadata[services.AttrDeploymentName] = &deploymentName
		if err != nil {
			return nil, err
		}
	}
	if args.ReplicaSetName != "" {
		replicaSetName, err := stringToRegexpAttr(args.ReplicaSetName)
		metadata[services.AttrReplicaSetName] = &replicaSetName
		if err != nil {
			return nil, err
		}
	}
	if args.StatefulSetName != "" {
		statefulSetName, err := stringToRegexpAttr(args.StatefulSetName)
		metadata[services.AttrStatefulSetName] = &statefulSetName
		if err != nil {
			return nil, err
		}
	}
	if args.DaemonSetName != "" {
		daemonSetName, err := stringToRegexpAttr(args.DaemonSetName)
		metadata[services.AttrDaemonSetName] = &daemonSetName
		if err != nil {
			return nil, err
		}
	}
	if args.OwnerName != "" {
		ownerName, err := stringToRegexpAttr(args.OwnerName)
		metadata[services.AttrOwnerName] = &ownerName
		if err != nil {
			return nil, err
		}
	}
	return metadata, nil
}

func (args Prometheus) Convert() prom.PrometheusConfig {
	p := beyla.DefaultConfig.Prometheus
	if args.Features != nil {
		p.Features = args.Features
	}
	if args.Instrumentations != nil {
		p.Instrumentations = args.Instrumentations
	}
	return p
}

func (args Network) Convert() beyla.NetworkConfig {
	networks := beyla.DefaultConfig.NetworkFlows
	if args.Enable {
		networks.Enable = true
	}
	return networks
}

func New(opts component.Options, args Arguments) (*Component, error) {
	reg := prometheus.NewRegistry()
	c := &Component{
		opts:   opts,
		args:   args,
		reload: make(chan struct{}, 1),
		reg:    reg,
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	var cancel context.CancelFunc
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.reload:
			// cancel any previously running Beyla instance
			if cancel != nil {
				cancel()
			}
			newCtx, cancelFunc := context.WithCancel(ctx)
			cancel = cancelFunc

			c.mut.Lock()
			cfg, err := c.args.Convert()
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "failed to convert arguments", "err", err)
				c.reportUnhealthy(err)
				c.mut.Unlock()
				continue
			}
			c.reportHealthy()
			cfg.Prometheus.Registry = c.reg
			c.mut.Unlock()
			err = components.RunBeyla(newCtx, cfg)
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "failed to run Beyla", "err", err)
				c.reportUnhealthy(err)
				continue
			}
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	baseTarget, err := c.baseTarget()
	if err != nil {
		return err
	}
	c.opts.OnStateChange(Exports{
		Targets: []discovery.Target{baseTarget},
	})
	select {
	case c.reload <- struct{}{}:
	default:
	}
	return nil
}

// baseTarget returns the base target for the component which includes metrics of the instrumented services.
func (c *Component) baseTarget() (discovery.Target, error) {
	data, err := c.opts.GetServiceData(http_service.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http_service.Data)

	return discovery.Target{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(c.opts.ID), "metrics"),
		"instance":             defaultInstance(),
		"job":                  "beyla",
	}, nil
}

func (c *Component) reportUnhealthy(err error) {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = component.Health{
		Health:     component.HealthTypeUnhealthy,
		Message:    err.Error(),
		UpdateTime: time.Now(),
	}
}

func (c *Component) reportHealthy() {
	c.healthMut.Lock()
	defer c.healthMut.Unlock()
	c.health = component.Health{
		Health:     component.HealthTypeHealthy,
		UpdateTime: time.Now(),
	}
}

func (c *Component) CurrentHealth() component.Health {
	c.healthMut.RLock()
	defer c.healthMut.RUnlock()
	return c.health
}

func (c *Component) Handler() http.Handler {
	return promhttp.HandlerFor(c.reg, promhttp.HandlerOpts{})
}

func (a *Arguments) Convert() (*beyla.Config, error) {
	var err error
	cfg := beyla.DefaultConfig
	if a.Output != nil {
		cfg.TracesReceiver = convertTraceConsumers(a.Output.Traces)
	}
	cfg.Port, err = stringToPortEnum(a.Port)
	if err != nil {
		return nil, err
	}
	cfg.Exec, err = stringToRegexpAttr(a.ExecutableName)
	if err != nil {
		return nil, err
	}
	cfg.Routes = a.Routes.Convert()
	cfg.Attributes = a.Attributes.Convert()
	cfg.Discovery, err = a.Discovery.Convert()
	if err != nil {
		return nil, err
	}
	cfg.Prometheus = a.Prometheus.Convert()
	cfg.NetworkFlows = a.Network.Convert()
	if a.Debug {
		cfg.SetDebugMode()
	}
	return &cfg, nil
}

func (args *Arguments) Validate() error {
	if args.Port == "" && args.ExecutableName == "" && len(args.Discovery.Services) == 0 {
		return fmt.Errorf("you need to define at least open_port, executable_name, or services in the discovery section")
	}
	validInstrumentations := map[string]struct{}{"*": {}, "http": {}, "grpc": {}, "redis": {}, "kafka": {}, "sql": {}}
	for _, instrumentation := range args.Prometheus.Instrumentations {
		if _, ok := validInstrumentations[instrumentation]; !ok {
			return fmt.Errorf("invalid prometheus.instrumentations entry: %s", instrumentation)
		}
	}
	validFeatures := map[string]struct{}{
		"application": {},
		"application_span": {},
		"application_service_graph": {},
		"application_process": {},
		"network": {},
	}
	for _, feature := range args.Prometheus.Features {
		if _, ok := validFeatures[feature]; !ok {
			return fmt.Errorf("invalid prometheus.features entry: %s", feature)
		}
	}
	return nil
}

func stringToRegexpAttr(s string) (services.RegexpAttr, error) {
	if s == "" {
		return services.RegexpAttr{}, nil
	}
	re, err := regexp.Compile(s)
	if err != nil {
		return services.RegexpAttr{}, err
	}
	return services.NewPathRegexp(re), nil
}

func stringToPortEnum(s string) (services.PortEnum, error) {
	if s == "" {
		return services.PortEnum{}, nil
	}
	p := services.PortEnum{}
	err := p.UnmarshalText([]byte(s))
	if err != nil {
		return services.PortEnum{}, err
	}
	return p, nil
}

func convertTraceConsumers(consumers []otelcol.Consumer) beyla.TracesReceiverConfig {
	convertedConsumers := make([]beyla.Consumer, len(consumers))
	for i, trace := range consumers {
		convertedConsumers[i] = trace
	}
	return beyla.TracesReceiverConfig{
		Traces: convertedConsumers,
	}
}

func defaultInstance() string {
	hostname := os.Getenv("HOSTNAME")
	if hostname != "" {
		return hostname
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

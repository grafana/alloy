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

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/beyla/pkg/beyla"
	"github.com/grafana/beyla/pkg/components"
	"github.com/grafana/beyla/pkg/services"
	"github.com/grafana/beyla/pkg/transform"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
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
	return &transform.RoutesConfig{
		Unmatch:        transform.UnmatchType(args.Unmatch),
		Patterns:       args.Patterns,
		IgnorePatterns: args.IgnorePatterns,
		IgnoredEvents:  transform.IgnoreMode(args.IgnoredEvents),
	}
}

func (args Attributes) Convert() beyla.Attributes {
	attrs := beyla.DefaultConfig.Attributes
	if args.Kubernetes.Enable != "" {
		attrs.Kubernetes.Enable = transform.KubeEnableFlag(args.Kubernetes.Enable)
	}
	return attrs
}

func (args Discovery) Convert() (services.DiscoveryConfig, error) {
	srv, err := args.Services.Convert()
	if err != nil {
		return services.DiscoveryConfig{}, err
	}
	return services.DiscoveryConfig{
		Services: srv,
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
	namespace, err := stringToRegexpAttr(args.Namespace)
	if err != nil {
		return nil, err
	}
	podName, err := stringToRegexpAttr(args.PodName)
	if err != nil {
		return nil, err
	}
	deploymentName, err := stringToRegexpAttr(args.DeploymentName)
	if err != nil {
		return nil, err
	}
	replicaSetName, err := stringToRegexpAttr(args.ReplicaSetName)
	if err != nil {
		return nil, err
	}
	statefulSetName, err := stringToRegexpAttr(args.StatefulSetName)
	if err != nil {
		return nil, err
	}
	dataaemonSetName, err := stringToRegexpAttr(args.DaemonSetName)
	if err != nil {
		return nil, err
	}
	ownerName, err := stringToRegexpAttr(args.OwnerName)
	if err != nil {
		return nil, err
	}
	return map[string]*services.RegexpAttr{
		services.AttrNamespace:       &namespace,
		services.AttrPodName:         &podName,
		services.AttrDeploymentName:  &deploymentName,
		services.AttrReplicaSetName:  &replicaSetName,
		services.AttrStatefulSetName: &statefulSetName,
		services.AttrDaemonSetName:   &dataaemonSetName,
		services.AttrOwnerName:       &ownerName,
	}, nil
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
			components.RunBeyla(newCtx, cfg)
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
	return &cfg, nil
}

func (args *Arguments) Validate() error {
	if args.Port == "" && args.ExecutableName == "" && len(args.Discovery.Services) == 0 {
		return fmt.Errorf("you need to define at least open_port, executable_name, or services in the discovery section")
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

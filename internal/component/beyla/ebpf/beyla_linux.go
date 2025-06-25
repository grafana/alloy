//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/grafana/beyla/v2/pkg/beyla"
	"github.com/grafana/beyla/v2/pkg/components"
	beylaCfg "github.com/grafana/beyla/v2/pkg/config"
	"github.com/grafana/beyla/v2/pkg/export/attributes"
	"github.com/grafana/beyla/v2/pkg/export/debug"
	"github.com/grafana/beyla/v2/pkg/export/prom"
	"github.com/grafana/beyla/v2/pkg/filter"
	"github.com/grafana/beyla/v2/pkg/kubeflags"
	"github.com/grafana/beyla/v2/pkg/services"
	"github.com/grafana/beyla/v2/pkg/transform"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"golang.org/x/sync/errgroup"

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
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	opts       component.Options
	mut        sync.Mutex
	args       Arguments
	argsUpdate chan Arguments
	reg        *prometheus.Registry
	healthMut  sync.RWMutex
	health     component.Health
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
	if args.WildcardChar != "" {
		routes.WildcardChar = args.WildcardChar
	}
	return routes
}

func (args Attributes) Convert() beyla.Attributes {
	attrs := beyla.DefaultConfig.Attributes
	// Kubernetes
	if args.Kubernetes.Enable != "" {
		attrs.Kubernetes.Enable = kubeflags.EnableFlag(args.Kubernetes.Enable)
	}
	if args.Kubernetes.InformersSyncTimeout != 0 {
		attrs.Kubernetes.InformersSyncTimeout = args.Kubernetes.InformersSyncTimeout
	}
	if args.Kubernetes.InformersResyncPeriod != 0 {
		attrs.Kubernetes.InformersResyncPeriod = args.Kubernetes.InformersResyncPeriod
	}
	attrs.Kubernetes.DisableInformers = args.Kubernetes.DisableInformers
	attrs.Kubernetes.MetaRestrictLocalNode = args.Kubernetes.MetaRestrictLocalNode
	attrs.Kubernetes.ClusterName = args.Kubernetes.ClusterName
	// InstanceID
	if args.InstanceID.HostnameDNSResolution {
		attrs.InstanceID.HostnameDNSResolution = args.InstanceID.HostnameDNSResolution
	}
	attrs.InstanceID.OverrideHostname = args.InstanceID.OverrideHostname
	// Selection
	if args.Select != nil {
		attrs.Select = args.Select.Convert()
	}
	return attrs
}

func (args Selections) Convert() attributes.Selection {
	s := attributes.Selection{}
	for _, a := range args {
		s[attributes.Section(a.Section)] = attributes.InclusionLists{
			Include: a.Include,
			Exclude: a.Exclude,
		}
	}
	return s
}

func (args Discovery) Convert() (services.DiscoveryConfig, error) {
	d := beyla.DefaultConfig.Discovery

	// Services
	srv, err := args.Services.Convert()
	if err != nil {
		return d, err
	}
	d.Services = srv
	excludeSrv, err := args.ExcludeServices.Convert()
	if err != nil {
		return d, err
	}
	d.ExcludeServices = excludeSrv
	if args.DefaultExcludeServices != nil {
		defaultExcludeSrv, err := args.DefaultExcludeServices.Convert()
		if err != nil {
			return d, err
		}
		d.DefaultExcludeServices = defaultExcludeSrv
	}

	// Survey
	survey, err := args.Survey.ConvertGlob()
	if err != nil {
		return d, err
	}
	d.Survey = survey

	// Common fields
	d.SkipGoSpecificTracers = args.SkipGoSpecificTracers
	if args.ExcludeOTelInstrumentedServices {
		d.ExcludeOTelInstrumentedServices = args.ExcludeOTelInstrumentedServices
	}
	return d, nil
}

func serviceConvert[Attr any](
	s Service,
	convertFunc func(string) (Attr, error),
	convertKubernetesFunc func(KubernetesService) (map[string]*Attr, error)) (services.PortEnum, Attr, map[string]*Attr, map[string]*Attr, map[string]*Attr, error) {

	var paths Attr
	var kubernetes map[string]*Attr
	var podLabels map[string]*Attr
	var podAnnotations map[string]*Attr

	ports, err := stringToPortEnum(s.OpenPorts)
	if err != nil {
		return ports, paths, kubernetes, podLabels, podAnnotations, err
	}
	paths, err = convertFunc(s.Path)
	if err != nil {
		return ports, paths, kubernetes, podLabels, podAnnotations, err
	}
	kubernetes, err = convertKubernetesFunc(s.Kubernetes)
	if err != nil {
		return ports, paths, kubernetes, podLabels, podAnnotations, err
	}
	podLabels = map[string]*Attr{}
	for k, v := range s.Kubernetes.PodLabels {
		label, err := convertFunc(v)
		if err != nil {
			return ports, paths, kubernetes, podLabels, podAnnotations, err
		}
		podLabels[k] = &label
	}
	// Convert pod annotations to attributes
	podAnnotations = map[string]*Attr{}
	for k, v := range s.Kubernetes.PodAnnotations {
		annotation, err := convertFunc(v)
		if err != nil {
			return ports, paths, kubernetes, podLabels, podAnnotations, err
		}
		podAnnotations[k] = &annotation
	}
	return ports, paths, kubernetes, podLabels, podAnnotations, nil
}

func (args Services) Convert() (services.RegexDefinitionCriteria, error) {
	var attrs services.RegexDefinitionCriteria
	for _, s := range args {
		ports, paths, kubernetes, podLabels, podAnnotations, err := serviceConvert(
			s,
			stringToRegexpAttr,
			convertKubernetes,
		)

		if err != nil {
			return nil, err
		}

		attrs = append(attrs, services.RegexSelector{
			Name:           s.Name,
			Namespace:      s.Namespace,
			OpenPorts:      ports,
			Path:           paths,
			Metadata:       kubernetes,
			PodLabels:      podLabels,
			ContainersOnly: s.ContainersOnly,
			PodAnnotations: podAnnotations,
		})
	}
	return attrs, nil
}

func (args Services) ConvertGlob() (services.GlobDefinitionCriteria, error) {
	var attrs services.GlobDefinitionCriteria
	for _, s := range args {
		ports, paths, kubernetes, podLabels, podAnnotations, err := serviceConvert(
			s,
			stringToGlobAttr,
			convertKubernetesGlob,
		)

		if err != nil {
			return nil, err
		}

		attrs = append(attrs, services.GlobAttributes{
			Name:           s.Name,
			Namespace:      s.Namespace,
			OpenPorts:      ports,
			Path:           paths,
			Metadata:       kubernetes,
			PodLabels:      podLabels,
			ContainersOnly: s.ContainersOnly,
			PodAnnotations: podAnnotations,
		})
	}
	return attrs, nil
}

func (args Services) Validate() error {
	for i, svc := range args {
		// Check if any Kubernetes fields are defined
		hasKubernetes := svc.Kubernetes.Namespace != "" ||
			svc.Kubernetes.PodName != "" ||
			svc.Kubernetes.DeploymentName != "" ||
			svc.Kubernetes.ReplicaSetName != "" ||
			svc.Kubernetes.StatefulSetName != "" ||
			svc.Kubernetes.DaemonSetName != "" ||
			svc.Kubernetes.OwnerName != "" ||
			len(svc.Kubernetes.PodLabels) > 0

		if svc.OpenPorts == "" && svc.Path == "" && !hasKubernetes {
			return fmt.Errorf("discovery.services[%d] must define at least one of: open_ports, exe_path, or kubernetes configuration", i)
		}
	}
	return nil
}

func convertKubernetesAttributes[T any, Attr any](
	args T,
	getters []func(T) (string, string),
	convertFunc func(string) (Attr, error),
) (map[string]*Attr, error) {

	metadata := map[string]*Attr{}
	for _, getter := range getters {
		alloyAttr, beylaAttr := getter(args)
		if alloyAttr != "" {
			attr, err := convertFunc(alloyAttr)
			if err != nil {
				return nil, err
			}
			metadata[beylaAttr] = &attr
		}
	}
	return metadata, nil
}

var kubernetesGetters = []func(KubernetesService) (string, string){
	func(a KubernetesService) (string, string) { return a.Namespace, services.AttrNamespace },
	func(a KubernetesService) (string, string) { return a.PodName, services.AttrPodName },
	func(a KubernetesService) (string, string) { return a.DeploymentName, services.AttrDeploymentName },
	func(a KubernetesService) (string, string) { return a.ReplicaSetName, services.AttrReplicaSetName },
	func(a KubernetesService) (string, string) { return a.StatefulSetName, services.AttrStatefulSetName },
	func(a KubernetesService) (string, string) { return a.DaemonSetName, services.AttrDaemonSetName },
	func(a KubernetesService) (string, string) { return a.OwnerName, services.AttrOwnerName },
}

// Convert to RegexpAttr
func convertKubernetes(args KubernetesService) (map[string]*services.RegexpAttr, error) {
	return convertKubernetesAttributes(args, kubernetesGetters, stringToRegexpAttr)
}

// Convert to GlobAttr
func convertKubernetesGlob(args KubernetesService) (map[string]*services.GlobAttr, error) {
	return convertKubernetesAttributes(args, kubernetesGetters, stringToGlobAttr)
}

func (args Metrics) Convert() prom.PrometheusConfig {
	p := beyla.DefaultConfig.Prometheus
	if args.Features != nil {
		p.Features = args.Features
	}
	if args.Instrumentations != nil {
		p.Instrumentations = args.Instrumentations
	}
	p.AllowServiceGraphSelfReferences = args.AllowServiceGraphSelfReferences
	return p
}

func (args Metrics) hasNetworkFeature() bool {
	for _, feature := range args.Features {
		if feature == "network" {
			return true
		}
	}
	return false
}

func (args Metrics) hasAppFeature() bool {
	for _, feature := range args.Features {
		switch feature {
		case "application", "application_span", "application_service_graph", "application_process":
			return true
		}
	}
	return false
}

func (args Metrics) Validate() error {
	validInstrumentations := map[string]struct{}{
		"*": {}, "http": {}, "grpc": {}, "redis": {}, "kafka": {}, "sql": {},
	}
	for _, instrumentation := range args.Instrumentations {
		if _, ok := validInstrumentations[instrumentation]; !ok {
			return fmt.Errorf("metrics.instrumentations: invalid value %q", instrumentation)
		}
	}

	validFeatures := map[string]struct{}{
		"application": {}, "application_span": {},
		"application_service_graph": {}, "application_process": {},
		"network": {},
	}
	for _, feature := range args.Features {
		if _, ok := validFeatures[feature]; !ok {
			return fmt.Errorf("metrics.features: invalid value %q", feature)
		}
	}
	return nil
}

func (args Network) Convert(enable bool) beyla.NetworkConfig {
	networks := beyla.DefaultConfig.NetworkFlows
	networks.Enable = enable
	if args.Source != "" {
		networks.Source = args.Source
	}
	if args.AgentIP != "" {
		networks.AgentIP = args.AgentIP
	}
	if args.AgentIPIface != "" {
		networks.AgentIPIface = args.AgentIPIface
	}
	if args.AgentIPType != "" {
		networks.AgentIPType = args.AgentIPType
	}
	if args.ExcludeInterfaces != nil {
		networks.ExcludeInterfaces = args.ExcludeInterfaces
	}
	if args.CacheMaxFlows != 0 {
		networks.CacheMaxFlows = args.CacheMaxFlows
	}
	if args.CacheActiveTimeout != 0 {
		networks.CacheActiveTimeout = args.CacheActiveTimeout
	}
	if args.Direction != "" {
		networks.Direction = args.Direction
	}
	networks.Interfaces = args.Interfaces
	networks.Protocols = args.Protocols
	networks.ExcludeProtocols = args.ExcludeProtocols
	networks.Sampling = args.Sampling
	networks.CIDRs = args.CIDRs
	return networks
}

func (args EBPF) Convert() (*beylaCfg.EBPFTracer, error) {
	ebpf := beyla.DefaultConfig.EBPF
	if args.HTTPRequestTimeout != 0 {
		ebpf.HTTPRequestTimeout = args.HTTPRequestTimeout
	}

	if args.ContextPropagation == "" {
		args.ContextPropagation = "disabled"
	}
	var contextPropagationMode beylaCfg.ContextPropagationMode
	err := contextPropagationMode.UnmarshalText([]byte(args.ContextPropagation))
	if err != nil {
		return nil, err
	}
	ebpf.ContextPropagation = contextPropagationMode

	ebpf.WakeupLen = args.WakeupLen
	ebpf.TrackRequestHeaders = args.TrackRequestHeaders
	ebpf.HighRequestVolume = args.HighRequestVolume
	ebpf.HeuristicSQLDetect = args.HeuristicSQLDetect
	ebpf.BpfDebug = args.BpfDebug
	ebpf.ProtocolDebug = args.ProtocolDebug
	return &ebpf, nil
}

func (args Filters) Convert() filter.AttributesConfig {
	filters := filter.AttributesConfig{
		Application: map[string]filter.MatchDefinition{},
		Network:     map[string]filter.MatchDefinition{},
	}
	for _, app := range args.Application {
		filters.Application[app.Attr] = filter.MatchDefinition{
			Match:    app.Match,
			NotMatch: app.NotMatch,
		}
	}
	for _, net := range args.Network {
		filters.Network[net.Attr] = filter.MatchDefinition{
			Match:    net.Match,
			NotMatch: net.NotMatch,
		}
	}
	return filters
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:       opts,
		args:       args,
		argsUpdate: make(chan Arguments, 1),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	// Add deprecation warnings at the start of Run
	if c.args.Port != "" {
		level.Warn(c.opts.Logger).Log("msg", "The 'open_port' field is deprecated. Use 'discovery.services' instead.")
	}
	if c.args.ExecutableName != "" {
		level.Warn(c.opts.Logger).Log("msg", "The 'executable_name' field is deprecated. Use 'discovery.services' instead.")
	}

	var cancel context.CancelFunc
	var cancelG *errgroup.Group
	for {
		select {
		case <-ctx.Done():
			return nil
		case newArgs := <-c.argsUpdate:
			newArgs = getLatestArgsFromChannel(c.argsUpdate, newArgs)
			c.args = newArgs
			if cancel != nil {
				// cancel any previously running Beyla instance
				cancel()
				level.Info(c.opts.Logger).Log("msg", "waiting for Beyla to terminate")
				if err := cancelG.Wait(); err != nil {
					level.Error(c.opts.Logger).Log("msg", "failed to terminate Beyla", "err", err)
					c.reportUnhealthy(err)
					return err
				}
			}

			level.Info(c.opts.Logger).Log("msg", "starting Beyla component")

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
			c.reg = prometheus.NewRegistry()
			c.reportHealthy()
			cfg.Prometheus.Registry = c.reg
			c.mut.Unlock()

			g, launchCtx := errgroup.WithContext(newCtx)
			cancelG = g

			g.Go(func() error {
				err := components.RunBeyla(launchCtx, cfg)
				if err != nil {
					level.Error(c.opts.Logger).Log("msg", "failed to run Beyla", "err", err)
					c.reportUnhealthy(err)
				}
				return err
			})
		}
	}
}

func getLatestArgsFromChannel[A any](ch chan A, current A) A {
	for {
		select {
		case x := <-ch:
			current = x
		default:
			return current
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

	newArgs := args.(Arguments)
	c.argsUpdate <- newArgs
	return nil
}

// baseTarget returns the base target for the component which includes metrics of the instrumented services.
func (c *Component) baseTarget() (discovery.Target, error) {
	data, err := c.opts.GetServiceData(http_service.ServiceName)
	if err != nil {
		return discovery.EmptyTarget, fmt.Errorf("failed to get HTTP information: %w", err)
	}
	httpData := data.(http_service.Data)

	return discovery.NewTargetFromMap(map[string]string{
		model.AddressLabel:     httpData.MemoryListenAddr,
		model.SchemeLabel:      "http",
		model.MetricsPathLabel: path.Join(httpData.HTTPPathForComponent(c.opts.ID), "metrics"),
		"instance":             defaultInstance(),
		"job":                  "beyla",
	}), nil
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
	cfg.Routes = a.Routes.Convert()
	cfg.Attributes = a.Attributes.Convert()
	cfg.Discovery, err = a.Discovery.Convert()
	if err != nil {
		return nil, err
	}
	cfg.Prometheus = a.Metrics.Convert()
	cfg.NetworkFlows = a.Metrics.Network.Convert(a.Metrics.hasNetworkFeature())
	cfg.EnforceSysCaps = a.EnforceSysCaps

	ebpf, err := a.EBPF.Convert()
	if err != nil {
		return nil, err
	}
	cfg.EBPF = *ebpf

	cfg.Filters = a.Filters.Convert()
	cfg.TracePrinter = debug.TracePrinter(a.TracePrinter)

	if a.Debug {
		// TODO: integrate Beyla internal logging with Alloy global logging
		lvl := slog.LevelVar{}
		lvl.Set(slog.LevelDebug)
		cfg.ExternalLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: &lvl,
		})).Handler(), a.Debug)
	}

	return &cfg, nil
}

func (args *Arguments) Validate() error {
	hasNetworkFeature := args.Metrics.hasNetworkFeature()
	hasAppFeature := args.Metrics.hasAppFeature()

	isTracingEnabled := args.TracePrinter != "" && args.TracePrinter != string(debug.TracePrinterDisabled)
	hasOutputConfig := args.Output != nil && args.Output.Traces != nil

	if args.TracePrinter == "" {
		args.TracePrinter = string(debug.TracePrinterDisabled)
	} else if !debug.TracePrinter(args.TracePrinter).Valid() {
		return fmt.Errorf("trace_printer: invalid value %q. Valid values are: disabled, counter, text, json, json_indent", args.TracePrinter)
	}

	if err := args.Metrics.Validate(); err != nil {
		return err
	}

	if hasAppFeature {
		if len(args.Discovery.Services) == 0 && len(args.Discovery.Survey) == 0 {
			return fmt.Errorf("discovery.services or discovery.survey is required when application features are enabled")
		}
		if len(args.Discovery.Services) > 0 {
			if err := args.Discovery.Services.Validate(); err != nil {
				return fmt.Errorf("invalid discovery configuration: %s", err.Error())
			}
		}
		if len(args.Discovery.Survey) > 0 {
			if err := args.Discovery.Survey.Validate(); err != nil {
				return fmt.Errorf("invalid survey configuration: %s", err.Error())
			}
		}
	}

	if len(args.Discovery.ExcludeServices) > 0 {
		if err := args.Discovery.ExcludeServices.Validate(); err != nil {
			return fmt.Errorf("invalid exclude_services configuration: %s", err.Error())
		}
	}

	if !hasNetworkFeature && !hasAppFeature && !isTracingEnabled && !hasOutputConfig {
		return fmt.Errorf("either metrics.features must include at least one of: [network, application, application_span, application_service_graph, application_process], or tracing must be enabled via trace_printer or output section")
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

func stringToGlobAttr(s string) (services.GlobAttr, error) {
	if s == "" {
		return services.GlobAttr{}, nil
	}

	globAttr := services.GlobAttr{}
	err := globAttr.UnmarshalText([]byte(s))

	if err != nil {
		return services.GlobAttr{}, err
	}
	return globAttr, nil
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

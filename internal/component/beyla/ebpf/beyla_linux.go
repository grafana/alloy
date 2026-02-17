//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/grafana/beyla/v2/pkg/beyla"
	"github.com/grafana/beyla/v2/pkg/components"
	beylaSvc "github.com/grafana/beyla/v2/pkg/services"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/obi/pkg/appolly/services"
	obiCfg "go.opentelemetry.io/obi/pkg/config"
	"go.opentelemetry.io/obi/pkg/export/attributes"
	"go.opentelemetry.io/obi/pkg/export/debug"
	"go.opentelemetry.io/obi/pkg/export/instrumentations"
	"go.opentelemetry.io/obi/pkg/export/prom"
	"go.opentelemetry.io/obi/pkg/filter"
	"go.opentelemetry.io/obi/pkg/kube/kubeflags"
	"go.opentelemetry.io/obi/pkg/obi"
	"go.opentelemetry.io/obi/pkg/transform"
	"golang.org/x/sync/errgroup" //nolint:depguard

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	http_service "github.com/grafana/alloy/internal/service/http"
)

func init() {
	beyla.OverrideOBIGlobalConfig()
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

const (
	SamplerAlwaysOn                = "always_on"
	SamplerAlwaysOff               = "always_off"
	SamplerTraceIDRatio            = "traceidratio"
	SamplerParentBasedAlwaysOn     = "parentbased_always_on"
	SamplerParentBasedAlwaysOff    = "parentbased_always_off"
	SamplerParentBasedTraceIDRatio = "parentbased_traceidratio"
)

var validInstrumentations = map[string]struct{}{
	"*": {}, "http": {}, "grpc": {}, "redis": {}, "kafka": {}, "sql": {}, "gpu": {}, "mongo": {},
}

func (args Routes) Convert() *transform.RoutesConfig {
	routes := beyla.DefaultConfig().Routes
	if args.Unmatch != "" {
		routes.Unmatch = transform.UnmatchType(args.Unmatch)
	}
	routes.Patterns = args.Patterns
	routes.IgnorePatterns = args.IgnorePatterns
	routes.IgnoredEvents = transform.IgnoreMode(args.IgnoredEvents)
	if args.WildcardChar != "" {
		routes.WildcardChar = args.WildcardChar
	}
	if args.MaxPathSegmentCardinality > 0 {
		routes.MaxPathSegmentCardinality = args.MaxPathSegmentCardinality
	}
	return routes
}

func (args SamplerConfig) Validate() error {
	if args.Name == "" {
		return nil // Empty name is valid, will use default
	}

	validSamplers := map[string]bool{
		SamplerAlwaysOn:                true,
		SamplerAlwaysOff:               true,
		SamplerTraceIDRatio:            true,
		SamplerParentBasedAlwaysOn:     true,
		SamplerParentBasedAlwaysOff:    true,
		SamplerParentBasedTraceIDRatio: true,
	}

	if !validSamplers[args.Name] {
		return fmt.Errorf("invalid sampler name %q. Valid values are: %s, %s, %s, %s, %s, %s", args.Name,
			SamplerAlwaysOn, SamplerAlwaysOff, SamplerTraceIDRatio,
			SamplerParentBasedAlwaysOn, SamplerParentBasedAlwaysOff, SamplerParentBasedTraceIDRatio)
	}

	// Validate arg for ratio-based samplers
	if args.Name == SamplerTraceIDRatio || args.Name == SamplerParentBasedTraceIDRatio {
		if args.Arg == "" {
			return fmt.Errorf("sampler %q requires an arg parameter with a ratio value between 0 and 1", args.Name)
		}

		ratio, err := strconv.ParseFloat(args.Arg, 64)
		if err != nil {
			return fmt.Errorf("invalid arg %q for sampler %q: must be a valid decimal number", args.Arg, args.Name)
		}

		if ratio < 0 || ratio > 1 {
			return fmt.Errorf("invalid arg %q for sampler %q: ratio must be between 0 and 1 (inclusive)", args.Arg, args.Name)
		}
	}

	return nil
}

func (args SamplerConfig) Convert() services.SamplerConfig {
	return services.SamplerConfig{
		Name: args.Name,
		Arg:  args.Arg,
	}
}

func (args Attributes) Convert() beyla.Attributes {
	attrs := beyla.DefaultConfig().Attributes
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
	if args.Kubernetes.MetaCacheAddress != "" {
		attrs.Kubernetes.MetaCacheAddress = args.Kubernetes.MetaCacheAddress
	}
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

func (args Discovery) Convert() (beylaSvc.BeylaDiscoveryConfig, error) {
	d := beyla.DefaultConfig().Discovery

	// Services (deprecated)
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

	if len(args.Instrument) > 0 {
		instrument, err := args.Instrument.ConvertGlob()
		if err != nil {
			return d, err
		}
		d.Instrument = instrument
	}

	if len(args.ExcludeInstrument) > 0 {
		excludeInstrument, err := args.ExcludeInstrument.ConvertGlob()
		if err != nil {
			return d, err
		}
		d.ExcludeInstrument = excludeInstrument
	}

	if len(args.DefaultExcludeInstrument) > 0 {
		defaultExcludeInstrument, err := args.DefaultExcludeInstrument.ConvertGlob()
		if err != nil {
			return d, err
		}
		d.DefaultExcludeInstrument = defaultExcludeInstrument
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

func convertExportModes(modes []string) (services.ExportModes, error) {
	if modes == nil {
		return services.ExportModeUnset, nil
	}

	ret := services.NewExportModes()

	for _, m := range modes {
		switch m {
		case "metrics":
			ret.AllowMetrics()
		case "traces":
			ret.AllowTraces()
		default:
			return ret, fmt.Errorf("invalid export mode: '%s'", m)
		}
	}

	return ret, nil
}

func serviceConvert[Attr any](
	s Service,
	convertFunc func(string) (Attr, error),
	convertKubernetesFunc func(KubernetesService) (map[string]*Attr, error)) (services.PortEnum, Attr, map[string]*Attr, map[string]*Attr, map[string]*Attr, services.ExportModes, error) {

	var paths Attr
	var kubernetes map[string]*Attr
	var podLabels map[string]*Attr
	var podAnnotations map[string]*Attr
	var exportModes services.ExportModes

	ports, err := stringToPortEnum(s.OpenPorts)
	if err != nil {
		return ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err
	}
	paths, err = convertFunc(s.Path)
	if err != nil {
		return ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err
	}
	kubernetes, err = convertKubernetesFunc(s.Kubernetes)
	if err != nil {
		return ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err
	}
	podLabels = map[string]*Attr{}
	for k, v := range s.Kubernetes.PodLabels {
		label, err := convertFunc(v)
		if err != nil {
			return ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err
		}
		podLabels[k] = &label
	}
	// Convert pod annotations to attributes
	podAnnotations = map[string]*Attr{}
	for k, v := range s.Kubernetes.PodAnnotations {
		annotation, err := convertFunc(v)
		if err != nil {
			return ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err
		}
		podAnnotations[k] = &annotation
	}

	exportModes, err = convertExportModes(s.ExportModes)

	if err != nil {
		return ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err
	}

	return ports, paths, kubernetes, podLabels, podAnnotations, exportModes, nil
}

func (args Services) Convert() (services.RegexDefinitionCriteria, error) {
	var attrs services.RegexDefinitionCriteria
	for _, s := range args {
		ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err := serviceConvert(
			s,
			stringToRegexpAttr,
			convertKubernetes,
		)

		if err != nil {
			return nil, err
		}

		var samplerConfig *services.SamplerConfig
		if s.Sampler.Name != "" || s.Sampler.Arg != "" {
			config := s.Sampler.Convert()
			samplerConfig = &config
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
			ExportModes:    exportModes,
			SamplerConfig:  samplerConfig,
		})
	}
	return attrs, nil
}

func (args Services) ConvertGlob() (services.GlobDefinitionCriteria, error) {
	var attrs services.GlobDefinitionCriteria
	for _, s := range args {
		ports, paths, kubernetes, podLabels, podAnnotations, exportModes, err := serviceConvert(
			s,
			stringToGlobAttr,
			convertKubernetesGlob,
		)

		if err != nil {
			return nil, err
		}

		var samplerConfig *services.SamplerConfig
		if s.Sampler.Name != "" || s.Sampler.Arg != "" {
			config := s.Sampler.Convert()
			samplerConfig = &config
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
			ExportModes:    exportModes,
			SamplerConfig:  samplerConfig,
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
	p := beyla.DefaultConfig().Prometheus
	if args.Features != nil {
		p.Features = args.Features
	}
	if args.Instrumentations != nil {
		p.Instrumentations = args.Instrumentations
	}
	p.AllowServiceGraphSelfReferences = args.AllowServiceGraphSelfReferences
	if args.ExtraResourceLabels != nil {
		p.ExtraResourceLabels = args.ExtraResourceLabels
	}
	if args.ExtraSpanResourceLabels != nil {
		p.ExtraSpanResourceLabels = args.ExtraSpanResourceLabels
	}
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
		case "application", "application_host", "application_span", "application_service_graph",
			"application_process", "application_span_otel", "application_span_sizes":
			return true
		}
	}
	return false
}

func (args Metrics) Validate() error {
	for _, instrumentation := range args.Instrumentations {
		if _, ok := validInstrumentations[instrumentation]; !ok {
			return fmt.Errorf("metrics.instrumentations: invalid value %q", instrumentation)
		}
	}

	validFeatures := map[string]struct{}{
		"application": {}, "application_span": {}, "application_span_otel": {},
		"application_span_sizes": {}, "application_host": {},
		"application_service_graph": {}, "application_process": {},
		"network": {}, "network_inter_zone": {},
	}
	for _, feature := range args.Features {
		if _, ok := validFeatures[feature]; !ok {
			return fmt.Errorf("metrics.features: invalid value %q", feature)
		}
	}
	return nil
}

func (args Network) Convert(enable bool) obi.NetworkConfig {
	networks := beyla.DefaultConfig().NetworkFlows
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

func (args EBPF) Convert() (*obiCfg.EBPFTracer, error) {
	ebpf := beyla.DefaultConfig().EBPF
	if args.HTTPRequestTimeout != 0 {
		ebpf.HTTPRequestTimeout = args.HTTPRequestTimeout
	}

	if args.ContextPropagation == "" {
		args.ContextPropagation = "disabled"
	}
	var contextPropagationMode obiCfg.ContextPropagationMode
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

func (c *Component) loadConfig() (*beyla.Config, error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	cfg, err := c.args.Convert()
	if err != nil {
		return nil, fmt.Errorf("failed to convert arguments: %w", err)
	}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse env: %w", err)
	}

	if cfg.Discovery.SurveyEnabled() {
		cfg.Discovery.OverrideDefaultExcludeForSurvey()
	}

	c.reg = prometheus.NewRegistry()
	c.reportHealthy()

	cfg.Prometheus.Registry = c.reg

	return cfg, nil
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

	// Add deprecation warnings for legacy discovery fields
	if len(c.args.Discovery.Services) > 0 {
		level.Warn(c.opts.Logger).Log("msg", "discovery.services is deprecated, use discovery.instrument instead")
	}
	if len(c.args.Discovery.ExcludeServices) > 0 {
		level.Warn(c.opts.Logger).Log("msg", "discovery.exclude_services is deprecated, use discovery.exclude_instrument instead")
	}
	if len(c.args.Discovery.DefaultExcludeServices) > 0 {
		level.Warn(c.opts.Logger).Log("msg", "discovery.default_exclude_services is deprecated, use discovery.default_exclude_instrument instead")
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
					level.Error(c.opts.Logger).Log("msg", "Beyla terminated with error", "err", err)
					c.reportUnhealthy(err)
				}
			}

			level.Info(c.opts.Logger).Log("msg", "starting Beyla component")

			newCtx, cancelFunc := context.WithCancel(ctx)
			cancel = cancelFunc

			cfg, err := c.loadConfig()
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "failed to load config", "err", err)
				c.reportUnhealthy(err)
				continue
			}

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
	cfg := beyla.DefaultConfig()

	if a.Output != nil {
		cfg.TracesReceiver = a.Traces.Convert(a.Output.Traces)
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

	return cfg, nil
}

func (args *Arguments) Validate() error {
	hasAppFeature := args.Metrics.hasAppFeature()

	if args.TracePrinter == "" {
		args.TracePrinter = string(debug.TracePrinterDisabled)
	} else if !debug.TracePrinter(args.TracePrinter).Valid() {
		return fmt.Errorf("trace_printer: invalid value %q. Valid values are: disabled, counter, text, json, json_indent", args.TracePrinter)
	}

	if err := args.Metrics.Validate(); err != nil {
		return err
	}

	if err := args.Traces.Validate(); err != nil {
		return err
	}

	// If traces block is defined with instrumentations, output section must be defined
	if len(args.Traces.Instrumentations) > 0 || args.Traces.Sampler.Name != "" {
		if args.Output == nil {
			return fmt.Errorf("traces block is defined but output section is missing. When using traces configuration, you must define an output block")
		}
	}

	if hasAppFeature {
		// Check if any discovery method is configured (new or legacy)
		hasAnyDiscovery := len(args.Discovery.Services) > 0 ||
			len(args.Discovery.Survey) > 0 ||
			len(args.Discovery.Instrument) > 0

		if !hasAnyDiscovery {
			return fmt.Errorf("discovery.services, discovery.instrument, or discovery.survey is required when application features are enabled")
		}

		// Validate legacy services field
		if len(args.Discovery.Services) > 0 {
			if err := args.Discovery.Services.Validate(); err != nil {
				return fmt.Errorf("invalid discovery configuration: %s", err.Error())
			}
		}

		// Validate survey field
		if len(args.Discovery.Survey) > 0 {
			if err := args.Discovery.Survey.Validate(); err != nil {
				return fmt.Errorf("invalid survey configuration: %s", err.Error())
			}
		}

		// Validate new instrument field
		if len(args.Discovery.Instrument) > 0 {
			if err := args.Discovery.Instrument.Validate(); err != nil {
				return fmt.Errorf("invalid instrument configuration: %s", err.Error())
			}
		}
	}

	// Validate legacy exclude_services field
	if len(args.Discovery.ExcludeServices) > 0 {
		if err := args.Discovery.ExcludeServices.Validate(); err != nil {
			return fmt.Errorf("invalid exclude_services configuration: %s", err.Error())
		}
	}

	// Validate new exclude_instrument field
	if len(args.Discovery.ExcludeInstrument) > 0 {
		if err := args.Discovery.ExcludeInstrument.Validate(); err != nil {
			return fmt.Errorf("invalid exclude_instrument configuration: %s", err.Error())
		}
	}

	// Validate new default_exclude_instrument field
	if len(args.Discovery.DefaultExcludeInstrument) > 0 {
		if err := args.Discovery.DefaultExcludeInstrument.Validate(); err != nil {
			return fmt.Errorf("invalid default_exclude_instrument configuration: %s", err.Error())
		}
	}

	// Validate per-service samplers for legacy services
	for i, service := range args.Discovery.Services {
		if err := service.Sampler.Validate(); err != nil {
			return fmt.Errorf("invalid sampler configuration in discovery.services[%d]: %s", i, err.Error())
		}
	}

	// Validate per-service samplers for new instrument field
	for i, service := range args.Discovery.Instrument {
		if err := service.Sampler.Validate(); err != nil {
			return fmt.Errorf("invalid sampler configuration in discovery.instrument[%d]: %s", i, err.Error())
		}
	}

	// Validate per-service samplers for survey field
	for i, service := range args.Discovery.Survey {
		if err := service.Sampler.Validate(); err != nil {
			return fmt.Errorf("invalid sampler configuration in discovery.survey[%d]: %s", i, err.Error())
		}
	}

	return nil
}

func (args Traces) Convert(consumers []otelcol.Consumer) beyla.TracesReceiverConfig {
	// Convert the OTEL consumers
	convertedConsumers := make([]beyla.Consumer, len(consumers))
	for i, trace := range consumers {
		convertedConsumers[i] = trace
	}

	config := beyla.TracesReceiverConfig{
		Traces: convertedConsumers,
	}

	if len(args.Instrumentations) == 0 {
		config.Instrumentations = []string{
			instrumentations.InstrumentationALL,
		}
	} else {
		config.Instrumentations = args.Instrumentations
	}
	if args.Sampler.Name != "" || args.Sampler.Arg != "" {
		config.Sampler = args.Sampler.Convert()
	}
	return config
}

func (args Traces) Validate() error {
	for _, instrumentation := range args.Instrumentations {
		if _, ok := validInstrumentations[instrumentation]; !ok {
			return fmt.Errorf("traces.instrumentations: invalid value %q", instrumentation)
		}
	}

	// Validate the global sampler config
	if err := args.Sampler.Validate(); err != nil {
		return fmt.Errorf("invalid global sampler configuration: %s", err.Error())
	}

	return nil
}

func stringToRegexpAttr(s string) (services.RegexpAttr, error) {
	var attr services.RegexpAttr
	if err := attr.UnmarshalText([]byte(s)); err != nil {
		return services.RegexpAttr{}, err
	}
	return attr, nil
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

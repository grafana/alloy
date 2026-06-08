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
	"strings"
	"sync"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/grafana/beyla/v3/pkg/beyla"
	"github.com/grafana/beyla/v3/pkg/components"
	beylaSvc "github.com/grafana/beyla/v3/pkg/services"
	"github.com/grafana/beyla/v3/pkg/webhook/configmap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/obi/pkg/appolly/services"
	obiCfg "go.opentelemetry.io/obi/pkg/config"
	"go.opentelemetry.io/obi/pkg/export"
	"go.opentelemetry.io/obi/pkg/export/attributes"
	"go.opentelemetry.io/obi/pkg/export/debug"
	"go.opentelemetry.io/obi/pkg/export/instrumentations"
	"go.opentelemetry.io/obi/pkg/export/prom"
	"go.opentelemetry.io/obi/pkg/filter"
	"go.opentelemetry.io/obi/pkg/kube"
	"go.opentelemetry.io/obi/pkg/kube/kubeflags"
	"go.opentelemetry.io/obi/pkg/netolly/flowdef"
	"go.opentelemetry.io/obi/pkg/obi"
	"go.opentelemetry.io/obi/pkg/transform"
	"golang.org/x/sync/errgroup" //nolint:depguard

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/featuregate"
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

const (
	none = "none"
)

var validInstrumentations = map[string]struct{}{
	"*": {}, "http": {}, "grpc": {}, "redis": {}, "kafka": {}, "sql": {}, "gpu": {}, "mongo": {}, "memcached": {}, "genai": {},
}

func (args Routes) Convert() *transform.RoutesConfig {
	defaultRoutes := *beyla.DefaultConfig().Routes
	routes := &defaultRoutes
	if args.Unmatch != "" {
		routes.Unmatch = transform.UnmatchType(args.Unmatch)
	}
	routes.Patterns = args.Patterns
	//nolint:staticcheck // OBI does not expose a replacement API for ignored route patterns yet.
	routes.IgnorePatterns = args.IgnorePatterns
	//nolint:staticcheck // OBI does not expose a replacement API for ignored route modes yet.
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
		Name: services.SamplerName(args.Name),
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
	if args.Kubernetes.KubeconfigPath != "" {
		attrs.Kubernetes.KubeconfigPath = args.Kubernetes.KubeconfigPath
	}
	if args.Kubernetes.ReconnectInitialInterval != 0 {
		attrs.Kubernetes.ReconnectInitialInterval = args.Kubernetes.ReconnectInitialInterval
	}
	attrs.Kubernetes.DropExternal = args.Kubernetes.DropExternal
	if args.Kubernetes.ServiceNameTemplate != "" {
		attrs.Kubernetes.ServiceNameTemplate = args.Kubernetes.ServiceNameTemplate
	}
	if args.Kubernetes.ResourceLabels != nil {
		attrs.Kubernetes.ResourceLabels = kube.ResourceLabels(args.Kubernetes.ResourceLabels)
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
	if args.RenameUnresolvedHosts != nil {
		attrs.RenameUnresolvedHosts = *args.RenameUnresolvedHosts
	}
	if args.RenameUnresolvedHostsOutgoing != nil {
		attrs.RenameUnresolvedHostsOutgoing = *args.RenameUnresolvedHostsOutgoing
	}
	if args.RenameUnresolvedHostsIncoming != nil {
		attrs.RenameUnresolvedHostsIncoming = *args.RenameUnresolvedHostsIncoming
	}
	if args.MetricSpanNamesLimit != 0 {
		attrs.MetricSpanNameAggregationLimit = args.MetricSpanNamesLimit
	}
	if args.HostID.Override != "" {
		attrs.HostID.Override = args.HostID.Override
	}
	if args.MetadataRetry.Timeout != 0 {
		attrs.MetadataRetry.Timeout = args.MetadataRetry.Timeout
	}
	if args.MetadataRetry.StartInterval != 0 {
		attrs.MetadataRetry.StartInterval = args.MetadataRetry.StartInterval
	}
	if args.MetadataRetry.MaxInterval != 0 {
		attrs.MetadataRetry.MaxInterval = args.MetadataRetry.MaxInterval
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
	if args.PollInterval != 0 {
		d.PollInterval = args.PollInterval
	}
	if args.MinProcessAge != 0 {
		d.MinProcessAge = args.MinProcessAge
	}
	if args.DefaultOtlpGRPCPort != 0 {
		d.DefaultOtlpGRPCPort = args.DefaultOtlpGRPCPort
	}
	d.ExcludeOTelInstrumentedServicesSpanMetrics = args.ExcludeOTelInstrumentedServicesSpanMetrics
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
	convertKubernetesFunc func(KubernetesService) (map[string]*Attr, error)) (services.IntEnum, Attr, Attr, map[string]*Attr, map[string]*Attr, map[string]*Attr, services.ExportModes, error) {

	var paths Attr
	var cmdArgs Attr
	var kubernetes map[string]*Attr
	var podLabels map[string]*Attr
	var podAnnotations map[string]*Attr
	var exportModes services.ExportModes

	ports, err := stringToPortEnum(s.OpenPorts)
	if err != nil {
		return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err
	}
	paths, err = convertFunc(s.Path)
	if err != nil {
		return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err
	}
	cmdArgs, err = convertFunc(s.CmdArgs)
	if err != nil {
		return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err
	}
	kubernetes, err = convertKubernetesFunc(s.Kubernetes)
	if err != nil {
		return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err
	}
	podLabels = map[string]*Attr{}
	for k, v := range s.Kubernetes.PodLabels {
		label, err := convertFunc(v)
		if err != nil {
			return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err
		}
		podLabels[k] = &label
	}
	// Convert pod annotations to attributes
	podAnnotations = map[string]*Attr{}
	for k, v := range s.Kubernetes.PodAnnotations {
		annotation, err := convertFunc(v)
		if err != nil {
			return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err
		}
		podAnnotations[k] = &annotation
	}

	exportModes, err = convertExportModes(s.ExportModes)

	if err != nil {
		return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err
	}

	return ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, nil
}

func (args Services) Convert() (services.RegexDefinitionCriteria, error) {
	var attrs services.RegexDefinitionCriteria
	for _, s := range args {
		ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err := serviceConvert(
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
			CmdArgs:        cmdArgs,
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
		ports, paths, cmdArgs, kubernetes, podLabels, podAnnotations, exportModes, err := serviceConvert(
			s,
			stringToGlobAttr,
			convertKubernetesGlob,
		)

		if err != nil {
			return nil, err
		}

		languages, err := stringToGlobAttr(s.Languages)
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
			CmdArgs:        cmdArgs,
			Languages:      languages,
			PIDs:           s.PIDs,
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

		if svc.OpenPorts == "" && svc.Path == "" && svc.CmdArgs == "" && !hasKubernetes {
			return fmt.Errorf("discovery.services[%d] must define at least one of: open_ports, exe_path, cmd_args, or kubernetes configuration", i)
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
	if args.Instrumentations != nil {
		p.Instrumentations = stringsToInstrumentations(args.Instrumentations)
	}
	p.AllowServiceGraphSelfReferences = args.AllowServiceGraphSelfReferences
	if args.ExtraResourceLabels != nil {
		p.ExtraResourceLabels = args.ExtraResourceLabels
	}
	if args.ExtraSpanResourceLabels != nil {
		p.ExtraSpanResourceLabels = args.ExtraSpanResourceLabels
	}
	if args.ExemplarFilter != "" {
		p.ExemplarFilter = args.ExemplarFilter
	}
	if args.TTL != 0 {
		p.TTL = args.TTL
	}
	if args.SpanServiceCacheSize != 0 {
		p.SpanMetricsServiceCacheSize = args.SpanServiceCacheSize
	}
	if args.NativeHistogram.BucketFactor != 0 {
		p.NativeHistogram.BucketFactor = args.NativeHistogram.BucketFactor
	}
	if args.NativeHistogram.MaxBucketNumber != 0 {
		p.NativeHistogram.MaxBucketNumber = args.NativeHistogram.MaxBucketNumber
	}
	if args.NativeHistogram.MinResetDuration != 0 {
		p.NativeHistogram.MinResetDuration = args.NativeHistogram.MinResetDuration
	}
	if args.Buckets.DurationHistogram != nil {
		p.Buckets.DurationHistogram = args.Buckets.DurationHistogram
	}
	if args.Buckets.RequestSizeHistogram != nil {
		p.Buckets.RequestSizeHistogram = args.Buckets.RequestSizeHistogram
	}
	if args.Buckets.ResponseSizeHistogram != nil {
		p.Buckets.ResponseSizeHistogram = args.Buckets.ResponseSizeHistogram
	}
	if args.Buckets.GenAITokenUsageHistogram != nil {
		p.Buckets.GenAITokenUsageHistogram = args.Buckets.GenAITokenUsageHistogram
	}
	if args.Buckets.GenAIClientDurationHistogram != nil {
		p.Buckets.GenAIClientDurationHistogram = args.Buckets.GenAIClientDurationHistogram
	}
	if args.Buckets.StatTCPRttHistogram != nil {
		p.Buckets.StatTCPRttHistogram = args.Buckets.StatTCPRttHistogram
	}
	return p
}

func (args Metrics) hasAppFeature() bool {
	for _, feature := range args.Features {
		switch feature {
		case "*", "all":
			return true
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
		"stats": {},
		"*":     {},
		"all":   {},
	}
	for _, feature := range args.Features {
		if _, ok := validFeatures[feature]; !ok {
			return fmt.Errorf("metrics.features: invalid value %q", feature)
		}
	}

	validExemplarFilters := map[string]struct{}{"always_on": {}, "always_off": {}, "trace_based": {}}
	if args.ExemplarFilter != "" {
		if _, ok := validExemplarFilters[args.ExemplarFilter]; !ok {
			return fmt.Errorf("metrics.exemplar_filter: invalid value %q", args.ExemplarFilter)
		}
	}
	return nil
}

// applyToNetwork copies GeoIP settings into the GeoIP sub-field of an
// obi.NetworkConfig. Only non-zero values override the existing field.
// We mutate through the parent struct because geoip/rdns are internal OBI
// packages that cannot be imported from outside the OBI module.
// applyToStats mirrors this for obi.StatsConfig (separate upstream type,
// same internal GeoIP/ReverseDNS fields).
func (args GeoIP) applyToNetwork(n *obi.NetworkConfig) {
	if args.IPInfoPath != "" {
		n.GeoIP.IPInfo.Path = args.IPInfoPath
	}
	if args.MaxMindCountryPath != "" {
		n.GeoIP.MaxMindInfo.CountryPath = args.MaxMindCountryPath
	}
	if args.MaxMindASNPath != "" {
		n.GeoIP.MaxMindInfo.ASNPath = args.MaxMindASNPath
	}
	if args.CacheLen != 0 {
		n.GeoIP.CacheLen = args.CacheLen
	}
	if args.CacheTTL != 0 {
		n.GeoIP.CacheTTL = args.CacheTTL
	}
}

// applyToNetwork copies ReverseDNS settings into the ReverseDNS sub-field of
// an obi.NetworkConfig. Only non-zero values override the existing field.
func (args ReverseDNS) applyToNetwork(n *obi.NetworkConfig) {
	if args.Type != "" {
		n.ReverseDNS.Type = args.Type
	}
	if args.CacheLen != 0 {
		n.ReverseDNS.CacheLen = args.CacheLen
	}
	if args.CacheTTL != 0 {
		n.ReverseDNS.CacheTTL = args.CacheTTL
	}
}

// applyToStats mirrors applyToNetwork for obi.StatsConfig (separate upstream type,
// same internal GeoIP field; the internal geoip package can't be named here).
func (args GeoIP) applyToStats(s *obi.StatsConfig) {
	if args.IPInfoPath != "" {
		s.GeoIP.IPInfo.Path = args.IPInfoPath
	}
	if args.MaxMindCountryPath != "" {
		s.GeoIP.MaxMindInfo.CountryPath = args.MaxMindCountryPath
	}
	if args.MaxMindASNPath != "" {
		s.GeoIP.MaxMindInfo.ASNPath = args.MaxMindASNPath
	}
	if args.CacheLen != 0 {
		s.GeoIP.CacheLen = args.CacheLen
	}
	if args.CacheTTL != 0 {
		s.GeoIP.CacheTTL = args.CacheTTL
	}
}

func (args ReverseDNS) applyToStats(s *obi.StatsConfig) {
	if args.Type != "" {
		s.ReverseDNS.Type = args.Type
	}
	if args.CacheLen != 0 {
		s.ReverseDNS.CacheLen = args.CacheLen
	}
	if args.CacheTTL != 0 {
		s.ReverseDNS.CacheTTL = args.CacheTTL
	}
}

func (args Network) Convert() obi.NetworkConfig {
	networks := beyla.DefaultConfig().NetworkFlows
	if args.Source != "" {
		networks.Source = args.Source
	}
	if args.AgentIP != "" {
		networks.AgentIP = args.AgentIP
	}
	if args.AgentIPIface != "" {
		networks.AgentIPIface = obi.AgentTypeIface(args.AgentIPIface)
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
	if len(args.CIDRs) > 0 {
		_ = networks.CIDRs.UnmarshalText([]byte(strings.Join(args.CIDRs, ",")))
	}
	if args.Deduper != "" {
		networks.Deduper = args.Deduper
	}
	if args.DeduperFCTTL != 0 {
		networks.DeduperFCTTL = args.DeduperFCTTL
	}
	if args.GuessPorts != "" {
		networks.GuessPorts = flowdef.PortGuessPolicy(args.GuessPorts)
	}
	if args.ListenInterfaces != "" {
		networks.ListenInterfaces = args.ListenInterfaces
	}
	if args.ListenPollPeriod != 0 {
		networks.ListenPollPeriod = args.ListenPollPeriod
	}
	networks.Print = args.PrintFlows
	args.GeoIP.applyToNetwork(&networks)
	args.ReverseDNS.applyToNetwork(&networks)
	return networks
}

func (args Stats) Convert() obi.StatsConfig {
	stats := beyla.DefaultConfig().Stats
	if args.AgentIP != "" {
		stats.AgentIP = args.AgentIP
	}
	if args.AgentIPIface != "" {
		stats.AgentIPIface = obi.AgentTypeIface(args.AgentIPIface)
	}
	if args.AgentIPType != "" {
		stats.AgentIPType = args.AgentIPType
	}
	if args.CIDRs != nil {
		if len(args.CIDRs) > 0 {
			_ = stats.CIDRs.UnmarshalText([]byte(strings.Join(args.CIDRs, ",")))
		}
	}
	stats.Print = args.Print
	args.GeoIP.applyToStats(&stats)
	args.ReverseDNS.applyToStats(&stats)
	return stats
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

	if args.WakeupLen != 0 {
		ebpf.WakeupLen = args.WakeupLen
	}
	ebpf.TrackRequestHeaders = args.TrackRequestHeaders
	ebpf.HighRequestVolume = args.HighRequestVolume
	ebpf.HeuristicSQLDetect = args.HeuristicSQLDetect
	ebpf.BpfDebug = args.BpfDebug
	ebpf.ProtocolDebug = args.ProtocolDebug
	ebpf.MapsConfig.GlobalScaleFactor = args.MapsConfig.GlobalScaleFactor
	h := args.PayloadExtraction.HTTP
	ebpf.PayloadExtraction.HTTP.GraphQL.Enabled = h.GraphQL.Enabled
	ebpf.PayloadExtraction.HTTP.Elasticsearch.Enabled = h.Elasticsearch.Enabled
	ebpf.PayloadExtraction.HTTP.AWS.Enabled = h.AWS.Enabled
	ebpf.PayloadExtraction.HTTP.JSONRPC.Enabled = h.JSONRPC.Enabled
	ebpf.PayloadExtraction.HTTP.SQLPP.Enabled = h.SQLPP.Enabled
	if h.SQLPP.EndpointPatterns != nil {
		ebpf.PayloadExtraction.HTTP.SQLPP.EndpointPatterns = h.SQLPP.EndpointPatterns
	}
	ebpf.PayloadExtraction.HTTP.GenAI.OpenAI.Enabled = h.GenAI.OpenAI.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.Anthropic.Enabled = h.GenAI.Anthropic.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.Gemini.Enabled = h.GenAI.Gemini.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.Qwen.Enabled = h.GenAI.Qwen.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.Bedrock.Enabled = h.GenAI.Bedrock.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.MCP.Enabled = h.GenAI.MCP.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.Embedding.Enabled = h.GenAI.Embedding.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.Rerank.Enabled = h.GenAI.Rerank.Enabled
	ebpf.PayloadExtraction.HTTP.GenAI.Retrieval.Enabled = h.GenAI.Retrieval.Enabled
	enrichment, err := h.Enrichment.Convert()
	if err != nil {
		return nil, err
	}
	ebpf.PayloadExtraction.HTTP.Enrichment = enrichment
	if args.InstrumentCuda != "" {
		if err := ebpf.InstrumentCuda.UnmarshalText([]byte(args.InstrumentCuda)); err != nil {
			return nil, fmt.Errorf("ebpf.instrument_cuda: %w", err)
		}
	}
	if args.TrafficControlBackend != "" {
		if err := ebpf.TCBackend.UnmarshalText([]byte(args.TrafficControlBackend)); err != nil {
			return nil, fmt.Errorf("ebpf.traffic_control_backend: %w", err)
		}
	}
	if args.MaxTransactionTime != 0 {
		ebpf.MaxTransactionTime = args.MaxTransactionTime
	}
	if args.DNSRequestTimeout != 0 {
		ebpf.DNSRequestTimeout = args.DNSRequestTimeout
	}
	ebpf.BufferSizes.HTTP = args.BufferSizes.HTTP
	ebpf.BufferSizes.MySQL = args.BufferSizes.MySQL
	ebpf.BufferSizes.Kafka = args.BufferSizes.Kafka
	ebpf.BufferSizes.Postgres = args.BufferSizes.Postgres
	ebpf.BufferSizes.MSSQL = args.BufferSizes.MSSQL
	ebpf.BufferSizes.TCP = args.BufferSizes.TCP
	return &ebpf, nil
}

func (args Enrichment) Convert() (obiCfg.EnrichmentConfig, error) {
	// Start from the default config so unset fields (e.g. the default
	// exclude/exclude policy and obfuscation string) are preserved.
	cfg := beyla.DefaultConfig().EBPF.PayloadExtraction.HTTP.Enrichment
	cfg.Enabled = args.Enabled
	if len(args.Rules) > 0 {
		cfg.Rules = make([]obiCfg.HTTPParsingRule, 0, len(args.Rules))
	}
	if args.Policy.ObfuscationString != "" {
		cfg.Policy.ObfuscationString = args.Policy.ObfuscationString
	}
	if args.Policy.DefaultAction.Headers != "" {
		if err := cfg.Policy.DefaultAction.Headers.UnmarshalText([]byte(args.Policy.DefaultAction.Headers)); err != nil {
			return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.policy.default_action.headers: %w", err)
		}
	}
	if args.Policy.DefaultAction.Body != "" {
		if err := cfg.Policy.DefaultAction.Body.UnmarshalText([]byte(args.Policy.DefaultAction.Body)); err != nil {
			return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.policy.default_action.body: %w", err)
		}
	}
	for i, r := range args.Rules {
		rule := obiCfg.HTTPParsingRule{
			Match: obiCfg.HTTPParsingMatch{
				CaseSensitive: r.Match.CaseSensitive,
			},
		}
		if r.Action != "" {
			if err := rule.Action.UnmarshalText([]byte(r.Action)); err != nil {
				return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].action: %w", i, err)
			}
		}
		if r.Type != "" {
			if err := rule.Type.UnmarshalText([]byte(r.Type)); err != nil {
				return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].type: %w", i, err)
			}
		}
		if r.Scope != "" {
			if err := rule.Scope.UnmarshalText([]byte(r.Scope)); err != nil {
				return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].scope: %w", i, err)
			}
		}
		for _, p := range r.Match.Patterns {
			g, err := stringToGlobAttr(p)
			if err != nil {
				return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].match.patterns: %w", i, err)
			}
			rule.Match.Patterns = append(rule.Match.Patterns, g)
		}
		for _, p := range r.Match.URLPathPatterns {
			g, err := stringToGlobAttr(p)
			if err != nil {
				return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].match.url_path_patterns: %w", i, err)
			}
			rule.Match.URLPathPatterns = append(rule.Match.URLPathPatterns, g)
		}
		for _, jp := range r.Match.ObfuscationJSONPaths {
			expr, err := obiCfg.NewJSONPathExpr(jp)
			if err != nil {
				return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].match.obfuscation_json_paths: %w", i, err)
			}
			rule.Match.ObfuscationJSONPaths = append(rule.Match.ObfuscationJSONPaths, expr)
		}
		for _, m := range r.Match.Methods {
			var hm obiCfg.HTTPMethod
			if err := hm.UnmarshalText([]byte(m)); err != nil {
				return cfg, fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].match.methods: %w", i, err)
			}
			rule.Match.Methods = append(rule.Match.Methods, hm)
		}
		cfg.Rules = append(cfg.Rules, rule)
	}
	return cfg, nil
}

func (args Filters) Convert() filter.AttributesConfig {
	filters := filter.AttributesConfig{
		Application: map[string]filter.MatchDefinition{},
		Network:     map[string]filter.MatchDefinition{},
	}
	for _, app := range args.Application {
		filters.Application[app.Attr] = filter.MatchDefinition{
			Match:         app.Match,
			NotMatch:      app.NotMatch,
			GreaterThan:   app.GreaterThan,
			GreaterEquals: app.GreaterEquals,
			Equals:        app.Equals,
			NotEquals:     app.NotEquals,
			LessEquals:    app.LessEquals,
			LessThan:      app.LessThan,
		}
	}
	for _, net := range args.Network {
		filters.Network[net.Attr] = filter.MatchDefinition{
			Match:         net.Match,
			NotMatch:      net.NotMatch,
			GreaterThan:   net.GreaterThan,
			GreaterEquals: net.GreaterEquals,
			Equals:        net.Equals,
			NotEquals:     net.NotEquals,
			LessEquals:    net.LessEquals,
			LessThan:      net.LessThan,
		}
	}
	return filters
}

func (args InjectorWebhook) Convert() beyla.WebhookConfig {
	w := beyla.DefaultConfig().Injector.Webhook
	w.ExternalWebhook = args.ExternalWebhook

	return w
}

func (args InjectorSDKExport) Convert() configmap.SDKExportedSignals {
	w := beyla.DefaultConfig().Injector.ExportedSignals
	if args.Traces != nil {
		w.Traces = args.Traces
	}
	if args.Metrics != nil {
		w.Metrics = args.Metrics
	}
	if args.Logs != nil {
		w.Logs = args.Logs
	}

	return w
}

func (args InjectorSDKResource) Convert() configmap.SDKResource {
	w := beyla.DefaultConfig().Injector.Resources
	if args.Attributes != nil {
		w.Attributes = args.Attributes
	}
	if args.AddK8sUIDAttributes != nil {
		w.AddK8sUIDAttributes = *args.AddK8sUIDAttributes
	}
	if args.AddK8sIPAttribute != nil {
		w.AddK8sIPAttribute = *args.AddK8sIPAttribute
	}
	if args.UseLabelsForResourceAttributes != nil {
		w.UseLabelsForResourceAttributes = *args.UseLabelsForResourceAttributes
	}

	return w
}

func selectorFromGlob(a *services.GlobAttributes) configmap.K8sSelector {
	var podLabels map[string]services.GlobAttr
	if len(a.PodLabels) > 0 {
		podLabels = make(map[string]services.GlobAttr, len(a.PodLabels))
		for k, v := range a.PodLabels {
			podLabels[k] = *v
		}
	}

	var podAnnotations map[string]services.GlobAttr
	if len(a.PodAnnotations) > 0 {
		podAnnotations = make(map[string]services.GlobAttr, len(a.PodAnnotations))
		for k, v := range a.PodAnnotations {
			podAnnotations[k] = *v
		}
	}

	metaGlob := func(name string) []services.GlobAttr {
		if g := a.Metadata[name]; g != nil {
			return []services.GlobAttr{*g}
		}
		return nil
	}

	// First check to see if the user used k8s_owner_name
	ownerNames := metaGlob(services.AttrOwnerName)
	var kinds []string
	// If no owner name, then we check the specific types of definitions.
	// In this case we set both the owner name and the kind to match the new
	// service definition format.
	if ownerNames == nil {
		for _, owner := range []struct {
			metadataKey string
			kind        string
		}{
			{metadataKey: services.AttrDeploymentName, kind: "Deployment"},
			{metadataKey: services.AttrDaemonSetName, kind: "DaemonSet"},
			{metadataKey: services.AttrReplicaSetName, kind: "ReplicaSet"},
			{metadataKey: services.AttrStatefulSetName, kind: "StatefulSet"},
			{metadataKey: services.AttrJobName, kind: "Job"},
			{metadataKey: services.AttrCronJobName, kind: "CronJob"},
			{metadataKey: services.AttrPodName, kind: "Pod"},
		} {
			if names := metaGlob(owner.metadataKey); names != nil {
				ownerNames = names
				kinds = []string{owner.kind}
				break
			}
		}
	}

	return configmap.K8sSelector{
		Namespaces:     metaGlob(services.AttrNamespace),
		OwnerNames:     ownerNames,
		OwnerKinds:     kinds,
		PodLabels:      podLabels,
		PodAnnotations: podAnnotations,
	}
}

func selectorsFromInstrument(g services.GlobDefinitionCriteria) []configmap.K8sSelector {
	var selectors []configmap.K8sSelector

	for i := range g {
		sel := selectorFromGlob(&g[i])
		if sel.IsEmpty() {
			continue
		}

		selectors = append(selectors, sel)
	}

	return selectors
}

func (args Injector) Convert() (beyla.SDKInject, error) {
	i := beyla.DefaultConfig().Injector

	if len(args.Instrument) > 0 {
		instrument, err := args.Instrument.ConvertGlob()
		if err != nil {
			return i, err
		}
		i.Instrument = selectorsFromInstrument(instrument)
	}

	if len(args.ExcludeInstrument) > 0 {
		exclude, err := args.ExcludeInstrument.ConvertGlob()
		if err != nil {
			return i, err
		}
		i.ExcludeInstrument = selectorsFromInstrument(exclude)
	}

	i.Webhook = args.Webhook.Convert()
	i.ExportedSignals = args.ExportedSignals.Convert()
	i.Resources = args.Resources.Convert()

	if args.DefaultSampler.Name != "" || args.DefaultSampler.Arg != "" {
		s := args.DefaultSampler.Convert()
		i.DefaultSampler = &s
	}

	if args.ImageVersion != "" {
		i.ImageVersion = args.ImageVersion
	}

	if len(args.Propagators) > 0 {
		i.Propagators = args.Propagators
	}

	if len(args.EnabledSDKs) > 0 {
		sdks := make([]beylaSvc.InstrumentableType, 0, len(args.EnabledSDKs))
		for _, s := range args.EnabledSDKs {
			var it beylaSvc.InstrumentableType
			if err := it.UnmarshalText([]byte(s)); err != nil {
				return i, fmt.Errorf("injector.enabled_sdks: %w", err)
			}
			sdks = append(sdks, it)
		}
		i.EnabledSDKs = sdks
	}

	return i, nil
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
		c.opts.SLogger.Warn("The 'open_port' field is deprecated. Use 'discovery.services' instead.")
	}
	if c.args.ExecutableName != "" {
		c.opts.SLogger.Warn("The 'executable_name' field is deprecated. Use 'discovery.services' instead.")
	}

	// Add deprecation warnings for legacy discovery fields
	if len(c.args.Discovery.Services) > 0 {
		c.opts.SLogger.Warn("discovery.services is deprecated, use discovery.instrument instead")
	}
	if len(c.args.Discovery.ExcludeServices) > 0 {
		c.opts.SLogger.Warn("discovery.exclude_services is deprecated, use discovery.exclude_instrument instead")
	}
	if len(c.args.Discovery.DefaultExcludeServices) > 0 {
		c.opts.SLogger.Warn("discovery.default_exclude_services is deprecated, use discovery.default_exclude_instrument instead")
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
				c.opts.SLogger.Info("waiting for Beyla to terminate")
				if err := cancelG.Wait(); err != nil {
					c.opts.SLogger.Error("Beyla terminated with error", "err", err)
					c.reportUnhealthy(err)
				}
			}

			c.opts.SLogger.Info("starting Beyla component")

			newCtx, cancelFunc := context.WithCancel(ctx)
			cancel = cancelFunc

			cfg, err := c.loadConfig()
			if err != nil {
				c.opts.SLogger.Error("failed to load config", "err", err)
				c.reportUnhealthy(err)
				continue
			}

			g, launchCtx := errgroup.WithContext(newCtx)
			cancelG = g

			g.Go(func() error {
				err := components.RunBeyla(launchCtx, cfg)
				if err != nil {
					c.opts.SLogger.Error("failed to run Beyla", "err", err)
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
	c.mut.Lock()
	nativeHistograms := c.args.Metrics.NativeHistograms
	c.mut.Unlock()
	return promhttp.HandlerFor(c.reg, promhttp.HandlerOpts{EnableOpenMetrics: nativeHistograms})
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
	if a.Metrics.Features != nil {
		cfg.Metrics.Features = export.LoadFeatures(a.Metrics.Features)
	}
	cfg.NetworkFlows = a.Metrics.Network.Convert()
	cfg.Stats = a.Stats.Convert()
	cfg.EnforceSysCaps = a.EnforceSysCaps

	ebpf, err := a.EBPF.Convert()
	if err != nil {
		return nil, err
	}
	cfg.EBPF = *ebpf

	cfg.Filters = a.Filters.Convert()
	cfg.TracePrinter = debug.TracePrinter(a.TracePrinter)

	cfg.Injector, err = a.Injector.Convert()
	if err != nil {
		return nil, err
	}

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

	switch args.EBPF.InstrumentCuda {
	case "", "auto", "on", "off":
	default:
		return fmt.Errorf("ebpf.instrument_cuda: invalid value %q (valid: auto, on, off)", args.EBPF.InstrumentCuda)
	}
	switch args.EBPF.TrafficControlBackend {
	case "", "auto", "tc", "tcx":
	default:
		return fmt.Errorf("ebpf.traffic_control_backend: invalid value %q (valid: auto, tc, tcx)", args.EBPF.TrafficControlBackend)
	}
	for name, v := range map[string]uint32{
		"http":     args.EBPF.BufferSizes.HTTP,
		"mysql":    args.EBPF.BufferSizes.MySQL,
		"kafka":    args.EBPF.BufferSizes.Kafka,
		"postgres": args.EBPF.BufferSizes.Postgres,
		"mssql":    args.EBPF.BufferSizes.MSSQL,
		"tcp":      args.EBPF.BufferSizes.TCP,
	} {
		if v > 65536 {
			return fmt.Errorf("ebpf.buffer_sizes.%s: must be <= 65536, got %d", name, v)
		}
	}

	validAction := map[string]struct{}{"": {}, "include": {}, "exclude": {}, "obfuscate": {}}
	en := args.EBPF.PayloadExtraction.HTTP.Enrichment
	if _, ok := validAction[en.Policy.DefaultAction.Headers]; !ok {
		return fmt.Errorf("ebpf.payload_extraction.http.enrichment.policy.default_action.headers: invalid value %q", en.Policy.DefaultAction.Headers)
	}
	if _, ok := validAction[en.Policy.DefaultAction.Body]; !ok {
		return fmt.Errorf("ebpf.payload_extraction.http.enrichment.policy.default_action.body: invalid value %q", en.Policy.DefaultAction.Body)
	}
	validType := map[string]struct{}{"": {}, "headers": {}, "body": {}}
	validScope := map[string]struct{}{"": {}, "request": {}, "response": {}, "all": {}}
	for i, r := range en.Rules {
		if _, ok := validAction[r.Action]; !ok {
			return fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].action: invalid value %q", i, r.Action)
		}
		if _, ok := validType[r.Type]; !ok {
			return fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].type: invalid value %q", i, r.Type)
		}
		if _, ok := validScope[r.Scope]; !ok {
			return fmt.Errorf("ebpf.payload_extraction.http.enrichment.rule[%d].scope: invalid value %q", i, r.Scope)
		}
	}

	if err := args.Metrics.Validate(); err != nil {
		return err
	}

	switch args.Metrics.Network.Deduper {
	case "", none, "first_come":
	default:
		return fmt.Errorf("metrics.network.deduper: invalid value %q (valid: none, first_come)", args.Metrics.Network.Deduper)
	}
	switch args.Metrics.Network.GuessPorts {
	case "", "ordinal", "disable":
	default:
		return fmt.Errorf("metrics.network.guess_ports: invalid value %q (valid: ordinal, disable)", args.Metrics.Network.GuessPorts)
	}
	switch args.Metrics.Network.ListenInterfaces {
	case "", "watch", "poll":
	default:
		return fmt.Errorf("metrics.network.listen_interfaces: invalid value %q (valid: watch, poll)", args.Metrics.Network.ListenInterfaces)
	}
	switch args.Metrics.Network.ReverseDNS.Type {
	case "", none, "local", "ebpf":
	default:
		return fmt.Errorf("metrics.network.reverse_dns.type: invalid value %q (valid: none, local, ebpf)", args.Metrics.Network.ReverseDNS.Type)
	}
	switch args.Stats.ReverseDNS.Type {
	case "", none, "local", "ebpf":
	default:
		return fmt.Errorf("stats.reverse_dns.type: invalid value %q (valid: none, local, ebpf)", args.Stats.ReverseDNS.Type)
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
		config.Instrumentations = []instrumentations.Instrumentation{
			instrumentations.InstrumentationALL,
		}
	} else {
		config.Instrumentations = stringsToInstrumentations(args.Instrumentations)
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

func stringToPortEnum(s string) (services.IntEnum, error) {
	if s == "" {
		return services.IntEnum{}, nil
	}
	p := services.IntEnum{}
	err := p.UnmarshalText([]byte(s))
	if err != nil {
		return services.IntEnum{}, err
	}
	return p, nil
}

func stringsToInstrumentations(ss []string) []instrumentations.Instrumentation {
	result := make([]instrumentations.Instrumentation, len(ss))
	for i, s := range ss {
		result[i] = instrumentations.Instrumentation(s)
	}
	return result
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

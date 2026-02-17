package beyla

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
)

// Arguments configures the Beyla component.
type Arguments struct {
	// Deprecated: Use discovery.services instead.
	Port string `alloy:"open_port,attr,optional"`
	// Deprecated: Use discovery.services instead.
	ExecutableName string                     `alloy:"executable_name,attr,optional"`
	Debug          bool                       `alloy:"debug,attr,optional"`
	TracePrinter   string                     `alloy:"trace_printer,attr,optional"`
	EnforceSysCaps bool                       `alloy:"enforce_sys_caps,attr,optional"`
	Routes         Routes                     `alloy:"routes,block,optional"`
	Attributes     Attributes                 `alloy:"attributes,block,optional"`
	Discovery      Discovery                  `alloy:"discovery,block,optional"`
	Metrics        Metrics                    `alloy:"metrics,block,optional"`
	Traces         Traces                     `alloy:"traces,block,optional"`
	EBPF           EBPF                       `alloy:"ebpf,block,optional"`
	Filters        Filters                    `alloy:"filters,block,optional"`
	Output         *otelcol.ConsumerArguments `alloy:"output,block,optional"`
}

type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type Routes struct {
	Unmatch                   string   `alloy:"unmatched,attr,optional"`
	Patterns                  []string `alloy:"patterns,attr,optional"`
	IgnorePatterns            []string `alloy:"ignored_patterns,attr,optional"`
	IgnoredEvents             string   `alloy:"ignore_mode,attr,optional"`
	WildcardChar              string   `alloy:"wildcard_char,attr,optional"`
	MaxPathSegmentCardinality int      `alloy:"max_path_segment_cardinality,attr,optional"`
}

type Attributes struct {
	Kubernetes KubernetesDecorator `alloy:"kubernetes,block"`
	InstanceID InstanceIDConfig    `alloy:"instance_id,block,optional"`
	Select     Selections          `alloy:"select,block,optional"`
}

type KubernetesDecorator struct {
	Enable                string        `alloy:"enable,attr"`
	ClusterName           string        `alloy:"cluster_name,attr,optional"`
	InformersSyncTimeout  time.Duration `alloy:"informers_sync_timeout,attr,optional"`
	InformersResyncPeriod time.Duration `alloy:"informers_resync_period,attr,optional"`
	DisableInformers      []string      `alloy:"disable_informers,attr,optional"`
	MetaRestrictLocalNode bool          `alloy:"meta_restrict_local_node,attr,optional"`
	MetaCacheAddress      string        `alloy:"meta_cache_address,attr,optional"`
}

type InstanceIDConfig struct {
	HostnameDNSResolution bool   `alloy:"dns,attr,optional"`
	OverrideHostname      string `alloy:"override_hostname,attr,optional"`
}

type Selections []Selection

type Selection struct {
	Section string   `alloy:"attr,attr"`
	Include []string `alloy:"include,attr,optional"`
	Exclude []string `alloy:"exclude,attr,optional"`
}

type Services []Service

type SamplerConfig struct {
	Name string `alloy:"name,attr,optional"`
	Arg  string `alloy:"arg,attr,optional"`
}

type Service struct {
	Name           string            `alloy:"name,attr,optional"`
	Namespace      string            `alloy:"namespace,attr,optional"`
	OpenPorts      string            `alloy:"open_ports,attr,optional"`
	Path           string            `alloy:"exe_path,attr,optional"`
	Kubernetes     KubernetesService `alloy:"kubernetes,block,optional"`
	ContainersOnly bool              `alloy:"containers_only,attr,optional"`
	ExportModes    []string          `alloy:"exports,attr,optional"`
	Sampler        SamplerConfig     `alloy:"sampler,block,optional"`
}

type KubernetesService struct {
	Namespace       string            `alloy:"namespace,attr,optional"`
	PodName         string            `alloy:"pod_name,attr,optional"`
	DeploymentName  string            `alloy:"deployment_name,attr,optional"`
	ReplicaSetName  string            `alloy:"replicaset_name,attr,optional"`
	StatefulSetName string            `alloy:"statefulset_name,attr,optional"`
	DaemonSetName   string            `alloy:"daemonset_name,attr,optional"`
	OwnerName       string            `alloy:"owner_name,attr,optional"`
	PodLabels       map[string]string `alloy:"pod_labels,attr,optional"`
	PodAnnotations  map[string]string `alloy:"pod_annotations,attr,optional"`
}

type Discovery struct {
	// Deprecated: Use discovery.instrument instead
	Services Services `alloy:"services,block,optional"`
	// Deprecated: Use discovery.exlcude_instrument instead
	ExcludeServices Services `alloy:"exclude_services,block,optional"`
	// Deprecated: Use discovery.default_exclude_instrument instead
	DefaultExcludeServices   Services `alloy:"default_exclude_services,block,optional"`
	Survey                   Services `alloy:"survey,block,optional"`
	Instrument               Services `alloy:"instrument,block,optional"`
	ExcludeInstrument        Services `alloy:"exclude_instrument,block,optional"`
	DefaultExcludeInstrument Services `alloy:"default_exclude_instrument,block,optional"`

	SkipGoSpecificTracers           bool `alloy:"skip_go_specific_tracers,attr,optional"`
	ExcludeOTelInstrumentedServices bool `alloy:"exclude_otel_instrumented_services,attr,optional"`
}

type Metrics struct {
	Features                        []string `alloy:"features,attr,optional"`
	Instrumentations                []string `alloy:"instrumentations,attr,optional"`
	AllowServiceGraphSelfReferences bool     `alloy:"allow_service_graph_self_references,attr,optional"`
	Network                         Network  `alloy:"network,block,optional"`
	ExtraResourceLabels             []string `alloy:"extra_resource_labels,attr,optional"`
	ExtraSpanResourceLabels         []string `alloy:"extra_span_resource_labels,attr,optional"`
}

type Traces struct {
	Instrumentations []string      `alloy:"instrumentations,attr,optional"`
	Sampler          SamplerConfig `alloy:"sampler,block,optional"`
}

type Network struct {
	// Deprecated: Add "network" to metrics.features instead
	Enable             bool          `alloy:"enable,attr,optional"`
	Source             string        `alloy:"source,attr,optional"`
	AgentIP            string        `alloy:"agent_ip,attr,optional"`
	AgentIPIface       string        `alloy:"agent_ip_iface,attr,optional"`
	AgentIPType        string        `alloy:"agent_ip_type,attr,optional"`
	Interfaces         []string      `alloy:"interfaces,attr,optional"`
	ExcludeInterfaces  []string      `alloy:"exclude_interfaces,attr,optional"`
	Protocols          []string      `alloy:"protocols,attr,optional"`
	ExcludeProtocols   []string      `alloy:"exclude_protocols,attr,optional"`
	CacheMaxFlows      int           `alloy:"cache_max_flows,attr,optional"`
	CacheActiveTimeout time.Duration `alloy:"cache_active_timeout,attr,optional"`
	Direction          string        `alloy:"direction,attr,optional"`
	Sampling           int           `alloy:"sampling,attr,optional"`
	CIDRs              []string      `alloy:"cidrs,attr,optional"`
}

type EBPF struct {
	WakeupLen           int           `alloy:"wakeup_len,attr,optional"`
	TrackRequestHeaders bool          `alloy:"track_request_headers,attr,optional"`
	HTTPRequestTimeout  time.Duration `alloy:"http_request_timeout,attr,optional"`
	ContextPropagation  string        `alloy:"context_propagation,attr,optional"`
	HighRequestVolume   bool          `alloy:"high_request_volume,attr,optional"`
	HeuristicSQLDetect  bool          `alloy:"heuristic_sql_detect,attr,optional"`
	BpfDebug            bool          `alloy:"bpf_debug,attr,optional"`
	ProtocolDebug       bool          `alloy:"protocol_debug_print,attr,optional"`
}

type Filters struct {
	Application AttributeFamilies `alloy:"application,block,optional"`
	Network     AttributeFamilies `alloy:"network,block,optional"`
}

type AttributeFamilies []AttributeFamily

type AttributeFamily struct {
	Attr     string `alloy:"attr,attr"`
	Match    string `alloy:"match,attr,optional"`
	NotMatch string `alloy:"not_match,attr,optional"`
}

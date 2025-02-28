package beyla

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
)

// Arguments configures the Beyla component.
type Arguments struct {
	Port           string                     `alloy:"open_port,attr,optional"`
	ExecutableName string                     `alloy:"executable_name,attr,optional"`
	Debug          bool                       `alloy:"debug,attr,optional"`
	EnforceSysCaps bool                       `alloy:"enforce_sys_caps,attr,optional"`
	Routes         Routes                     `alloy:"routes,block,optional"`
	Attributes     Attributes                 `alloy:"attributes,block,optional"`
	Discovery      Discovery                  `alloy:"discovery,block,optional"`
	Metrics        Metrics                    `alloy:"metrics,block,optional"`
	EBPF           EBPF                       `alloy:"ebpf,block,optional"`
	Filters        Filters                    `alloy:"filters,block,optional"`
	Output         *otelcol.ConsumerArguments `alloy:"output,block,optional"`
}

type Exports struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type Routes struct {
	Unmatch        string   `alloy:"unmatched,attr,optional"`
	Patterns       []string `alloy:"patterns,attr,optional"`
	IgnorePatterns []string `alloy:"ignored_patterns,attr,optional"`
	IgnoredEvents  string   `alloy:"ignore_mode,attr,optional"`
	WildcardChar   string   `alloy:"wildcard_char,attr,optional"`
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
}

type InstanceIDConfig struct {
	HostnameDNSResolution bool   `alloy:"dns,attr,optional"`
	OverrideHostname      string `alloy:"override_hostname,attr,optional"`
}

type Selections []Selection

type Selection struct {
	Section string   `alloy:",label"`
	Include []string `alloy:"include,attr"`
	Exclude []string `alloy:"exclude,attr"`
}

type Services []Service

// NOTE(@tpaschalis) Used for both Services and Survey
type Service struct {
	Name           string            `alloy:"name,attr,optional"`
	Namespace      string            `alloy:"namespace,attr,optional"`
	OpenPorts      string            `alloy:"open_ports,attr,optional"`
	Path           string            `alloy:"exe_path,attr,optional"`
	ContainersOnly bool              `alloy:"containers_only,attr,optional"`
	Kubernetes     KubernetesService `alloy:"kubernetes,block,optional"`
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
}

type Discovery struct {
	Services                        Services `alloy:"services,block"`
	Survey                          Services `alloy:"survey,block"`
	ExcludeServices                 Services `alloy:"exclude_services,block,optional"`
	ExcludeSurvey                   Services `alloy:"exclude_survey,block,optional"`
	DefaultExcludeServices          Services `alloy:"default_exclude_services,block,optional"`
	DefaultExcludeSurvey            Services `alloy:"default_exclude_survey,block,optional"`
	SkipGoSpecificTracers           bool     `alloy:"skip_go_specific_tracers,attr,optional"`
	ExcludeOTelInstrumentedServices bool     `alloy:"exclude_otel_instrumented_services,attr,optional"`
}

type Metrics struct {
	Features                        []string `alloy:"features,attr,optional"`
	Instrumentations                []string `alloy:"instrumentations,attr,optional"`
	AllowServiceGraphSelfReferences bool     `alloy:"allow_service_graph_self_references,attr,optional"`
	Network                         Network  `alloy:"network,block,optional"`
}

type Network struct {
	Enable             bool          `alloy:"enable,attr"`
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
	WakeupLen                 int           `alloy:"wakeup_len,attr,optional"`
	TrackRequestHeaders       bool          `alloy:"track_request_headers,attr,optional"`
	HTTPRequestTimeout        time.Duration `alloy:"http_request_timeout,attr,optional"`
	ContextPropagationEnabled bool          `alloy:"enable_context_propagation,attr,optional"`
	HighRequestVolume         bool          `alloy:"high_request_volume,attr,optional"`
	HeuristicSQLDetect        bool          `alloy:"heuristic_sql_detect,attr,optional"`
}

type Filters struct {
	Application AttributeFamilies `alloy:"application,block,optional"`
	Network     AttributeFamilies `alloy:"network,block,optional"`
}

type AttributeFamilies []AttributeFamily

type AttributeFamily struct {
	Attr     string `alloy:",label"`
	Match    string `alloy:"match,attr,optional"`
	NotMatch string `alloy:"not_match,attr,optional"`
}

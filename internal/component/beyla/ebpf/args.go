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
	Injector       Injector                   `alloy:"injector,block,optional"`
	Stats          Stats                      `alloy:"stats,block,optional"`
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

	RenameUnresolvedHosts         *string       `alloy:"rename_unresolved_hosts,attr,optional"`
	RenameUnresolvedHostsOutgoing *string       `alloy:"rename_unresolved_hosts_outgoing,attr,optional"`
	RenameUnresolvedHostsIncoming *string       `alloy:"rename_unresolved_hosts_incoming,attr,optional"`
	MetricSpanNamesLimit          int           `alloy:"metric_span_names_limit,attr,optional"`
	HostID                        HostIDConfig  `alloy:"host_id,block,optional"`
	MetadataRetry                 MetadataRetry `alloy:"metadata_retry,block,optional"`
}

type HostIDConfig struct {
	Override string `alloy:"override,attr,optional"`
}

type MetadataRetry struct {
	Timeout       time.Duration `alloy:"timeout,attr,optional"`
	StartInterval time.Duration `alloy:"start_interval,attr,optional"`
	MaxInterval   time.Duration `alloy:"max_interval,attr,optional"`
}

type KubernetesDecorator struct {
	Enable                string        `alloy:"enable,attr"`
	ClusterName           string        `alloy:"cluster_name,attr,optional"`
	InformersSyncTimeout  time.Duration `alloy:"informers_sync_timeout,attr,optional"`
	InformersResyncPeriod time.Duration `alloy:"informers_resync_period,attr,optional"`
	DisableInformers      []string      `alloy:"disable_informers,attr,optional"`
	MetaRestrictLocalNode bool          `alloy:"meta_restrict_local_node,attr,optional"`
	MetaCacheAddress      string        `alloy:"meta_cache_address,attr,optional"`

	KubeconfigPath           string              `alloy:"kubeconfig_path,attr,optional"`
	ReconnectInitialInterval time.Duration       `alloy:"reconnect_initial_interval,attr,optional"`
	DropExternal             bool                `alloy:"drop_external,attr,optional"`
	ServiceNameTemplate      string              `alloy:"service_name_template,attr,optional"`
	ResourceLabels           map[string][]string `alloy:"resource_labels,attr,optional"`
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
	CmdArgs        string            `alloy:"cmd_args,attr,optional"`
	Languages      string            `alloy:"languages,attr,optional"`
	PIDs           []uint32          `alloy:"target_pids,attr,optional"`
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
	// Deprecated: Use discovery.exclude_instrument instead
	ExcludeServices Services `alloy:"exclude_services,block,optional"`
	// Deprecated: Use discovery.default_exclude_instrument instead
	DefaultExcludeServices   Services `alloy:"default_exclude_services,block,optional"`
	Survey                   Services `alloy:"survey,block,optional"`
	Instrument               Services `alloy:"instrument,block,optional"`
	ExcludeInstrument        Services `alloy:"exclude_instrument,block,optional"`
	DefaultExcludeInstrument Services `alloy:"default_exclude_instrument,block,optional"`

	SkipGoSpecificTracers           bool `alloy:"skip_go_specific_tracers,attr,optional"`
	ExcludeOTelInstrumentedServices bool `alloy:"exclude_otel_instrumented_services,attr,optional"`

	PollInterval                               time.Duration `alloy:"poll_interval,attr,optional"`
	MinProcessAge                              time.Duration `alloy:"min_process_age,attr,optional"`
	DefaultOtlpGRPCPort                        int           `alloy:"default_otlp_grpc_port,attr,optional"`
	ExcludeOTelInstrumentedServicesSpanMetrics bool          `alloy:"exclude_otel_instrumented_services_span_metrics,attr,optional"`
}

type Metrics struct {
	Features                        []string        `alloy:"features,attr,optional"`
	Instrumentations                []string        `alloy:"instrumentations,attr,optional"`
	AllowServiceGraphSelfReferences bool            `alloy:"allow_service_graph_self_references,attr,optional"`
	Network                         Network         `alloy:"network,block,optional"`
	ExtraResourceLabels             []string        `alloy:"extra_resource_labels,attr,optional"`
	ExtraSpanResourceLabels         []string        `alloy:"extra_span_resource_labels,attr,optional"`
	NativeHistograms                bool            `alloy:"native_histograms,attr,optional"`
	ExemplarFilter                  string          `alloy:"exemplar_filter,attr,optional"`
	TTL                             time.Duration   `alloy:"ttl,attr,optional"`
	SpanServiceCacheSize            int             `alloy:"span_service_cache_size,attr,optional"`
	NativeHistogram                 NativeHistogram `alloy:"native_histogram,block,optional"`
	Buckets                         Buckets         `alloy:"buckets,block,optional"`
}

type NativeHistogram struct {
	BucketFactor     float64       `alloy:"bucket_factor,attr,optional"`
	MaxBucketNumber  uint32        `alloy:"max_bucket_number,attr,optional"`
	MinResetDuration time.Duration `alloy:"min_reset_duration,attr,optional"`
}

type Buckets struct {
	DurationHistogram            []float64 `alloy:"duration_histogram,attr,optional"`
	RequestSizeHistogram         []float64 `alloy:"request_size_histogram,attr,optional"`
	ResponseSizeHistogram        []float64 `alloy:"response_size_histogram,attr,optional"`
	GenAITokenUsageHistogram     []float64 `alloy:"gen_ai_token_usage_histogram,attr,optional"`
	GenAIClientDurationHistogram []float64 `alloy:"gen_ai_client_duration_histogram,attr,optional"`
	StatTCPRttHistogram          []float64 `alloy:"stat_tcp_rtt_histogram,attr,optional"`
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
	Deduper            string        `alloy:"deduper,attr,optional"`
	DeduperFCTTL       time.Duration `alloy:"deduper_fc_ttl,attr,optional"`
	GuessPorts         string        `alloy:"guess_ports,attr,optional"`
	ListenInterfaces   string        `alloy:"listen_interfaces,attr,optional"`
	ListenPollPeriod   time.Duration `alloy:"listen_poll_period,attr,optional"`
	PrintFlows         bool          `alloy:"print_flows,attr,optional"`
	GeoIP              GeoIP         `alloy:"geo_ip,block,optional"`
	ReverseDNS         ReverseDNS    `alloy:"reverse_dns,block,optional"`
}

type EBPFMapsConfig struct {
	GlobalScaleFactor int `alloy:"global_scale_factor,attr,optional"`
}

type EBPFMapsConfig struct {
	GlobalScaleFactor int `alloy:"global_scale_factor,attr,optional"`
}

type EBPF struct {
	WakeupLen             int               `alloy:"wakeup_len,attr,optional"`
	TrackRequestHeaders   bool              `alloy:"track_request_headers,attr,optional"`
	HTTPRequestTimeout    time.Duration     `alloy:"http_request_timeout,attr,optional"`
	ContextPropagation    string            `alloy:"context_propagation,attr,optional"`
	HighRequestVolume     bool              `alloy:"high_request_volume,attr,optional"`
	HeuristicSQLDetect    bool              `alloy:"heuristic_sql_detect,attr,optional"`
	BpfDebug              bool              `alloy:"bpf_debug,attr,optional"`
	ProtocolDebug         bool              `alloy:"protocol_debug_print,attr,optional"`
	PayloadExtraction     PayloadExtraction `alloy:"payload_extraction,block,optional"`
	MapsConfig            EBPFMapsConfig    `alloy:"maps_config,block,optional"`
	InstrumentCuda        string            `alloy:"instrument_cuda,attr,optional"`
	TrafficControlBackend string            `alloy:"traffic_control_backend,attr,optional"`
	MaxTransactionTime    time.Duration     `alloy:"max_transaction_time,attr,optional"`
	DNSRequestTimeout     time.Duration     `alloy:"dns_request_timeout,attr,optional"`
	BufferSizes           BufferSizes       `alloy:"buffer_sizes,block,optional"`
}

type BufferSizes struct {
	HTTP     uint32 `alloy:"http,attr,optional"`
	MySQL    uint32 `alloy:"mysql,attr,optional"`
	Kafka    uint32 `alloy:"kafka,attr,optional"`
	Postgres uint32 `alloy:"postgres,attr,optional"`
	MSSQL    uint32 `alloy:"mssql,attr,optional"`
	TCP      uint32 `alloy:"tcp,attr,optional"`
}

type PayloadExtraction struct {
	HTTP HTTPPayloadExtraction `alloy:"http,block,optional"`
}

type HTTPPayloadExtraction struct {
	GraphQL       ProtocolToggle `alloy:"graphql,block,optional"`
	Elasticsearch ProtocolToggle `alloy:"elasticsearch,block,optional"`
	AWS           ProtocolToggle `alloy:"aws,block,optional"`
	JSONRPC       ProtocolToggle `alloy:"jsonrpc,block,optional"`
	SQLPP         SQLPP          `alloy:"sqlpp,block,optional"`
	GenAI         GenAI          `alloy:"genai,block,optional"`
	Enrichment    Enrichment     `alloy:"enrichment,block,optional"`
}

type ProtocolToggle struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

type SQLPP struct {
	Enabled          bool     `alloy:"enabled,attr,optional"`
	EndpointPatterns []string `alloy:"endpoint_patterns,attr,optional"`
}

type GenAI struct {
	OpenAI    ProtocolToggle `alloy:"openai,block,optional"`
	Anthropic ProtocolToggle `alloy:"anthropic,block,optional"`
	Gemini    ProtocolToggle `alloy:"gemini,block,optional"`
	Qwen      ProtocolToggle `alloy:"qwen,block,optional"`
	Bedrock   ProtocolToggle `alloy:"bedrock,block,optional"`
	MCP       ProtocolToggle `alloy:"mcp,block,optional"`
	Embedding ProtocolToggle `alloy:"embedding,block,optional"`
	Rerank    ProtocolToggle `alloy:"rerank,block,optional"`
	Retrieval ProtocolToggle `alloy:"retrieval,block,optional"`
}

type Enrichment struct {
	Enabled bool             `alloy:"enabled,attr,optional"`
	Policy  EnrichmentPolicy `alloy:"policy,block,optional"`
	Rules   []EnrichmentRule `alloy:"rule,block,optional"`
}

type EnrichmentPolicy struct {
	DefaultAction     EnrichmentDefaultAction `alloy:"default_action,block,optional"`
	ObfuscationString string                  `alloy:"obfuscation_string,attr,optional"`
}

type EnrichmentDefaultAction struct {
	Headers string `alloy:"headers,attr,optional"`
	Body    string `alloy:"body,attr,optional"`
}

type EnrichmentRule struct {
	Action string          `alloy:"action,attr,optional"`
	Type   string          `alloy:"type,attr,optional"`
	Scope  string          `alloy:"scope,attr,optional"`
	Match  EnrichmentMatch `alloy:"match,block,optional"`
}

type EnrichmentMatch struct {
	Patterns             []string `alloy:"patterns,attr,optional"`
	CaseSensitive        bool     `alloy:"case_sensitive,attr,optional"`
	ObfuscationJSONPaths []string `alloy:"obfuscation_json_paths,attr,optional"`
	URLPathPatterns      []string `alloy:"url_path_patterns,attr,optional"`
	Methods              []string `alloy:"methods,attr,optional"`
}

type Filters struct {
	Application AttributeFamilies `alloy:"application,block,optional"`
	Network     AttributeFamilies `alloy:"network,block,optional"`
}

type Injector struct {
	Instrument        Services            `alloy:"instrument,block,optional"`
	ExcludeInstrument Services            `alloy:"exclude_instrument,block,optional"`
	Webhook           InjectorWebhook     `alloy:"webhook,block,optional"`
	ImageVersion      string              `alloy:"image_version,attr,optional"`
	DefaultSampler    SamplerConfig       `alloy:"sampler,block,optional"`
	Propagators       []string            `alloy:"propagators,attr,optional"`
	ExportedSignals   InjectorSDKExport   `alloy:"otel_exported_signals,block,optional"`
	Resources         InjectorSDKResource `alloy:"resources,block,optional"`
	EnabledSDKs       []string            `alloy:"enabled_sdks,attr,optional"`
}

type InjectorWebhook struct {
	ExternalWebhook string `alloy:"external_deployment_name,attr,optional"`
}

type InjectorSDKExport struct {
	Traces  *bool `alloy:"traces,attr,optional"`
	Metrics *bool `alloy:"metrics,attr,optional"`
	Logs    *bool `alloy:"logs,attr,optional"`
}

type InjectorSDKResource struct {
	// Attributes defines attributes that are added to the resource.
	// For example environment: dev
	Attributes map[string]string `alloy:"attributes,attr,optional"`
	// AddK8sUIDAttributes defines whether K8s UID attr should be collected (e.g. k8s.deployment.uid).
	AddK8sUIDAttributes *bool `alloy:"add_k8s_uid_attributes,attr,optional"`
	// AddK8sIPAttribute defines whether the k8s.pod.ip resource attribute should be set
	// from the Kubernetes downward API (status.podIP). Useful for environments where the
	// OTel k8sattributesprocessor cannot infer the pod IP from the connection source
	// (e.g. clusters behind a NAT gateway).
	// +optional
	AddK8sIPAttribute *bool `alloy:"add_k8s_ip_attribute,attr,optional"`
	// UseLabelsForResourceAttributes defines whether to use common labels for resource attributes:
	// Note: first entry wins:
	//   - `app.kubernetes.io/instance` becomes `service.name`
	//   - `app.kubernetes.io/name` becomes `service.name`
	//   - `app.kubernetes.io/version` becomes `service.version`
	UseLabelsForResourceAttributes *bool `alloy:"use_k8s_labels_for_resource_attributes,attr,optional"`
}

type Stats struct {
	AgentIP      string     `alloy:"agent_ip,attr,optional"`
	AgentIPIface string     `alloy:"agent_ip_iface,attr,optional"`
	AgentIPType  string     `alloy:"agent_ip_type,attr,optional"`
	CIDRs        []string   `alloy:"cidrs,attr,optional"`
	Print        bool       `alloy:"print_stats,attr,optional"`
	GeoIP        GeoIP      `alloy:"geo_ip,block,optional"`
	ReverseDNS   ReverseDNS `alloy:"reverse_dns,block,optional"`
}

type GeoIP struct {
	IPInfoPath         string        `alloy:"ipinfo_path,attr,optional"`
	MaxMindCountryPath string        `alloy:"maxmind_country_path,attr,optional"`
	MaxMindASNPath     string        `alloy:"maxmind_asn_path,attr,optional"`
	CacheLen           int           `alloy:"cache_len,attr,optional"`
	CacheTTL           time.Duration `alloy:"cache_ttl,attr,optional"`
}

type ReverseDNS struct {
	Type     string        `alloy:"type,attr,optional"`
	CacheLen int           `alloy:"cache_len,attr,optional"`
	CacheTTL time.Duration `alloy:"cache_ttl,attr,optional"`
}

type AttributeFamilies []AttributeFamily

type AttributeFamily struct {
	Attr          string `alloy:"attr,attr"`
	Match         string `alloy:"match,attr,optional"`
	NotMatch      string `alloy:"not_match,attr,optional"`
	GreaterThan   *int   `alloy:"greater_than,attr,optional"`
	GreaterEquals *int   `alloy:"greater_equals,attr,optional"`
	Equals        *int   `alloy:"equals,attr,optional"`
	NotEquals     *int   `alloy:"not_equals,attr,optional"`
	LessEquals    *int   `alloy:"less_equals,attr,optional"`
	LessThan      *int   `alloy:"less_than,attr,optional"`
}

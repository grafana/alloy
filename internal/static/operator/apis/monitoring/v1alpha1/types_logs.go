package v1alpha1

import (
	prom_v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogsSubsystemSpec defines global settings to apply across the logging
// subsystem.
type LogsSubsystemSpec struct {
	// A global set of clients to use when a discovered LogsInstance does not
	// have any clients defined.
	Clients []LogsClientSpec `json:"clients,omitempty"`
	// LogsExternalLabelName is the name of the external label used to
	// denote Grafana Agent cluster. Defaults to "cluster." External label will
	// _not_ be added when value is set to the empty string.
	LogsExternalLabelName *string `json:"logsExternalLabelName,omitempty"`
	// InstanceSelector determines which LogInstances should be selected
	// for running. Each instance runs its own set of Prometheus components,
	// including service discovery, scraping, and remote_write.
	InstanceSelector *metav1.LabelSelector `json:"instanceSelector,omitempty"`
	// InstanceNamespaceSelector are the set of labels to determine which
	// namespaces to watch for LogInstances. If not provided, only checks own
	// namespace.
	InstanceNamespaceSelector *metav1.LabelSelector `json:"instanceNamespaceSelector,omitempty"`

	// IgnoreNamespaceSelectors, if true, will ignore NamespaceSelector settings
	// from the PodLogs configs, and they will only discover endpoints within
	// their current namespace.
	IgnoreNamespaceSelectors bool `json:"ignoreNamespaceSelectors,omitempty"`
	// EnforcedNamespaceLabel enforces adding a namespace label of origin for
	// each metric that is user-created. The label value will always be the
	// namespace of the object that is being created.
	EnforcedNamespaceLabel string `json:"enforcedNamespaceLabel,omitempty"`
}

// LogsClientSpec defines the client integration for logs, indicating which
// Loki server to send logs to.
type LogsClientSpec struct {
	// URL is the URL where Loki is listening. Must be a full HTTP URL, including
	// protocol. Required.
	// Example: https://logs-prod-us-central1.grafana.net/loki/api/v1/push.
	URL string `json:"url"`
	// Tenant ID used by default to push logs to Loki. If omitted assumes remote
	// Loki is running in single-tenant mode or an authentication layer is used
	// to inject an X-Scope-OrgID header.
	TenantID string `json:"tenantId,omitempty"`
	// Maximum amount of time to wait before sending a batch, even if that batch
	// isn't full.
	BatchWait string `json:"batchWait,omitempty"`
	// Maximum batch size (in bytes) of logs to accumulate before sending the
	// batch to Loki.
	BatchSize int `json:"batchSize,omitempty"`
	// BasicAuth for the Loki server.
	BasicAuth *prom_v1.BasicAuth `json:"basicAuth,omitempty"`
	// Oauth2 for URL
	OAuth2 *prom_v1.OAuth2 `json:"oauth2,omitempty"`
	// BearerToken used for remote_write.
	BearerToken string `json:"bearerToken,omitempty"`
	// BearerTokenFile used to read bearer token.
	BearerTokenFile string `json:"bearerTokenFile,omitempty"`
	// ProxyURL to proxy requests through. Optional.
	ProxyURL string `json:"proxyUrl,omitempty"`
	// TLSConfig to use for the client. Only used when the protocol of the URL
	// is https.
	TLSConfig *prom_v1.TLSConfig `json:"tlsConfig,omitempty"`
	// Configures how to retry requests to Loki when a request fails.
	// Defaults to a minPeriod of 500ms, maxPeriod of 5m, and maxRetries of 10.
	BackoffConfig *LogsBackoffConfigSpec `json:"backoffConfig,omitempty"`
	// ExternalLabels are labels to add to any time series when sending data to
	// Loki.
	ExternalLabels map[string]string `json:"externalLabels,omitempty"`
	// Maximum time to wait for a server to respond to a request.
	Timeout string `json:"timeout,omitempty"`
}

// LogsBackoffConfigSpec configures timing for retrying failed requests.
type LogsBackoffConfigSpec struct {
	// Initial backoff time between retries. Time between retries is
	// increased exponentially.
	MinPeriod string `json:"minPeriod,omitempty"`
	// Maximum backoff time between retries.
	MaxPeriod string `json:"maxPeriod,omitempty"`
	// Maximum number of retries to perform before giving up a request.
	MaxRetries int `json:"maxRetries,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path="logsinstances"
// +kubebuilder:resource:singular="logsinstance"
// +kubebuilder:resource:categories="agent-operator"

// LogsInstance controls an individual logs instance within a Grafana Agent
// deployment.
type LogsInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the specification of the desired behavior for the logs
	// instance.
	Spec LogsInstanceSpec `json:"spec,omitempty"`
}

// PodLogsSelector returns the selector to discover PodLogs.
func (i *LogsInstance) PodLogsSelector() ObjectSelector {
	return ObjectSelector{
		ObjectType:        &PodLogs{},
		ParentNamespace:   i.Namespace,
		NamespaceSelector: i.Spec.PodLogsNamespaceSelector,
		Labels:            i.Spec.PodLogsSelector,
	}
}

// LogsInstanceSpec controls how an individual instance will be used to
// discover LogMonitors.
type LogsInstanceSpec struct {
	// Clients controls where logs are written to for this instance.
	Clients []LogsClientSpec `json:"clients,omitempty"`

	// Determines which PodLogs should be selected for including in this
	// instance.
	PodLogsSelector *metav1.LabelSelector `json:"podLogsSelector,omitempty"`
	// Set of labels to determine which namespaces should be watched
	// for PodLogs. If not provided, checks only namespace of the
	// instance.
	PodLogsNamespaceSelector *metav1.LabelSelector `json:"podLogsNamespaceSelector,omitempty"`

	// AdditionalScrapeConfigs allows specifying a key of a Secret containing
	// additional Grafana Agent logging scrape configurations. Scrape
	// configurations specified are appended to the configurations generated by
	// the Grafana Agent Operator.
	//
	// Job configurations specified must have the form as specified in the
	// official Promtail documentation:
	//
	// https://grafana.com/docs/loki/latest/clients/promtail/configuration/#scrape_configs
	//
	// As scrape configs are appended, the user is responsible to make sure it is
	// valid. Note that using this feature may expose the possibility to break
	// upgrades of Grafana Agent. It is advised to review both Grafana Agent and
	// Promtail release notes to ensure that no incompatible scrape configs are
	// going to break Grafana Agent after the upgrade.
	AdditionalScrapeConfigs *v1.SecretKeySelector `json:"additionalScrapeConfigs,omitempty"`

	// Configures how tailed targets are watched.
	TargetConfig *LogsTargetConfigSpec `json:"targetConfig,omitempty"`
}

// LogsTargetConfigSpec configures how tailed targets are watched.
type LogsTargetConfigSpec struct {
	// Period to resync directories being watched and files being tailed to discover
	// new ones or stop watching removed ones.
	SyncPeriod string `json:"syncPeriod,omitempty"`
}

// +kubebuilder:object:root=true

// LogsInstanceList is a list of LogsInstance.
type LogsInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of LogsInstance.
	Items []*LogsInstance `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories="agent-operator"

// PodLogs defines how to collect logs for a pod.
type PodLogs struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the specification of the desired behavior for the PodLogs.
	Spec PodLogsSpec `json:"spec,omitempty"`
}

// PodLogsSpec defines how to collect logs for a pod.
type PodLogsSpec struct {
	// The label to use to retrieve the job name from.
	JobLabel string `json:"jobLabel,omitempty"`
	// PodTargetLabels transfers labels on the Kubernetes Pod onto the target.
	PodTargetLabels []string `json:"podTargetLabels,omitempty"`
	// Selector to select Pod objects. Required.
	Selector metav1.LabelSelector `json:"selector"`
	// Selector to select which namespaces the Pod objects are discovered from.
	NamespaceSelector prom_v1.NamespaceSelector `json:"namespaceSelector,omitempty"`

	// Pipeline stages for this pod. Pipeline stages support transforming and
	// filtering log lines.
	PipelineStages []*PipelineStageSpec `json:"pipelineStages,omitempty"`

	// RelabelConfigs to apply to logs before delivering.
	// Grafana Agent Operator automatically adds relabelings for a few standard
	// Kubernetes fields and replaces original scrape job name with
	// __tmp_logs_job_name.
	//
	// More info: https://grafana.com/docs/loki/latest/clients/promtail/configuration/#relabel_configs
	RelabelConfigs []*prom_v1.RelabelConfig `json:"relabelings,omitempty"`
}

// +kubebuilder:object:root=true

// PodLogsList is a list of PodLogs.
type PodLogsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of PodLogs.
	Items []*PodLogs `json:"items"`
}

// PipelineStageSpec defines an individual pipeline stage. Each stage type is
// mutually exclusive and no more than one may be set per stage.
//
// More information on pipelines can be found in the Promtail documentation:
// https://grafana.com/docs/loki/latest/clients/promtail/pipelines/
type PipelineStageSpec struct {
	// CRI is a parsing stage that reads log lines using the standard
	// CRI logging format. Supply cri: {} to enable.
	CRI *CRIStageSpec `json:"cri,omitempty"`
	// Docker is a parsing stage that reads log lines using the standard
	// Docker logging format. Supply docker: {} to enable.
	Docker *DockerStageSpec `json:"docker,omitempty"`
	// Drop is a filtering stage that lets you drop certain logs.
	Drop *DropStageSpec `json:"drop,omitempty"`
	// JSON is a parsing stage that reads the log line as JSON and accepts
	// JMESPath expressions to extract data.
	//
	// Information on JMESPath: http://jmespath.org/
	JSON *JSONStageSpec `json:"json,omitempty"`
	// LabelAllow is an action stage that only allows the provided labels to be
	// included in the label set that is sent to Loki with the log entry.
	LabelAllow []string `json:"labelAllow,omitempty"`
	// LabelDrop is an action stage that drops labels from the label set that
	// is sent to Loki with the log entry.
	LabelDrop []string `json:"labelDrop,omitempty"`
	// Labels is an action stage that takes data from the extracted map and
	// modifies the label set that is sent to Loki with the log entry.
	//
	// The key is REQUIRED and represents the name for the label that will
	// be created. Value is optional and will be the name from extracted data
	// to use for the value of the label. If the value is not provided, it
	// defaults to match the key.
	Labels map[string]string `json:"labels,omitempty"`
	// Limit is a rate-limiting stage that throttles logs based on
	// several options.
	Limit *LimitStageSpec `json:"limit,omitempty"`
	// Match is a filtering stage that conditionally applies a set of stages
	// or drop entries when a log entry matches a configurable LogQL stream
	// selector and filter expressions.
	Match *MatchStageSpec `json:"match,omitempty"`
	// Metrics is an action stage that supports defining and updating metrics
	// based on data from the extracted map. Created metrics are not pushed to
	// Loki or Prometheus and are instead exposed via the /metrics endpoint of
	// the Grafana Agent pod. The Grafana Agent Operator should be configured
	// with a MetricsInstance that discovers the logging DaemonSet to collect
	// metrics created by this stage.
	Metrics map[string]MetricsStageSpec `json:"metrics,omitempty"`
	// Multiline stage merges multiple lines into a multiline block before
	// passing it on to the next stage in the pipeline.
	Multiline *MultilineStageSpec `json:"multiline,omitempty"`
	// Output stage is an action stage that takes data from the extracted map and
	// changes the log line that will be sent to Loki.
	Output *OutputStageSpec `json:"output,omitempty"`
	// Pack is a transform stage that lets you embed extracted values and labels
	// into the log line by packing the log line and labels inside of a JSON
	// object.
	Pack *PackStageSpec `json:"pack,omitempty"`
	// Regex is a parsing stage that parses a log line using a regular
	// expression.  Named capture groups in the regex allows for adding data into
	// the extracted map.
	Regex *RegexStageSpec `json:"regex,omitempty"`
	// Replace is a parsing stage that parses a log line using a regular
	// expression and replaces the log line. Named capture groups in the regex
	// allows for adding data into the extracted map.
	Replace *ReplaceStageSpec `json:"replace,omitempty"`
	// Template is a transform stage that manipulates the values in the extracted
	// map using Go's template syntax.
	Template *TemplateStageSpec `json:"template,omitempty"`
	// Tenant is an action stage that sets the tenant ID for the log entry picking it from a
	// field in the extracted data map. If the field is missing, the default
	// LogsClientSpec.tenantId will be used.
	Tenant *TenantStageSpec `json:"tenant,omitempty"`
	// Timestamp is an action stage that can change the timestamp of a log line
	// before it is sent to Loki. If not present, the timestamp of a log line
	// defaults to the time when the log line was read.
	Timestamp *TimestampStageSpec `json:"timestamp,omitempty"`
}

// CRIStageSpec is a parsing stage that reads log lines using the standard CRI
// logging format. It needs no defined fields.
type CRIStageSpec struct{}

// DockerStageSpec is a parsing stage that reads log lines using the standard
// Docker logging format. It needs no defined fields.
type DockerStageSpec struct{}

// DropStageSpec is a filtering stage that lets you drop certain logs.
type DropStageSpec struct {
	// Name from the extract data to parse. If empty, uses the log message.
	Source string `json:"source,omitempty"`

	// RE2 regular expression.
	//
	// If source is provided, the regex attempts
	// to match the source.
	//
	// If no source is provided, then the regex attempts
	// to attach the log line.
	//
	// If the provided regex matches the log line or a provided source, the
	// line is dropped.
	Expression string `json:"expression,omitempty"`

	// Value can only be specified when source is specified. If the value
	// provided is an exact match for the given source then the line will be
	// dropped.
	//
	// Mutually exclusive with expression.
	Value string `json:"value,omitempty"`

	// OlderThan will be parsed as a Go duration. If the log line's timestamp
	// is older than the current time minus the provided duration, it will be
	// dropped.
	OlderThan string `json:"olderThan,omitempty"`

	// LongerThan will drop a log line if it its content is longer than this
	// value (in bytes). Can be expressed as an integer (8192) or a number with a
	// suffix (8kb).
	LongerThan string `json:"longerThan,omitempty"`

	// Every time a log line is dropped, the metric logentry_dropped_lines_total
	// is incremented. A "reason" label is added, and can be customized by
	// providing a custom value here. Defaults to "drop_stage".
	DropCounterReason string `json:"dropCounterReason,omitempty"`
}

// JSONStageSpec is a parsing stage that reads the log line as JSON and accepts
// JMESPath expressions to extract data.
type JSONStageSpec struct {
	// Name from the extracted data to parse as JSON. If empty, uses entire log
	// message.
	Source string `json:"source,omitempty"`

	// Set of the key/value pairs of JMESPath expressions. The key will be the
	// key in the extracted data while the expression will be the value,
	// evaluated as a JMESPath from the source data.
	//
	// Literal JMESPath expressions can be used by wrapping a key in double
	// quotes, which then must be wrapped again in single quotes in YAML
	// so they get passed to the JMESPath parser.
	Expressions map[string]string `json:"expressions,omitempty"`
}

// The limit stage is a rate-limiting stage that throttles logs based on
// several options.
type LimitStageSpec struct {
	// The rate limit in lines per second that Promtail will push to Loki.
	Rate int `json:"rate,omitempty"`

	// The cap in the quantity of burst lines that Promtail will push to Loki.
	Burst int `json:"burst,omitempty"`

	// When drop is true, log lines that exceed the current rate limit are discarded.
	// When drop is false, log lines that exceed the current rate limit wait
	// to enter the back pressure mode.
	//
	// Defaults to false.
	Drop bool `json:"drop,omitempty"`
}

// MatchStageSpec is a filtering stage that conditionally applies a set of
// stages or drop entries when a log entry matches a configurable LogQL stream
// selector and filter expressions.
type MatchStageSpec struct {
	// LogQL stream selector and filter expressions. Required.
	Selector string `json:"selector"`

	// Names the pipeline. When defined, creates an additional label
	// in the pipeline_duration_seconds histogram, where the value is
	// concatenated with job_name using an underscore.
	PipelineName string `json:"pipelineName,omitempty"`

	// Determines what action is taken when the selector matches the log line.
	// Can be keep or drop. Defaults to keep. When set to drop, entries are
	// dropped and no later metrics are recorded.
	// Stages must be empty when dropping metrics.
	Action string `json:"action,omitempty"`

	// Every time a log line is dropped, the metric logentry_dropped_lines_total
	// is incremented. A "reason" label is added, and can be customized by
	// providing a custom value here. Defaults to "match_stage."
	DropCounterReason string `json:"dropCounterReason,omitempty"`

	// Nested set of pipeline stages to execute when action is keep and the log
	// line matches selector.
	//
	// An example value for stages may be:
	//
	//   stages: |
	//     - json: {}
	//     - labelAllow: [foo, bar]
	//
	// Note that stages is a string because SIG API Machinery does not
	// support recursive types, and so it cannot be validated for correctness. Be
	// careful not to mistype anything.
	Stages string `json:"stages,omitempty"`
}

// MetricsStageSpec is an action stage that allows for defining and updating
// metrics based on data from the extracted map. Created metrics are not pushed
// to Loki or Prometheus and are instead exposed via the /metrics endpoint of
// the Grafana Agent pod. The Grafana Agent Operator should be configured with
// a MetricsInstance that discovers the logging DaemonSet to collect metrics
// created by this stage.
type MetricsStageSpec struct {
	// The metric type to create. Must be one of counter, gauge, histogram.
	// Required.
	Type string `json:"type"`

	// Sets the description for the created metric.
	Description string `json:"description,omitempty"`

	// Sets the custom prefix name for the metric. Defaults to "promtail_custom_".
	Prefix string `json:"prefix,omitempty"`

	// Key from the extracted data map to use for the metric. Defaults to the
	// metrics name if not present.
	Source string `json:"source,omitempty"`

	// Label values on metrics are dynamic which can cause exported metrics
	// to go stale. To prevent unbounded cardinality, any metrics not updated
	// within MaxIdleDuration are removed.
	//
	// Must be greater or equal to 1s. Defaults to 5m.
	MaxIdleDuration string `json:"maxIdleDuration,omitempty"`

	// If true, all log lines are counted without attempting to match the
	// source to the extracted map. Mutually exclusive with value.
	//
	// Only valid for type: counter.
	MatchAll *bool `json:"matchAll,omitempty"`

	// If true all log line bytes are counted. Can only be set with
	// matchAll: true and action: add.
	//
	// Only valid for type: counter.
	CountEntryBytes *bool `json:"countEntryBytes,omitempty"`

	// Filters down source data and only changes the metric if the targeted
	// value matches the provided string exactly. If not present, all
	// data matches.
	Value string `json:"value,omitempty"`

	// The action to take against the metric. Required.
	//
	// Must be either "inc" or "add" for type: counter or type: histogram.
	// When type: gauge, must be one of "set", "inc", "dec", "add", or "sub".
	//
	// "add", "set", or "sub" requires the extracted value to be convertible
	// to a positive float.
	Action string `json:"action"`

	// Buckets to create. Bucket values must be convertible to float64s. Extremely
	// large or small numbers are subject to some loss of precision.
	// Only valid for type: histogram.
	Buckets []string `json:"buckets,omitempty"`
}

// MultilineStageSpec merges multiple lines into a multiline block before
// passing it on to the next stage in the pipeline.
type MultilineStageSpec struct {
	// RE2 regular expression. Creates a new multiline block when matched.
	// Required.
	FirstLine string `json:"firstLine"`

	// Maximum time to wait before passing on the multiline block to the next
	// stage if no new lines are received. Defaults to 3s.
	MaxWaitTime string `json:"maxWaitTime,omitempty"`

	// Maximum number of lines a block can have. A new block is started if
	// the number of lines surpasses this value. Defaults to 128.
	MaxLines int `json:"maxLines,omitempty"`
}

// OutputStageSpec is an action stage that takes data from the extracted map
// and changes the log line that will be sent to Loki.
type OutputStageSpec struct {
	// Name from extract data to use for the log entry. Required.
	Source string `json:"source"`
}

// PackStageSpec is a transform stage that lets you embed extracted values and
// labels into the log line by packing the log line and labels inside of a JSON
// object.
type PackStageSpec struct {
	// Name from extracted data or line labels. Required.
	// Labels provided here are automatically removed from output labels.
	Labels []string `json:"labels"`

	// If the resulting log line should use any existing timestamp or use time.Now()
	// when the line was created. Set to true when combining several log streams from
	// different containers to avoid out of order errors.
	IngestTimestamp bool `json:"ingestTimestamp,omitempty"`
}

// RegexStageSpec is a parsing stage that parses a log line using a regular
// expression. Named capture groups in the regex allows for adding data into
// the extracted map.
type RegexStageSpec struct {
	// Name from extracted data to parse. If empty, defaults to using the log
	// message.
	Source string `json:"source,omitempty"`

	// RE2 regular expression. Each capture group MUST be named. Required.
	Expression string `json:"expression"`
}

// ReplaceStageSpec is a parsing stage that parses a log line using a regular
// expression and replaces the log line. Named capture groups in the regex
// allows for adding data into the extracted map.
type ReplaceStageSpec struct {
	// Name from extracted data to parse. If empty, defaults to using the log
	// message.
	Source string `json:"source,omitempty"`

	// RE2 regular expression. Each capture group MUST be named. Required.
	Expression string `json:"expression"`

	// Value to replace the captured group with.
	Replace string `json:"replace,omitempty"`
}

// TemplateStageSpec is a transform stage that manipulates the values in the
// extracted map using Go's template syntax.
type TemplateStageSpec struct {
	// Name from extracted data to parse. Required. If empty, defaults to using
	// the log message.
	Source string `json:"source"`

	// Go template string to use. Required. In addition to normal template
	// functions, ToLower, ToUpper, Replace, Trim, TrimLeft, TrimRight,
	// TrimPrefix, and TrimSpace are also available.
	Template string `json:"template"`
}

// TenantStageSpec is an action stage that sets the tenant ID for the log entry
// picking it from a field in the extracted data map.
type TenantStageSpec struct {
	// Name from labels whose value should be set as tenant ID. Mutually exclusive with
	// source and value.
	Label string `json:"label,omitempty"`

	// Name from extracted data to use as the tenant ID. Mutually exclusive with
	// label and value.
	Source string `json:"source,omitempty"`

	// Value to use for the template ID. Useful when this stage is used within a
	// conditional pipeline such as match. Mutually exclusive with label and source.
	Value string `json:"value,omitempty"`
}

// TimestampStageSpec is an action stage that can change the timestamp of a log
// line before it is sent to Loki.
type TimestampStageSpec struct {
	// Name from extracted data to use as the timestamp. Required.
	Source string `json:"source"`

	// Determines format of the time string. Required. Can be one of:
	// ANSIC, UnixDate, RubyDate, RFC822, RFC822Z, RFC850, RFC1123, RFC1123Z,
	// RFC3339, RFC3339Nano, Unix, UnixMs, UnixUs, UnixNs.
	Format string `json:"format"`

	// Fallback formats to try if format fails.
	FallbackFormats []string `json:"fallbackFormats,omitempty"`

	// IANA Timezone Database string.
	Location string `json:"location,omitempty"`

	// Action to take when the timestamp can't be extracted or parsed.
	// Can be skip or fudge. Defaults to fudge.
	ActionOnFailure string `json:"actionOnFailure,omitempty"`
}

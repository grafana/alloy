package traces

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"github.com/prometheus/client_golang/prometheus"
	prom_config "github.com/prometheus/common/config"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	otelexporter "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	otelprocessor "go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.uber.org/multierr"
	"gopkg.in/yaml.v2"

	promsdconsumer "github.com/grafana/alloy/internal/static/traces/promsdprocessor/consumer"

	"github.com/grafana/alloy/internal/static/logs"
	"github.com/grafana/alloy/internal/static/traces/automaticloggingprocessor"
	"github.com/grafana/alloy/internal/static/traces/forked_otelcol"
	"github.com/grafana/alloy/internal/static/traces/noopreceiver"
	"github.com/grafana/alloy/internal/static/traces/promsdprocessor"
	"github.com/grafana/alloy/internal/static/traces/pushreceiver"
	"github.com/grafana/alloy/internal/static/traces/remotewriteexporter"
	"github.com/grafana/alloy/internal/static/traces/servicegraphprocessor"
	"github.com/grafana/alloy/internal/static/traces/spanmetricsprocessor"
	"github.com/grafana/alloy/internal/util"
	_ "github.com/grafana/alloy/internal/util/otelfeaturegatefix" // Gracefully handle duplicate OTEL feature gates
)

const (
	spanMetricsPipelineType     = "metrics"
	spanMetricsPipelineName     = "spanmetrics"
	spanMetricsPipelineFullName = spanMetricsPipelineType + "/" + spanMetricsPipelineName

	// defaultDecisionWait is the default time to wait for a trace before making a sampling decision
	defaultDecisionWait = time.Second * 5

	// defaultNumTraces is the default number of traces kept on memory.
	defaultNumTraces = uint64(50000)

	// defaultLoadBalancingPort is the default port the agent uses for internal load balancing
	defaultLoadBalancingPort = "4318"
	// agent's load balancing options
	dnsTagName        = "dns"
	staticTagName     = "static"
	kubernetesTagName = "kubernetes"

	// sampling policies
	alwaysSamplePolicy = "always_sample"

	// otlp receiver
	otlpReceiverName = "otlp"

	// A string to print out when marshaling "secrets" strings, like passwords.
	secretMarshalString = "<secret>"
)

// Config controls the configuration of Traces trace pipelines.
type Config struct {
	Configs []InstanceConfig `yaml:"configs,omitempty"`

	// Unmarshaled is true when the Config was unmarshaled from YAML.
	Unmarshaled bool `yaml:"-"`
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Unmarshaled = true
	type plain Config
	return unmarshal((*plain)(c))
}

// Validate ensures that the Config is valid.
func (c *Config) Validate(logsConfig *logs.Config) error {
	names := make(map[string]struct{}, len(c.Configs))
	for idx, c := range c.Configs {
		if c.Name == "" {
			return fmt.Errorf("traces config at index %d is missing a name", idx)
		}
		if _, exist := names[c.Name]; exist {
			return fmt.Errorf("found multiple traces configs with name %s", c.Name)
		}
		names[c.Name] = struct{}{}
	}

	for _, inst := range c.Configs {
		if inst.AutomaticLogging != nil {
			if err := inst.AutomaticLogging.Validate(logsConfig); err != nil {
				return fmt.Errorf("failed to validate automatic_logging for traces config %s: %w", inst.Name, err)
			}
		}
	}

	return nil
}

// InstanceConfig configures an individual Traces trace pipeline.
type InstanceConfig struct {
	Name string `yaml:"name"`

	// RemoteWrite defines one or multiple backends that can receive the pipeline's traffic.
	RemoteWrite []RemoteWriteConfig `yaml:"remote_write,omitempty"`

	// Receivers:
	// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.96.0/receiver/README.md
	Receivers ReceiverMap `yaml:"receivers,omitempty"`

	// Batch:
	// https://github.com/open-telemetry/opentelemetry-collector/tree/v0.96.0/processor/batchprocessor
	Batch map[string]interface{} `yaml:"batch,omitempty"`

	// Attributes:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.96.0/processor
	Attributes map[string]interface{} `yaml:"attributes,omitempty"`

	// prom service discovery config
	ScrapeConfigs   []interface{} `yaml:"scrape_configs,omitempty"`
	OperationType   string        `yaml:"prom_sd_operation_type,omitempty"`
	PodAssociations []string      `yaml:"prom_sd_pod_associations,omitempty"`

	// SpanMetricsProcessor:
	SpanMetrics *SpanMetricsConfig `yaml:"spanmetrics,omitempty"`

	// AutomaticLogging
	AutomaticLogging *automaticloggingprocessor.AutomaticLoggingConfig `yaml:"automatic_logging,omitempty"`

	// TailSampling defines a sampling strategy for the pipeline
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.96.0/processor/tailsamplingprocessor
	TailSampling *tailSamplingConfig `yaml:"tail_sampling,omitempty"`

	// LoadBalancing is used to distribute spans of the same trace to the same agent instance
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.96.0/exporter/loadbalancingexporter
	LoadBalancing *loadBalancingConfig `yaml:"load_balancing"`

	// ServiceGraphs
	ServiceGraphs *serviceGraphsConfig `yaml:"service_graphs,omitempty"`

	// Jaeger's Remote Sampling extension:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.96.0/extension/jaegerremotesampling
	JaegerRemoteSampling []JaegerRemoteSamplingConfig `yaml:"jaeger_remote_sampling"`
}

// A string type for secrets like passwords.
// Hides the value of the string during marshaling.
type SecretString string

var _ yaml.Marshaler = (*SecretString)(nil)

// MarshalYAML implements yaml.Marshaler.
func (s SecretString) MarshalYAML() (interface{}, error) {
	return secretMarshalString, nil
}

// JaegerRemoteSamplingMap is a set of Jaeger Remote Sampling extensions.
// Because receivers may be configured with an unknown set of sensitive information,
// ReceiverMap will marshal as YAML to the text "<secret>".
type JaegerRemoteSamplingConfig map[string]interface{}

var _ yaml.Marshaler = (*JaegerRemoteSamplingConfig)(nil)

// MarshalYAML implements yaml.Marshaler.
func (jrsm JaegerRemoteSamplingConfig) MarshalYAML() (interface{}, error) {
	return secretMarshalString, nil
}

// ReceiverMap stores a set of receivers. Because receivers may be configured
// with an unknown set of sensitive information, ReceiverMap will marshal as
// YAML to the text "<secret>".
type ReceiverMap map[string]interface{}

var (
	_ yaml.Marshaler   = (*ReceiverMap)(nil)
	_ yaml.Unmarshaler = (*ReceiverMap)(nil)
)

// UnmarshalYAML implements yaml.Unmarshaler.
func (r *ReceiverMap) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ReceiverMap
	if err := unmarshal((*plain)(r)); err != nil {
		return err
	}

	protocols := []string{protocolHTTP, protocolGRPC}
	// enable include_metadata by default if receiver is OTLP
	for k := range *r {
		if strings.HasPrefix(k, otlpReceiverName) {
			// for http and grpc receivers, include_metadata is set to true by default
			receiverCfg, ok := (*r)[k].(map[interface{}]interface{})
			if !ok {
				return fmt.Errorf("failed to parse OTLP receiver config: %s", k)
			}

			protocolsCfg, ok := receiverCfg["protocols"].(map[interface{}]interface{})
			if !ok {
				return fmt.Errorf("otlp receiver requires a \"protocols\" field which must be a YAML map: %s", k)
			}

			for _, p := range protocols {
				if cfg, ok := protocolsCfg[p]; ok {
					if cfg == nil {
						protocolsCfg[p] = map[interface{}]interface{}{"include_metadata": true}
					} else {
						if _, ok := cfg.(map[interface{}]interface{})["include_metadata"]; !ok {
							protocolsCfg[p].(map[interface{}]interface{})["include_metadata"] = true
						}
					}
				}
			}
		}
	}

	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (r ReceiverMap) MarshalYAML() (interface{}, error) {
	return secretMarshalString, nil
}

const (
	compressionNone = "none"
	compressionGzip = "gzip"
	protocolGRPC    = "grpc"
	protocolHTTP    = "http"
)

const (
	formatOtlp   = "otlp"
	formatJaeger = "jaeger"
)

// DefaultRemoteWriteConfig holds the default settings for a PushConfig.
var DefaultRemoteWriteConfig = RemoteWriteConfig{
	Compression: compressionGzip,
	Protocol:    protocolGRPC,
	Format:      formatOtlp,
}

// TLSClientSetting configures the oauth2client extension TLS; compatible with configtls.TLSClientSetting
type TLSClientSetting struct {
	CAFile                   string        `yaml:"ca_file,omitempty"`
	CAPem                    SecretString  `yaml:"ca_pem,omitempty"`
	IncludeSystemCACertsPool bool          `yaml:"include_system_ca_certs_pool"`
	CertFile                 string        `yaml:"cert_file,omitempty"`
	CertPem                  SecretString  `yaml:"cert_pem,omitempty"`
	KeyFile                  string        `yaml:"key_file,omitempty"`
	KeyPem                   SecretString  `yaml:"key_pem,omitempty"`
	MinVersion               string        `yaml:"min_version,omitempty"`
	MaxVersion               string        `yaml:"max_version,omitempty"`
	CipherSuites             []string      `yaml:"cipher_suites,omitempty"`
	ReloadInterval           time.Duration `yaml:"reload_interval"`
	Insecure                 bool          `yaml:"insecure"`
	InsecureSkipVerify       bool          `yaml:"insecure_skip_verify"`
	ServerNameOverride       string        `yaml:"server_name_override,omitempty"`
}

// OAuth2Config configures the oauth2client extension for a remote_write exporter
// compatible with oauth2clientauthextension.Config
type OAuth2Config struct {
	ClientID       string           `yaml:"client_id"`
	ClientSecret   SecretString     `yaml:"client_secret"`
	EndpointParams url.Values       `yaml:"endpoint_params,omitempty"`
	TokenURL       string           `yaml:"token_url"`
	Scopes         []string         `yaml:"scopes,omitempty"`
	TLS            TLSClientSetting `yaml:"tls,omitempty"`
	Timeout        time.Duration    `yaml:"timeout,omitempty"`
}

// Agent uses standard YAML unmarshalling, while the oauth2clientauthextension relies on
// mapstructure without providing YAML labels. `toOtelConfig` marshals `Oauth2Config` to configuration type expected by
// the oauth2clientauthextension Extension Factory
func (c OAuth2Config) toOtelConfig() (*oauth2clientauthextension.Config, error) {
	var result *oauth2clientauthextension.Config
	decoderConfig := &mapstructure.DecoderConfig{
		MatchName:        func(s, t string) bool { return util.CamelToSnake(s) == t },
		Result:           &result,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToSliceHookFunc(","),
			mapstructure.StringToTimeDurationHookFunc(),
		),
	}
	decoder, _ := mapstructure.NewDecoder(decoderConfig)
	if err := decoder.Decode(c); err != nil {
		return nil, err
	}
	return result, nil
}

// RemoteWriteConfig controls the configuration of an exporter
type RemoteWriteConfig struct {
	Endpoint    string `yaml:"endpoint,omitempty"`
	Compression string `yaml:"compression,omitempty"`
	Protocol    string `yaml:"protocol,omitempty"`
	Insecure    bool   `yaml:"insecure,omitempty"`
	Format      string `yaml:"format,omitempty"`
	// Deprecated
	InsecureSkipVerify bool                   `yaml:"insecure_skip_verify,omitempty"`
	TLSConfig          *prom_config.TLSConfig `yaml:"tls_config,omitempty"`
	BasicAuth          *prom_config.BasicAuth `yaml:"basic_auth,omitempty"`
	Oauth2             *OAuth2Config          `yaml:"oauth2,omitempty"`
	Headers            map[string]string      `yaml:"headers,omitempty"`
	SendingQueue       map[string]interface{} `yaml:"sending_queue,omitempty"`    // https://github.com/open-telemetry/opentelemetry-collector/blob/v0.96.0/exporter/exporterhelper/queued_retry.go
	RetryOnFailure     map[string]interface{} `yaml:"retry_on_failure,omitempty"` // https://github.com/open-telemetry/opentelemetry-collector/blob/v0.96.0/exporter/exporterhelper/queued_retry.go
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (c *RemoteWriteConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultRemoteWriteConfig

	type plain RemoteWriteConfig

	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	if c.Compression != compressionGzip && c.Compression != compressionNone {
		return fmt.Errorf("unsupported compression '%s', expected 'gzip' or 'none'", c.Compression)
	}

	if c.Format != formatOtlp && c.Format != formatJaeger {
		return fmt.Errorf("unsupported format '%s', expected 'otlp' or 'jaeger'", c.Format)
	}
	return nil
}

// SpanMetricsConfig controls the configuration of spanmetricsprocessor and the related metrics exporter.
type SpanMetricsConfig struct {
	LatencyHistogramBuckets []time.Duration                  `yaml:"latency_histogram_buckets,omitempty"`
	Dimensions              []spanmetricsprocessor.Dimension `yaml:"dimensions,omitempty"`
	// Namespace if set, exports metrics under the provided value.
	Namespace string `yaml:"namespace,omitempty"`
	// ConstLabels are values that are applied for every exported metric.
	ConstLabels *prometheus.Labels `yaml:"const_labels,omitempty"`
	// MetricsInstance is the Agent's metrics instance that will be used to push metrics
	MetricsInstance string `yaml:"metrics_instance"`
	// HandlerEndpoint is the address where a prometheus exporter will be exposed
	HandlerEndpoint string `yaml:"handler_endpoint"`

	// DimensionsCacheSize defines the size of cache for storing Dimensions, which helps to avoid cache memory growing
	// indefinitely over the lifetime of the collector.
	DimensionsCacheSize int `yaml:"dimensions_cache_size"`

	// Defines the aggregation temporality of the generated metrics. Can be either of:
	// * "AGGREGATION_TEMPORALITY_CUMULATIVE"
	// * "AGGREGATION_TEMPORALITY_DELTA"
	AggregationTemporality string `yaml:"aggregation_temporality"`

	// MetricsEmitInterval is the time period between when metrics are flushed
	// or emitted to the configured MetricsInstance or HandlerEndpoint.
	MetricsFlushInterval time.Duration `yaml:"metrics_flush_interval"`
}

// tailSamplingConfig is the configuration for tail-based sampling
type tailSamplingConfig struct {
	// Policies are the strategies used for sampling. Multiple policies can be used in the same pipeline.
	// For more information, refer to https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.96.0/processor/tailsamplingprocessor
	Policies []policy `yaml:"policies"`
	// DecisionWait defines the time to wait for a complete trace before making a decision
	DecisionWait time.Duration `yaml:"decision_wait,omitempty"`
	// NumTraces is the number of traces kept on memory. Typically most of the data
	// of a trace is released after a sampling decision is taken.
	NumTraces uint64 `yaml:"num_traces,omitempty"`
	// ExpectedNewTracesPerSec sets the expected number of new traces sending to the tail sampling processor
	// per second. This helps with allocating data structures with closer to actual usage size.
	ExpectedNewTracesPerSec uint64 `yaml:"expected_new_traces_per_sec,omitempty"`
}

type policy struct {
	Name   string                 `yaml:"name,omitempty"`
	Type   string                 `yaml:"type"`
	Policy map[string]interface{} `yaml:",inline"`
}

// loadBalancingConfig defines the configuration for load balancing spans between agent instances
// loadBalancingConfig is an OTel exporter's config with extra resolver config
type loadBalancingConfig struct {
	Exporter exporterConfig         `yaml:"exporter"`
	Resolver map[string]interface{} `yaml:"resolver"`
	// ReceiverPort is the port the instance will use to receive load balanced traces
	ReceiverPort string `yaml:"receiver_port"`
	RoutingKey   string `yaml:"routing_key,omitempty"`
}

// exporterConfig defined the config for an otlp exporter for load balancing
type exporterConfig struct {
	Compression        string                 `yaml:"compression,omitempty"`
	Insecure           bool                   `yaml:"insecure,omitempty"`
	InsecureSkipVerify bool                   `yaml:"insecure_skip_verify,omitempty"`
	BasicAuth          *prom_config.BasicAuth `yaml:"basic_auth,omitempty"`
	Format             string                 `yaml:"format,omitempty"`
}

type serviceGraphsConfig struct {
	Enabled  bool          `yaml:"enabled,omitempty"`
	Wait     time.Duration `yaml:"wait,omitempty"`
	MaxItems int           `yaml:"max_items,omitempty"`
}

// exporter builds an OTel exporter from RemoteWriteConfig
func exporter(rwCfg RemoteWriteConfig) (map[string]interface{}, error) {
	if len(rwCfg.Endpoint) == 0 {
		return nil, errors.New("must have a configured a backend endpoint")
	}

	headers := map[string]string{}
	if rwCfg.Headers != nil {
		headers = rwCfg.Headers
	}

	if rwCfg.BasicAuth != nil && rwCfg.Oauth2 != nil {
		return nil, fmt.Errorf("only one auth type may be configured per exporter (basic_auth or oauth2)")
	}

	if rwCfg.BasicAuth != nil {
		password := string(rwCfg.BasicAuth.Password)

		if len(rwCfg.BasicAuth.PasswordFile) > 0 {
			buff, err := os.ReadFile(rwCfg.BasicAuth.PasswordFile)
			if err != nil {
				return nil, fmt.Errorf("unable to load password file %s: %w", rwCfg.BasicAuth.PasswordFile, err)
			}
			password = strings.TrimSpace(string(buff))
		}

		encodedAuth := base64.StdEncoding.EncodeToString([]byte(rwCfg.BasicAuth.Username + ":" + password))
		headers["authorization"] = "Basic " + encodedAuth
	}

	compression := rwCfg.Compression
	if compression == "" {
		compression = compressionNone
	}

	// Default OTLP exporter config awaits an empty headers map. Other exporters
	// (e.g. Jaeger) may expect a nil value instead
	if len(headers) == 0 && rwCfg.Format == formatJaeger {
		headers = nil
	}
	exporter := map[string]interface{}{
		"endpoint":         rwCfg.Endpoint,
		"compression":      compression,
		"headers":          headers,
		"sending_queue":    rwCfg.SendingQueue,
		"retry_on_failure": rwCfg.RetryOnFailure,
	}

	tlsConfig := map[string]interface{}{
		"insecure": rwCfg.Insecure,
	}
	if !rwCfg.Insecure {
		// If there is a TLSConfig use it
		if rwCfg.TLSConfig != nil {
			tlsConfig["ca_file"] = rwCfg.TLSConfig.CAFile
			tlsConfig["cert_file"] = rwCfg.TLSConfig.CertFile
			tlsConfig["key_file"] = rwCfg.TLSConfig.KeyFile
			tlsConfig["insecure_skip_verify"] = rwCfg.TLSConfig.InsecureSkipVerify
		} else {
			// If not, set whatever value is specified in the old config.
			tlsConfig["insecure_skip_verify"] = rwCfg.InsecureSkipVerify
		}
	}
	exporter["tls"] = tlsConfig

	// Apply some sane defaults to the exporter. The
	// sending_queue.retry_on_failure default is 300s which prevents any
	// sending-related errors to not be logged for 5 minutes. We'll lower that
	// to 60s.
	if retryConfig := exporter["retry_on_failure"].(map[string]interface{}); retryConfig == nil {
		exporter["retry_on_failure"] = map[string]interface{}{
			"max_elapsed_time": "60s",
		}
	} else if retryConfig["max_elapsed_time"] == nil {
		retryConfig["max_elapsed_time"] = "60s"
	}

	return exporter, nil
}

func getExporterName(index int, protocol string, format string) (string, error) {
	switch format {
	case formatOtlp:
		switch protocol {
		case protocolGRPC:
			return fmt.Sprintf("otlp/%d", index), nil
		case protocolHTTP:
			return fmt.Sprintf("otlphttp/%d", index), nil
		default:
			return "", errors.New("unknown protocol, expected either 'http' or 'grpc'")
		}
	case formatJaeger:
		switch protocol {
		case protocolGRPC:
			return fmt.Sprintf("jaeger/%d", index), nil
		default:
			return "", errors.New("unknown protocol, expected 'grpc'")
		}
	default:
		return "", errors.New("unknown format, expected either 'otlp' or 'jaeger'")
	}
}

// exporters builds one or multiple exporters from a remote_write block.
func (c *InstanceConfig) exporters() (map[string]interface{}, error) {
	exporters := map[string]interface{}{}
	for i, remoteWriteConfig := range c.RemoteWrite {
		exporter, err := exporter(remoteWriteConfig)
		if err != nil {
			return nil, err
		}
		exporterName, err := getExporterName(i, remoteWriteConfig.Protocol, remoteWriteConfig.Format)
		if err != nil {
			return nil, err
		}
		if remoteWriteConfig.Oauth2 != nil {
			exporter["auth"] = map[string]string{"authenticator": getAuthExtensionName(exporterName)}
		}
		exporters[exporterName] = exporter
	}
	return exporters, nil
}

func getAuthExtensionName(exporterName string) string {
	return fmt.Sprintf("oauth2client/%s", strings.Replace(exporterName, "/", "", -1))
}

// builds oauth2clientauth extensions required to support RemoteWriteConfigurations.
func (c *InstanceConfig) extensions() (map[string]interface{}, error) {
	extensions := map[string]interface{}{}
	for i, remoteWriteConfig := range c.RemoteWrite {
		if remoteWriteConfig.Oauth2 == nil {
			continue
		}
		exporterName, err := getExporterName(i, remoteWriteConfig.Protocol, remoteWriteConfig.Format)
		if err != nil {
			return nil, err
		}
		oauthConfig, err := remoteWriteConfig.Oauth2.toOtelConfig()
		if err != nil {
			return nil, err
		}
		extensions[getAuthExtensionName(exporterName)] = oauthConfig
	}
	if c.JaegerRemoteSampling != nil {
		if len(c.JaegerRemoteSampling) == 0 {
			return nil, fmt.Errorf("at least one jaeger_remote_sampling configuration must be specified")
		}
		for i, jrsConfig := range c.JaegerRemoteSampling {
			extName := fmt.Sprintf("jaegerremotesampling/%d", i)
			extensions[extName] = jrsConfig
		}
	}
	return extensions, nil
}

func resolver(config map[string]interface{}) (map[string]interface{}, error) {
	if len(config) == 0 {
		return nil, fmt.Errorf("must configure one resolver (dns, static, or kubernetes)")
	}
	resolverCfg := make(map[string]interface{})
	for typ, cfg := range config {
		switch typ {
		case dnsTagName, staticTagName:
			resolverCfg[typ] = cfg
		case kubernetesTagName:
			resolverCfg["k8s"] = cfg
		default:
			return nil, fmt.Errorf("unsupported resolver config type: %s", typ)
		}
	}
	return resolverCfg, nil
}

func (c *InstanceConfig) loadBalancingExporter() (map[string]interface{}, error) {
	exporter, err := exporter(RemoteWriteConfig{
		// Endpoint is omitted in OTel load balancing exporter
		Endpoint:    "noop:8888",
		Compression: c.LoadBalancing.Exporter.Compression,
		Insecure:    c.LoadBalancing.Exporter.Insecure,
		TLSConfig:   &prom_config.TLSConfig{InsecureSkipVerify: c.LoadBalancing.Exporter.InsecureSkipVerify},
		BasicAuth:   c.LoadBalancing.Exporter.BasicAuth,
		Format:      c.LoadBalancing.Exporter.Format,
		Headers:     map[string]string{},
	})
	if err != nil {
		return nil, err
	}
	resolverCfg, err := resolver(c.LoadBalancing.Resolver)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"protocol": map[string]interface{}{
			"otlp": exporter,
		},
		"resolver":    resolverCfg,
		"routing_key": c.LoadBalancing.RoutingKey,
	}, nil
}

// formatPolicies creates sampling policies (i.e. rules) compatible with OTel's tail sampling processor
// https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.96.0/processor/tailsamplingprocessor
func formatPolicies(cfg []policy) ([]map[string]interface{}, error) {
	policies := make([]map[string]interface{}, 0, len(cfg))
	for i, policy := range cfg {
		typ, name := policy.Type, policy.Name
		if typ == "" {
			return nil, fmt.Errorf("policy %d must have a type", i)
		}

		if name == "" {
			name = fmt.Sprintf("%s/%d", typ, i)
		}

		switch typ {
		case alwaysSamplePolicy:
			policies = append(policies, map[string]interface{}{
				"name": name,
				"type": typ,
			})
		default:
			policies = append(policies, map[string]interface{}{
				"name": name,
				"type": typ,
				typ:    policy.Policy[typ],
			})
		}
	}
	return policies, nil
}

func (c *InstanceConfig) OtelConfig() (*otelcol.Config, error) {
	otelMapStructure := map[string]interface{}{}

	if len(c.Receivers) == 0 {
		return nil, errors.New("must have at least one configured receiver")
	}

	// add a hacky push receiver for when an integration
	// wants to push traces directly, e.g. app agent receiver.
	// it can only accept traces programmatically from inside the agent
	c.Receivers[pushreceiver.TypeStr] = nil

	extensions, err := c.extensions()
	if err != nil {
		return nil, err
	}
	extensionsNames := make([]string, 0, len(extensions))
	for name := range extensions {
		extensionsNames = append(extensionsNames, name)
	}

	exporters, err := c.exporters()
	if err != nil {
		return nil, err
	}
	exportersNames := make([]string, 0, len(exporters))
	for name := range exporters {
		exportersNames = append(exportersNames, name)
	}

	// processors
	processors := map[string]interface{}{}
	processorNames := []string{}
	if c.ScrapeConfigs != nil {
		opType := promsdconsumer.OperationTypeUpsert
		if c.OperationType != "" {
			opType = c.OperationType
		}
		processorNames = append(processorNames, promsdprocessor.TypeStr)
		processors[promsdprocessor.TypeStr] = map[string]interface{}{
			"scrape_configs":   c.ScrapeConfigs,
			"operation_type":   opType,
			"pod_associations": c.PodAssociations,
		}
	}

	if c.AutomaticLogging != nil {
		processorNames = append(processorNames, automaticloggingprocessor.TypeStr)
		processors[automaticloggingprocessor.TypeStr] = map[string]interface{}{
			"automatic_logging": c.AutomaticLogging,
		}
	}

	if c.Attributes != nil {
		processors["attributes"] = c.Attributes
		processorNames = append(processorNames, "attributes")
	}

	if c.Batch != nil {
		processors["batch"] = c.Batch
		processorNames = append(processorNames, "batch")
	}

	pipelines := make(map[string]interface{})
	if c.SpanMetrics != nil {
		// Configure the metrics exporter.
		namespace := "traces_spanmetrics"
		if len(c.SpanMetrics.Namespace) != 0 {
			namespace = fmt.Sprintf("%s_%s", c.SpanMetrics.Namespace, namespace)
		}

		var exporterName string
		if len(c.SpanMetrics.MetricsInstance) != 0 && len(c.SpanMetrics.HandlerEndpoint) == 0 {
			exporterName = remotewriteexporter.TypeStr
			exporters[remotewriteexporter.TypeStr] = map[string]interface{}{
				"namespace":        namespace,
				"const_labels":     c.SpanMetrics.ConstLabels,
				"metrics_instance": c.SpanMetrics.MetricsInstance,
			}
		} else if len(c.SpanMetrics.MetricsInstance) == 0 && len(c.SpanMetrics.HandlerEndpoint) != 0 {
			exporterName = "prometheus"
			exporters[exporterName] = map[string]interface{}{
				"endpoint":     c.SpanMetrics.HandlerEndpoint,
				"namespace":    namespace,
				"const_labels": c.SpanMetrics.ConstLabels,
			}
		} else {
			return nil, fmt.Errorf("must specify a prometheus instance or a metrics handler endpoint to export the metrics")
		}

		processorNames = append(processorNames, "spanmetrics")
		spanMetrics := map[string]interface{}{
			"metrics_exporter":          exporterName,
			"latency_histogram_buckets": c.SpanMetrics.LatencyHistogramBuckets,
			"dimensions":                c.SpanMetrics.Dimensions,
		}
		if c.SpanMetrics.AggregationTemporality != "" {
			spanMetrics["aggregation_temporality"] = c.SpanMetrics.AggregationTemporality
		}
		if c.SpanMetrics.MetricsFlushInterval != 0 {
			spanMetrics["metrics_flush_interval"] = c.SpanMetrics.MetricsFlushInterval
		}
		if c.SpanMetrics.DimensionsCacheSize != 0 {
			spanMetrics["dimensions_cache_size"] = c.SpanMetrics.DimensionsCacheSize
		}
		processors["spanmetrics"] = spanMetrics

		pipelines[spanMetricsPipelineFullName] = map[string]interface{}{
			"receivers": []string{noopreceiver.TypeStr},
			"exporters": []string{exporterName},
		}
	}

	// receivers
	receiverNames := []string{}
	for name := range c.Receivers {
		receiverNames = append(receiverNames, name)
	}

	if c.TailSampling != nil {
		expectedNewTracesPerSec := c.TailSampling.ExpectedNewTracesPerSec

		numTraces := defaultNumTraces
		if c.TailSampling.NumTraces != 0 {
			numTraces = c.TailSampling.NumTraces
		}

		wait := defaultDecisionWait
		if c.TailSampling.DecisionWait != 0 {
			wait = c.TailSampling.DecisionWait
		}

		policies, err := formatPolicies(c.TailSampling.Policies)
		if err != nil {
			return nil, err
		}

		// tail_sampling should be executed before the batch processor
		// TODO(mario.rodriguez): put attributes processor before tail_sampling. Maybe we want to sample on mutated spans
		processorNames = append([]string{"tail_sampling"}, processorNames...)
		processors["tail_sampling"] = map[string]interface{}{
			"policies":                    policies,
			"decision_wait":               wait,
			"num_traces":                  numTraces,
			"expected_new_traces_per_sec": expectedNewTracesPerSec,
		}
	}

	if c.LoadBalancing != nil {
		internalExporter, err := c.loadBalancingExporter()
		if err != nil {
			return nil, err
		}
		exporters["loadbalancing"] = internalExporter

		receiverPort := defaultLoadBalancingPort
		if c.LoadBalancing.ReceiverPort != "" {
			receiverPort = c.LoadBalancing.ReceiverPort
		}
		c.Receivers["otlp/lb"] = map[string]interface{}{
			"protocols": map[string]interface{}{
				"grpc": map[string]interface{}{
					"endpoint": net.JoinHostPort("0.0.0.0", receiverPort),
				},
			},
		}
	}

	if c.ServiceGraphs != nil && c.ServiceGraphs.Enabled {
		processors[servicegraphprocessor.TypeStr] = map[string]interface{}{
			"wait":      c.ServiceGraphs.Wait,
			"max_items": c.ServiceGraphs.MaxItems,
		}
		processorNames = append(processorNames, servicegraphprocessor.TypeStr)
	}

	// Build Pipelines
	splitPipeline := c.LoadBalancing != nil
	orderedSplitProcessors := orderProcessors(processorNames, splitPipeline)
	if splitPipeline {
		// load balancing pipeline
		pipelines["traces/0"] = map[string]interface{}{
			"receivers":  receiverNames,
			"processors": orderedSplitProcessors[0],
			"exporters":  []string{"loadbalancing"},
		}
		// processing pipeline
		pipelines["traces/1"] = map[string]interface{}{
			"exporters":  exportersNames,
			"processors": orderedSplitProcessors[1],
			"receivers":  []string{"otlp/lb"},
		}
	} else {
		pipelines["traces"] = map[string]interface{}{
			"exporters":  exportersNames,
			"processors": orderedSplitProcessors[0],
			"receivers":  receiverNames,
		}
	}

	if c.SpanMetrics != nil {
		// Insert a noop receiver in the metrics pipeline.
		// Added to pass validation requiring at least one receiver in a pipeline.
		c.Receivers[noopreceiver.TypeStr] = nil
	}

	receiversMap := map[string]interface{}(c.Receivers)

	otelMapStructure["extensions"] = extensions
	otelMapStructure["exporters"] = exporters
	otelMapStructure["processors"] = processors
	otelMapStructure["receivers"] = receiversMap

	// pipelines
	serviceMap := map[string]interface{}{
		"pipelines": pipelines,
	}
	if len(extensionsNames) > 0 {
		serviceMap["extensions"] = extensionsNames
	}
	otelMapStructure["service"] = serviceMap

	factories, err := tracingFactories()
	if err != nil {
		return nil, fmt.Errorf("failed to create factories: %w", err)
	}

	if err := validateConfigFromFactories(factories); err != nil {
		return nil, fmt.Errorf("failed to validate factories: %w", err)
	}

	return otelcolConfigFromStringMap(otelMapStructure, &factories)
}

// tracingFactories() only creates the needed factories.  if we decide to add support for a new
// processor, exporter, receiver we need to add it here
func tracingFactories() (otelcol.Factories, error) {
	extensions, err := extension.MakeFactoryMap(
		oauth2clientauthextension.NewFactory(),
		jaegerremotesampling.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	receivers, err := receiver.MakeFactoryMap(
		jaegerreceiver.NewFactory(),
		zipkinreceiver.NewFactory(),
		otlpreceiver.NewFactory(),
		opencensusreceiver.NewFactory(),
		kafkareceiver.NewFactory(),
		noopreceiver.NewFactory(),
		pushreceiver.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	exporters, err := otelexporter.MakeFactoryMap(
		otlpexporter.NewFactory(),
		otlphttpexporter.NewFactory(),
		loadbalancingexporter.NewFactory(),
		prometheusexporter.NewFactory(),
		remotewriteexporter.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	processors, err := otelprocessor.MakeFactoryMap(
		batchprocessor.NewFactory(),
		attributesprocessor.NewFactory(),
		promsdprocessor.NewFactory(),
		spanmetricsprocessor.NewFactory(),
		automaticloggingprocessor.NewFactory(),
		tailsamplingprocessor.NewFactory(),
		servicegraphprocessor.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	return otelcol.Factories{
		Extensions: extensions,
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
	}, nil
}

// orders the passed processors into their preferred order in a tracing pipeline. pass
// true to splitPipelines if this function should split the input pipelines into two
// sets: before and after load balancing
func orderProcessors(processors []string, splitPipelines bool) [][]string {
	order := map[string]int{
		"attributes": 0,
		// Spanmetrics should be before tail_sampling so that
		// metrics are generated using as many spans as possible.
		"spanmetrics":       1,
		"service_graphs":    2,
		"tail_sampling":     3,
		"automatic_logging": 4,
		"batch":             5,
	}

	sort.Slice(processors, func(i, j int) bool {
		iVal := order[processors[i]]
		jVal := order[processors[j]]

		return iVal < jVal
	})

	if !splitPipelines {
		return [][]string{
			processors,
		}
	}

	// if we're splitting pipelines we have to look for the first processor that belongs in the second
	// stage and split on that. if nothing belongs in the second stage just leave them all in the first
	foundAt := len(processors)
	for i, processor := range processors {
		if processor == "batch" ||
			processor == "tail_sampling" ||
			processor == "automatic_logging" ||
			processor == "spanmetrics" ||
			processor == "service_graphs" {

			foundAt = i
			break
		}
	}

	return [][]string{
		processors[:foundAt],
		processors[foundAt:],
	}
}

func otelcolConfigFromStringMap(otelMapStructure map[string]interface{}, factories *otelcol.Factories) (*otelcol.Config, error) {
	configMap := confmap.NewFromStringMap(otelMapStructure)
	otelCfg, err := forked_otelcol.Unmarshal(configMap, *factories)
	if err != nil {
		return nil, fmt.Errorf("failed to load OTel config: %w", err)
	}

	res := otelcol.Config{
		Receivers:  otelCfg.Receivers.Configs(),
		Processors: otelCfg.Processors.Configs(),
		Exporters:  otelCfg.Exporters.Configs(),
		Connectors: otelCfg.Connectors.Configs(),
		Extensions: otelCfg.Extensions.Configs(),
		Service:    otelCfg.Service,
	}

	if err := res.Validate(); err != nil {
		return nil, err
	}

	return &res, nil
}

// Code taken from OTel's service/configcheck.go
// https://github.com/grafana/opentelemetry-collector/blob/0.40-grafana/service/configcheck.go#L26-L43
func validateConfigFromFactories(factories otelcol.Factories) error {
	var errs error

	// TODO: We should not use componenttest in non-test code
	for _, factory := range factories.Receivers {
		errs = multierr.Append(errs, componenttest.CheckConfigStruct(factory.CreateDefaultConfig()))
	}
	for _, factory := range factories.Processors {
		errs = multierr.Append(errs, componenttest.CheckConfigStruct(factory.CreateDefaultConfig()))
	}
	for _, factory := range factories.Exporters {
		errs = multierr.Append(errs, componenttest.CheckConfigStruct(factory.CreateDefaultConfig()))
	}
	for _, factory := range factories.Extensions {
		errs = multierr.Append(errs, componenttest.CheckConfigStruct(factory.CreateDefaultConfig()))
	}

	return errs
}

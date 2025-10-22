package kafka_exporter

import (
	"fmt"

	config_util "github.com/prometheus/common/config"

	"github.com/IBM/sarama"
	"github.com/go-kit/log"
	kafka_exporter "github.com/grafana/kafka_exporter/exporter"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig holds the default settings for the kafka_lag_exporter
// integration.
var DefaultConfig = Config{
	UseSASLHandshake:        true,
	KafkaVersion:            sarama.V2_0_0_0.String(),
	MetadataRefreshInterval: "1m",
	AllowConcurrent:         true,
	MaxOffsets:              1000,
	PruneIntervalSeconds:    30,
	OffsetShowAll:           true,
	TopicWorkers:            100,
	TopicsFilter:            ".*",
	TopicsExclude:           "^$",
	GroupFilter:             ".*",
	GroupExclude:            "^$",
}

// Config controls kafka_exporter
type Config struct {
	// The instance label for metrics.
	Instance string `yaml:"instance,omitempty"`

	// Address array (host:port) of Kafka server
	KafkaURIs []string `yaml:"kafka_uris,omitempty"`

	// Connect using SASL/PLAIN
	UseSASL bool `yaml:"use_sasl,omitempty"`

	// Only set this to false if using a non-Kafka SASL proxy
	UseSASLHandshake bool `yaml:"use_sasl_handshake,omitempty"`

	// SASL user name
	SASLUsername string `yaml:"sasl_username,omitempty"`

	// SASL user password
	SASLPassword config_util.Secret `yaml:"sasl_password,omitempty"`

	// The SASL SCRAM SHA algorithm sha256 or sha512 as mechanism
	SASLMechanism string `yaml:"sasl_mechanism,omitempty"`

	// Configure the Kerberos client to not use PA_FX_FAST.
	SASLDisablePAFXFast bool `yaml:"sasl_disable_pafx_fast,omitempty"`

	// Connect using TLS
	UseTLS bool `yaml:"use_tls,omitempty"`

	// Used to verify the hostname on the returned certificates unless tls.insecure-skip-tls-verify is given. The kafka server's name should be given.
	TlsServerName string `yaml:"tls_server_name,omitempty"`

	// The optional certificate authority file for TLS client authentication
	CAFile string `yaml:"ca_file,omitempty"`

	// The optional certificate file for TLS client authentication
	CertFile string `yaml:"cert_file,omitempty"`

	// The optional key file for TLS client authentication
	KeyFile string `yaml:"key_file,omitempty"`

	// If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
	InsecureSkipVerify bool `yaml:"insecure_skip_verify,omitempty"`

	// Kafka broker version
	KafkaVersion string `yaml:"kafka_version,omitempty"`

	// if you need to use a group from zookeeper
	UseZooKeeperLag bool `yaml:"use_zookeeper_lag,omitempty"`

	// Address array (hosts) of zookeeper server.
	ZookeeperURIs []string `yaml:"zookeeper_uris,omitempty"`

	// Kafka cluster name
	ClusterName string `yaml:"kafka_cluster_name,omitempty"`

	// Metadata refresh interval
	MetadataRefreshInterval string `yaml:"metadata_refresh_interval,omitempty"`

	// Service name when using kerberos Auth.
	ServiceName string `yaml:"gssapi_service_name,omitempty"`

	// Kerberos config path.
	KerberosConfigPath string `yaml:"gssapi_kerberos_config_path,omitempty"`

	// Kerberos realm.
	Realm string `yaml:"gssapi_realm,omitempty"`

	// Kerberos keytab file path.
	KeyTabPath string `yaml:"gssapi_key_tab_path,omitempty"`

	// Kerberos auth type. Either 'keytabAuth' or 'userAuth'.
	KerberosAuthType string `yaml:"gssapi_kerberos_auth_type,omitempty"`

	// Whether show the offset/lag for all consumer group, otherwise, only show connected consumer groups.
	OffsetShowAll bool `yaml:"offset_show_all,omitempty"`

	// Number of topic workers.
	TopicWorkers int `yaml:"topic_workers,omitempty"`

	// If true, all scrapes will trigger kafka operations otherwise, they will share results. WARN: This should be disabled on large clusters
	AllowConcurrent bool `yaml:"allow_concurrency,omitempty"`

	// If true, the broker may auto-create topics that we requested which do not already exist.
	AllowAutoTopicCreation bool `yaml:"allow_auto_topic_creation,omitempty"`

	// Maximum number of offsets to store in the interpolation table for a partition
	MaxOffsets int `yaml:"max_offsets,omitempty"`

	// No-op (deprecated). Use metadata_refresh_interval instead.
	PruneIntervalSeconds int `yaml:"prune_interval_seconds,omitempty"`

	// Regex filter for topics to be monitored
	TopicsFilter string `yaml:"topics_filter_regex,omitempty"`

	// Regex that determines which topics to exclude.
	TopicsExclude string `yaml:"topics_exclude_regex,omitempty"`

	// Regex filter for consumer groups to be monitored
	GroupFilter string `yaml:"groups_filter_regex,omitempty"`

	// Regex that determines which consumer groups to exclude.
	GroupExclude string `yaml:"groups_exclude_regex,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "kafka_exporter"
}

// InstanceKey returns the hostname:port of the first Kafka node, if any. If
// there is not exactly one Kafka node, the user must manually provide
// their own value for instance key in the common config.
func (c *Config) InstanceKey(_ string) (string, error) {
	if len(c.KafkaURIs) == 1 {
		return c.KafkaURIs[0], nil
	}
	if c.Instance == "" && len(c.KafkaURIs) > 1 {
		return "", fmt.Errorf("an automatic value for `instance` cannot be determined from %d kafka servers, manually provide one for this integration", len(c.KafkaURIs))
	}

	return c.Instance, nil
}

// NewIntegration creates a new elasticsearch_exporter
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	return New(logger, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("kafka"))
}

// New creates a new kafka_exporter integration.
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	if len(c.KafkaURIs) == 0 || c.KafkaURIs[0] == "" {
		return nil, fmt.Errorf("empty kafka_uris provided")
	}
	if c.UseSASL && (c.SASLPassword == "" || c.SASLUsername == "") {
		return nil, fmt.Errorf("SASL is enabled but username or password was not provided")
	}
	if c.UseZooKeeperLag && (len(c.ZookeeperURIs) == 0 || c.ZookeeperURIs[0] == "") {
		return nil, fmt.Errorf("zookeeper lag is enabled but no zookeeper uri was provided")
	}

	// 30 is the default value
	if c.PruneIntervalSeconds != 30 {
		level.Warn(logger).Log("msg", "prune_interval_seconds is not used anymore, use metadata_refresh_interval instead")
	}

	options := kafka_exporter.Options{
		Uri:                      c.KafkaURIs,
		UseSASL:                  c.UseSASL,
		UseSASLHandshake:         c.UseSASLHandshake,
		SaslUsername:             c.SASLUsername,
		SaslPassword:             string(c.SASLPassword),
		SaslMechanism:            c.SASLMechanism,
		SaslDisablePAFXFast:      c.SASLDisablePAFXFast,
		UseTLS:                   c.UseTLS,
		TlsServerName:            c.TlsServerName,
		TlsCAFile:                c.CAFile,
		TlsCertFile:              c.CertFile,
		TlsKeyFile:               c.KeyFile,
		TlsInsecureSkipTLSVerify: c.InsecureSkipVerify,
		KafkaVersion:             c.KafkaVersion,
		UseZooKeeperLag:          c.UseZooKeeperLag,
		UriZookeeper:             c.ZookeeperURIs,
		Labels:                   c.ClusterName,
		MetadataRefreshInterval:  c.MetadataRefreshInterval,
		ServiceName:              c.ServiceName,
		KerberosConfigPath:       c.KerberosConfigPath,
		Realm:                    c.Realm,
		KeyTabPath:               c.KeyTabPath,
		KerberosAuthType:         c.KerberosAuthType,
		OffsetShowAll:            c.OffsetShowAll,
		TopicWorkers:             c.TopicWorkers,
		AllowConcurrent:          c.AllowConcurrent,
		AllowAutoTopicCreation:   c.AllowAutoTopicCreation,
		MaxOffsets:               c.MaxOffsets,
	}

	newExporter, err := kafka_exporter.New(logger, options, c.TopicsFilter, c.TopicsExclude, c.GroupFilter, c.GroupExclude)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate kafka lag exporter: %w", err)
	}

	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(newExporter),
	), nil
}

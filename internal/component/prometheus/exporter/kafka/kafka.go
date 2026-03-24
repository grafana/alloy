package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/kafka_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

var DefaultArguments = Arguments{
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

type Arguments struct {
	Instance                string            `alloy:"instance,attr,optional"`
	KafkaURIs               []string          `alloy:"kafka_uris,attr,optional"`
	UseSASL                 bool              `alloy:"use_sasl,attr,optional"`
	UseSASLHandshake        bool              `alloy:"use_sasl_handshake,attr,optional"`
	SASLUsername            string            `alloy:"sasl_username,attr,optional"`
	SASLPassword            alloytypes.Secret `alloy:"sasl_password,attr,optional"`
	SASLMechanism           string            `alloy:"sasl_mechanism,attr,optional"`
	SASLDisablePAFXFast     bool              `alloy:"sasl_disable_pafx_fast,attr,optional"`
	UseTLS                  bool              `alloy:"use_tls,attr,optional"`
	TlsServerName           string            `alloy:"tls_server_name,attr,optional"`
	CAFile                  string            `alloy:"ca_file,attr,optional"`
	CertFile                string            `alloy:"cert_file,attr,optional"`
	KeyFile                 string            `alloy:"key_file,attr,optional"`
	InsecureSkipVerify      bool              `alloy:"insecure_skip_verify,attr,optional"`
	KafkaVersion            string            `alloy:"kafka_version,attr,optional"`
	UseZooKeeperLag         bool              `alloy:"use_zookeeper_lag,attr,optional"`
	ZookeeperURIs           []string          `alloy:"zookeeper_uris,attr,optional"`
	ClusterName             string            `alloy:"kafka_cluster_name,attr,optional"`
	MetadataRefreshInterval string            `alloy:"metadata_refresh_interval,attr,optional"`
	ServiceName             string            `alloy:"gssapi_service_name,attr,optional"`
	KerberosConfigPath      string            `alloy:"gssapi_kerberos_config_path,attr,optional"`
	Realm                   string            `alloy:"gssapi_realm,attr,optional"`
	KeyTabPath              string            `alloy:"gssapi_key_tab_path,attr,optional"`
	KerberosAuthType        string            `alloy:"gssapi_kerberos_auth_type,attr,optional"`
	OffsetShowAll           bool              `alloy:"offset_show_all,attr,optional"`
	TopicWorkers            int               `alloy:"topic_workers,attr,optional"`
	AllowConcurrent         bool              `alloy:"allow_concurrency,attr,optional"`
	AllowAutoTopicCreation  bool              `alloy:"allow_auto_topic_creation,attr,optional"`
	MaxOffsets              int               `alloy:"max_offsets,attr,optional"`
	PruneIntervalSeconds    int               `alloy:"prune_interval_seconds,attr,optional"` // deprecated - no-op
	TopicsFilter            string            `alloy:"topics_filter_regex,attr,optional"`
	TopicsExclude           string            `alloy:"topics_exclude_regex,attr,optional"`
	GroupFilter             string            `alloy:"groups_filter_regex,attr,optional"`
	GroupExclude            string            `alloy:"groups_exclude_regex,attr,optional"`
}

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.kafka",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.NewWithTargetBuilder(createExporter, "kafka", customizeTarget),
	})
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.Instance == "" && len(a.KafkaURIs) > 1 {
		return fmt.Errorf("an automatic value for `instance` cannot be determined from %d kafka servers, manually provide one for this component", len(a.KafkaURIs))
	}
	return nil
}

func customizeTarget(baseTarget discovery.Target, args component.Arguments) []discovery.Target {
	a := args.(Arguments)
	targetBuilder := discovery.NewTargetBuilderFrom(baseTarget)
	if len(a.KafkaURIs) > 1 {
		targetBuilder.Set("instance", a.Instance)
	} else {
		targetBuilder.Set("instance", a.KafkaURIs[0])
	}
	return []discovery.Target{targetBuilder.Target()}
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

func (a *Arguments) Convert() *kafka_exporter.Config {
	return &kafka_exporter.Config{
		Instance:                a.Instance,
		KafkaURIs:               a.KafkaURIs,
		UseSASL:                 a.UseSASL,
		UseSASLHandshake:        a.UseSASLHandshake,
		SASLUsername:            a.SASLUsername,
		SASLPassword:            config.Secret(a.SASLPassword),
		SASLMechanism:           a.SASLMechanism,
		SASLDisablePAFXFast:     a.SASLDisablePAFXFast,
		UseTLS:                  a.UseTLS,
		TlsServerName:           a.TlsServerName,
		CAFile:                  a.CAFile,
		CertFile:                a.CertFile,
		KeyFile:                 a.KeyFile,
		InsecureSkipVerify:      a.InsecureSkipVerify,
		KafkaVersion:            a.KafkaVersion,
		UseZooKeeperLag:         a.UseZooKeeperLag,
		ZookeeperURIs:           a.ZookeeperURIs,
		ClusterName:             a.ClusterName,
		MetadataRefreshInterval: a.MetadataRefreshInterval,
		ServiceName:             a.ServiceName,
		KerberosConfigPath:      a.KerberosConfigPath,
		Realm:                   a.Realm,
		KeyTabPath:              a.KeyTabPath,
		KerberosAuthType:        a.KerberosAuthType,
		OffsetShowAll:           a.OffsetShowAll,
		TopicWorkers:            a.TopicWorkers,
		AllowConcurrent:         a.AllowConcurrent,
		AllowAutoTopicCreation:  a.AllowAutoTopicCreation,
		MaxOffsets:              a.MaxOffsets,
		PruneIntervalSeconds:    a.PruneIntervalSeconds,
		TopicsFilter:            a.TopicsFilter,
		TopicsExclude:           a.TopicsExclude,
		GroupFilter:             a.GroupFilter,
		GroupExclude:            a.GroupExclude,
	}
}

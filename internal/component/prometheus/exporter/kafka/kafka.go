package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/kafka_exporter"
	"github.com/grafana/river/rivertypes"
	"github.com/prometheus/common/config"
)

var DefaultArguments = Arguments{
	UseSASLHandshake:        true,
	KafkaVersion:            sarama.V2_0_0_0.String(),
	MetadataRefreshInterval: "1m",
	AllowConcurrent:         true,
	MaxOffsets:              1000,
	PruneIntervalSeconds:    30,
	TopicsFilter:            ".*",
	GroupFilter:             ".*",
}

type Arguments struct {
	Instance                string            `river:"instance,attr,optional"`
	KafkaURIs               []string          `river:"kafka_uris,attr,optional"`
	UseSASL                 bool              `river:"use_sasl,attr,optional"`
	UseSASLHandshake        bool              `river:"use_sasl_handshake,attr,optional"`
	SASLUsername            string            `river:"sasl_username,attr,optional"`
	SASLPassword            rivertypes.Secret `river:"sasl_password,attr,optional"`
	SASLMechanism           string            `river:"sasl_mechanism,attr,optional"`
	UseTLS                  bool              `river:"use_tls,attr,optional"`
	CAFile                  string            `river:"ca_file,attr,optional"`
	CertFile                string            `river:"cert_file,attr,optional"`
	KeyFile                 string            `river:"key_file,attr,optional"`
	InsecureSkipVerify      bool              `river:"insecure_skip_verify,attr,optional"`
	KafkaVersion            string            `river:"kafka_version,attr,optional"`
	UseZooKeeperLag         bool              `river:"use_zookeeper_lag,attr,optional"`
	ZookeeperURIs           []string          `river:"zookeeper_uris,attr,optional"`
	ClusterName             string            `river:"kafka_cluster_name,attr,optional"`
	MetadataRefreshInterval string            `river:"metadata_refresh_interval,attr,optional"`
	AllowConcurrent         bool              `river:"allow_concurrency,attr,optional"`
	MaxOffsets              int               `river:"max_offsets,attr,optional"`
	PruneIntervalSeconds    int               `river:"prune_interval_seconds,attr,optional"`
	TopicsFilter            string            `river:"topics_filter_regex,attr,optional"`
	GroupFilter             string            `river:"groups_filter_regex,attr,optional"`
}

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.kafka",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.NewWithTargetBuilder(createExporter, "kafka", customizeTarget),
	})
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	if a.Instance == "" && len(a.KafkaURIs) > 1 {
		return fmt.Errorf("an automatic value for `instance` cannot be determined from %d kafka servers, manually provide one for this component", len(a.KafkaURIs))
	}
	return nil
}

func customizeTarget(baseTarget discovery.Target, args component.Arguments) []discovery.Target {
	a := args.(Arguments)
	target := baseTarget
	if len(a.KafkaURIs) > 1 {
		target["instance"] = a.Instance
	} else {
		target["instance"] = a.KafkaURIs[0]
	}
	return []discovery.Target{target}
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
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
		UseTLS:                  a.UseTLS,
		CAFile:                  a.CAFile,
		CertFile:                a.CertFile,
		KeyFile:                 a.KeyFile,
		InsecureSkipVerify:      a.InsecureSkipVerify,
		KafkaVersion:            a.KafkaVersion,
		UseZooKeeperLag:         a.UseZooKeeperLag,
		ZookeeperURIs:           a.ZookeeperURIs,
		ClusterName:             a.ClusterName,
		MetadataRefreshInterval: a.MetadataRefreshInterval,
		AllowConcurrent:         a.AllowConcurrent,
		MaxOffsets:              a.MaxOffsets,
		PruneIntervalSeconds:    a.PruneIntervalSeconds,
		TopicsFilter:            a.TopicsFilter,
		GroupFilter:             a.GroupFilter,
	}
}

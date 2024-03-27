package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/kafka_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
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
	Instance                string            `alloy:"instance,attr,optional"`
	KafkaURIs               []string          `alloy:"kafka_uris,attr,optional"`
	UseSASL                 bool              `alloy:"use_sasl,attr,optional"`
	UseSASLHandshake        bool              `alloy:"use_sasl_handshake,attr,optional"`
	SASLUsername            string            `alloy:"sasl_username,attr,optional"`
	SASLPassword            alloytypes.Secret `alloy:"sasl_password,attr,optional"`
	SASLMechanism           string            `alloy:"sasl_mechanism,attr,optional"`
	UseTLS                  bool              `alloy:"use_tls,attr,optional"`
	CAFile                  string            `alloy:"ca_file,attr,optional"`
	CertFile                string            `alloy:"cert_file,attr,optional"`
	KeyFile                 string            `alloy:"key_file,attr,optional"`
	InsecureSkipVerify      bool              `alloy:"insecure_skip_verify,attr,optional"`
	KafkaVersion            string            `alloy:"kafka_version,attr,optional"`
	UseZooKeeperLag         bool              `alloy:"use_zookeeper_lag,attr,optional"`
	ZookeeperURIs           []string          `alloy:"zookeeper_uris,attr,optional"`
	ClusterName             string            `alloy:"kafka_cluster_name,attr,optional"`
	MetadataRefreshInterval string            `alloy:"metadata_refresh_interval,attr,optional"`
	AllowConcurrent         bool              `alloy:"allow_concurrency,attr,optional"`
	MaxOffsets              int               `alloy:"max_offsets,attr,optional"`
	PruneIntervalSeconds    int               `alloy:"prune_interval_seconds,attr,optional"`
	TopicsFilter            string            `alloy:"topics_filter_regex,attr,optional"`
	GroupFilter             string            `alloy:"groups_filter_regex,attr,optional"`
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

package kafkatarget

import (
	"github.com/IBM/sarama"
	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/dskit/flagext"
	promconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
)

// Config describes a job to scrape.
type Config struct {
	KafkaConfig    TargetConfig      `mapstructure:"kafka,omitempty" yaml:"kafka,omitempty"`
	RelabelConfigs []*relabel.Config `mapstructure:"relabel_configs,omitempty" yaml:"relabel_configs,omitempty"`
	// List of Docker service discovery configurations.
}

type TargetConfig struct {
	// Labels optionally holds labels to associate with each log line.
	Labels model.LabelSet `yaml:"labels"`

	// UseIncomingTimestamp sets the timestamp to the incoming kafka messages
	// timestamp if it's set.
	UseIncomingTimestamp bool `yaml:"use_incoming_timestamp"`

	// The list of brokers to connect to kafka (Required).
	Brokers []string `yaml:"brokers"`

	// The consumer group id (Required).
	GroupID string `yaml:"group_id"`

	// Kafka Topics to consume (Required).
	Topics []string `yaml:"topics"`

	// Kafka version. Default to 2.2.1
	Version string `yaml:"version"`

	// Rebalancing strategy to use. (e.g. sticky, roundrobin or range)
	Assignor string `yaml:"assignor"`

	// Authentication strategy with Kafka brokers
	Authentication Authentication `yaml:"authentication"`

	MessageParser MessageParser
}

// AuthenticationType specifies method to authenticate with Kafka brokers
type AuthenticationType string

const (
	// AuthenticationTypeNone represents using no authentication
	AuthenticationTypeNone = "none"
	// AuthenticationTypeSSL represents using SSL/TLS to authenticate
	AuthenticationTypeSSL = "ssl"
	// AuthenticationTypeSASL represents using SASL to authenticate
	AuthenticationTypeSASL = "sasl"
)

// Authentication describe the configuration for authentication with Kafka brokers
type Authentication struct {
	// Type is authentication type
	// Possible values: none, sasl and ssl (defaults to none).
	Type AuthenticationType `yaml:"type"`

	// TLSConfig is used for TLS encryption and authentication with Kafka brokers
	TLSConfig promconfig.TLSConfig `yaml:"tls_config,omitempty"`

	// SASLConfig is used for SASL authentication with Kafka brokers
	SASLConfig SASLConfig `yaml:"sasl_config,omitempty"`
}

// TokenProviderType specifies the provider used for resolving the access token
type TokenProviderType string

const (
	// TokenProviderTypeAzure represents using the Azure as the token provider
	TokenProviderTypeAzure TokenProviderType = "azure"
)

// KafkaSASLConfig describe the SASL configuration for authentication with Kafka brokers
type SASLConfig struct {
	// SASL mechanism. Supports PLAIN, SCRAM-SHA-256 and SCRAM-SHA-512
	Mechanism sarama.SASLMechanism `yaml:"mechanism"`

	// SASL Username
	User string `yaml:"user"`

	// SASL Password for the User
	Password flagext.Secret `yaml:"password"`

	// UseTLS sets whether TLS is used with SASL
	UseTLS bool `yaml:"use_tls"`

	// TLSConfig is used for SASL over TLS. It is used only when UseTLS is true
	TLSConfig promconfig.TLSConfig `yaml:",inline"`

	// OAuthConfig is used for configuring the token provider
	OAuthConfig OAuthConfig `yaml:"oauth_provider_config,omitempty"`
}

type OAuthConfig struct {
	// TokenProvider is used for resolving the OAuth access token
	TokenProvider TokenProviderType `yaml:"token_provider,omitempty"`

	Scopes []string
}

// MessageParser defines parsing for each incoming message
type MessageParser interface {
	Parse(message *sarama.ConsumerMessage, labels model.LabelSet, relabels []*relabel.Config, useIncomingTimestamp bool) ([]loki.Entry, error)
}

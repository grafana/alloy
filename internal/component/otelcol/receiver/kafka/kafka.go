// Package kafka provides an otelcol.receiver.kafka component.
package kafka

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.kafka",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := kafkareceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.kafka component.
type Arguments struct {
	Brokers         []string `alloy:"brokers,attr"`
	ProtocolVersion string   `alloy:"protocol_version,attr"`
	Topic           string   `alloy:"topic,attr,optional"`
	Encoding        string   `alloy:"encoding,attr,optional"`
	GroupID         string   `alloy:"group_id,attr,optional"`
	ClientID        string   `alloy:"client_id,attr,optional"`
	InitialOffset   string   `alloy:"initial_offset,attr,optional"`

	ResolveCanonicalBootstrapServersOnly bool `alloy:"resolve_canonical_bootstrap_servers_only,attr,optional"`

	Authentication   AuthenticationArguments `alloy:"authentication,block,optional"`
	Metadata         MetadataArguments       `alloy:"metadata,block,optional"`
	AutoCommit       AutoCommitArguments     `alloy:"autocommit,block,optional"`
	MessageMarking   MessageMarkingArguments `alloy:"message_marking,block,optional"`
	HeaderExtraction HeaderExtraction        `alloy:"header_extraction,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		// We use the defaults from the upstream OpenTelemetry Collector component
		// for compatibility, even though that means using a client and group ID of
		// "otel-collector".

		Encoding:      "otlp_proto",
		Brokers:       []string{"localhost:9092"},
		ClientID:      "otel-collector",
		GroupID:       "otel-collector",
		InitialOffset: "latest",
	}
	args.Metadata.SetToDefault()
	args.AutoCommit.SetToDefault()
	args.MessageMarking.SetToDefault()
	args.HeaderExtraction.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	var signals []string

	if len(args.Topic) > 0 {
		if len(args.Output.Logs) > 0 {
			signals = append(signals, "logs")
		}
		if len(args.Output.Metrics) > 0 {
			signals = append(signals, "metrics")
		}
		if len(args.Output.Traces) > 0 {
			signals = append(signals, "traces")
		}
		if len(signals) > 1 {
			return fmt.Errorf("if the argument topic is specified, only one signal can be set in the output block, current: %s", strings.Join(signals, ", "))
		}
	}
	return nil
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]interface{})
	input["auth"] = args.Authentication.Convert()

	var result kafkareceiver.Config
	err := mapstructure.Decode(input, &result)
	if err != nil {
		return nil, err
	}

	result.Brokers = args.Brokers
	result.ProtocolVersion = args.ProtocolVersion
	result.Topic = args.Topic
	result.Encoding = args.Encoding
	result.GroupID = args.GroupID
	result.ClientID = args.ClientID
	result.InitialOffset = args.InitialOffset
	result.ResolveCanonicalBootstrapServersOnly = args.ResolveCanonicalBootstrapServersOnly
	result.Metadata = args.Metadata.Convert()
	result.AutoCommit = args.AutoCommit.Convert()
	result.MessageMarking = args.MessageMarking.Convert()
	result.HeaderExtraction = args.HeaderExtraction.Convert()

	return &result, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// AuthenticationArguments configures how to authenticate to the Kafka broker.
type AuthenticationArguments struct {
	Plaintext *PlaintextArguments         `alloy:"plaintext,block,optional"`
	SASL      *SASLArguments              `alloy:"sasl,block,optional"`
	TLS       *otelcol.TLSClientArguments `alloy:"tls,block,optional"`
	Kerberos  *KerberosArguments          `alloy:"kerberos,block,optional"`
}

// Convert converts args into the upstream type.
func (args AuthenticationArguments) Convert() map[string]interface{} {
	auth := make(map[string]interface{})

	if args.Plaintext != nil {
		conv := args.Plaintext.Convert()
		auth["plain_text"] = &conv
	}
	if args.SASL != nil {
		conv := args.SASL.Convert()
		auth["sasl"] = &conv
	}
	if args.TLS != nil {
		auth["tls"] = args.TLS.Convert()
	}
	if args.Kerberos != nil {
		conv := args.Kerberos.Convert()
		auth["kerberos"] = &conv
	}

	return auth
}

// PlaintextArguments configures plaintext authentication against the Kafka
// broker.
type PlaintextArguments struct {
	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`
}

// Convert converts args into the upstream type.
func (args PlaintextArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
		"username": args.Username,
		"password": string(args.Password),
	}
}

// SASLArguments configures SASL authentication against the Kafka broker.
type SASLArguments struct {
	Username  string            `alloy:"username,attr"`
	Password  alloytypes.Secret `alloy:"password,attr"`
	Mechanism string            `alloy:"mechanism,attr"`
	Version   int               `alloy:"version,attr,optional"`
	AWSMSK    AWSMSKArguments   `alloy:"aws_msk,block,optional"`
}

// Convert converts args into the upstream type.
func (args SASLArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
		"username":  args.Username,
		"password":  string(args.Password),
		"mechanism": args.Mechanism,
		"version":   args.Version,
		"aws_msk":   args.AWSMSK.Convert(),
	}
}

// AWSMSKArguments exposes additional SASL authentication measures required to
// use the AWS_MSK_IAM mechanism.
type AWSMSKArguments struct {
	Region     string `alloy:"region,attr"`
	BrokerAddr string `alloy:"broker_addr,attr"`
}

// Convert converts args into the upstream type.
func (args AWSMSKArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
		"region":      args.Region,
		"broker_addr": args.BrokerAddr,
	}
}

// KerberosArguments configures Kerberos authentication against the Kafka
// broker.
type KerberosArguments struct {
	ServiceName string            `alloy:"service_name,attr,optional"`
	Realm       string            `alloy:"realm,attr,optional"`
	UseKeyTab   bool              `alloy:"use_keytab,attr,optional"`
	Username    string            `alloy:"username,attr"`
	Password    alloytypes.Secret `alloy:"password,attr,optional"`
	ConfigPath  string            `alloy:"config_file,attr,optional"`
	KeyTabPath  string            `alloy:"keytab_file,attr,optional"`
}

// Convert converts args into the upstream type.
func (args KerberosArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
		"service_name": args.ServiceName,
		"realm":        args.Realm,
		"use_keytab":   args.UseKeyTab,
		"username":     args.Username,
		"password":     string(args.Password),
		"config_file":  args.ConfigPath,
		"keytab_file":  args.KeyTabPath,
	}
}

// MetadataArguments configures how the otelcol.receiver.kafka component will
// retrieve metadata from the Kafka broker.
type MetadataArguments struct {
	IncludeAllTopics bool                   `alloy:"include_all_topics,attr,optional"`
	Retry            MetadataRetryArguments `alloy:"retry,block,optional"`
}

func (args *MetadataArguments) SetToDefault() {
	*args = MetadataArguments{
		IncludeAllTopics: true,
		Retry: MetadataRetryArguments{
			MaxRetries: 3,
			Backoff:    250 * time.Millisecond,
		},
	}
}

// Convert converts args into the upstream type.
func (args MetadataArguments) Convert() kafkaexporter.Metadata {
	return kafkaexporter.Metadata{
		Full:  args.IncludeAllTopics,
		Retry: args.Retry.Convert(),
	}
}

// MetadataRetryArguments configures how to retry retrieving metadata from the
// Kafka broker. Retrying is useful to avoid race conditions when the Kafka
// broker is starting at the same time as the otelcol.receiver.kafka component.
type MetadataRetryArguments struct {
	MaxRetries int           `alloy:"max_retries,attr,optional"`
	Backoff    time.Duration `alloy:"backoff,attr,optional"`
}

// Convert converts args into the upstream type.
func (args MetadataRetryArguments) Convert() kafkaexporter.MetadataRetry {
	return kafkaexporter.MetadataRetry{
		Max:     args.MaxRetries,
		Backoff: args.Backoff,
	}
}

// AutoCommitArguments configures how to automatically commit updated topic
// offsets back to the Kafka broker.
type AutoCommitArguments struct {
	Enable   bool          `alloy:"enable,attr,optional"`
	Interval time.Duration `alloy:"interval,attr,optional"`
}

func (args *AutoCommitArguments) SetToDefault() {
	*args = AutoCommitArguments{
		Enable:   true,
		Interval: time.Second,
	}
}

// Convert converts args into the upstream type.
func (args AutoCommitArguments) Convert() kafkareceiver.AutoCommit {
	return kafkareceiver.AutoCommit{
		Enable:   args.Enable,
		Interval: args.Interval,
	}
}

// MessageMarkingArguments configures when Kafka messages are marked as read.
type MessageMarkingArguments struct {
	AfterExecution      bool `alloy:"after_execution,attr,optional"`
	IncludeUnsuccessful bool `alloy:"include_unsuccessful,attr,optional"`
}

func (args *MessageMarkingArguments) SetToDefault() {
	*args = MessageMarkingArguments{
		AfterExecution:      false,
		IncludeUnsuccessful: false,
	}
}

// Convert converts args into the upstream type.
func (args MessageMarkingArguments) Convert() kafkareceiver.MessageMarking {
	return kafkareceiver.MessageMarking{
		After:   args.AfterExecution,
		OnError: args.IncludeUnsuccessful,
	}
}

type HeaderExtraction struct {
	ExtractHeaders bool     `alloy:"extract_headers,attr,optional"`
	Headers        []string `alloy:"headers,attr,optional"`
}

func (h *HeaderExtraction) SetToDefault() {
	*h = HeaderExtraction{
		ExtractHeaders: false,
		Headers:        []string{},
	}
}

// Convert converts HeaderExtraction into the upstream type.
func (h HeaderExtraction) Convert() kafkareceiver.HeaderExtraction {
	return kafkareceiver.HeaderExtraction{
		ExtractHeaders: h.ExtractHeaders,
		Headers:        h.Headers,
	}
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}

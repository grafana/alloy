package otelcolconvert

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/receiver/kafka"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/river/rivertypes"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
)

func init() {
	converters = append(converters, kafkaReceiverConverter{})
}

type kafkaReceiverConverter struct{}

func (kafkaReceiverConverter) Factory() component.Factory { return kafkareceiver.NewFactory() }

func (kafkaReceiverConverter) InputComponentName() string { return "" }

func (kafkaReceiverConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toKafkaReceiver(state, id, cfg.(*kafkareceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "kafka"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toKafkaReceiver(state *state, id component.InstanceID, cfg *kafkareceiver.Config) *kafka.Arguments {
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
		nextLogs    = state.Next(id, component.DataTypeLogs)
		nextTraces  = state.Next(id, component.DataTypeTraces)
	)

	return &kafka.Arguments{
		Brokers:         cfg.Brokers,
		ProtocolVersion: cfg.ProtocolVersion,
		Topic:           cfg.Topic,
		Encoding:        cfg.Encoding,
		GroupID:         cfg.GroupID,
		ClientID:        cfg.ClientID,
		InitialOffset:   cfg.InitialOffset,

		Authentication:   toKafkaAuthentication(encodeMapstruct(cfg.Authentication)),
		Metadata:         toKafkaMetadata(cfg.Metadata),
		AutoCommit:       toKafkaAutoCommit(cfg.AutoCommit),
		MessageMarking:   toKafkaMessageMarking(cfg.MessageMarking),
		HeaderExtraction: toKafkaHeaderExtraction(cfg.HeaderExtraction),

		DebugMetrics: common.DefaultValue[kafka.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: toTokenizedConsumers(nextMetrics),
			Logs:    toTokenizedConsumers(nextLogs),
			Traces:  toTokenizedConsumers(nextTraces),
		},
	}
}

func toKafkaAuthentication(cfg map[string]any) kafka.AuthenticationArguments {
	spew.Dump(cfg)

	return kafka.AuthenticationArguments{
		Plaintext: toKafkaPlaintext(encodeMapstruct(cfg["plain_text"])),
		SASL:      toKafkaSASL(encodeMapstruct(cfg["sasl"])),
		TLS:       toKafkaTLSClientArguments(encodeMapstruct(cfg["tls"])),
		Kerberos:  toKafkaKerberos(encodeMapstruct(cfg["kerberos"])),
	}
}

func toKafkaPlaintext(cfg map[string]any) *kafka.PlaintextArguments {
	if cfg == nil {
		return nil
	}

	return &kafka.PlaintextArguments{
		Username: cfg["username"].(string),
		Password: rivertypes.Secret(cfg["password"].(string)),
	}
}

func toKafkaSASL(cfg map[string]any) *kafka.SASLArguments {
	if cfg == nil {
		return nil
	}

	return &kafka.SASLArguments{
		Username:  cfg["username"].(string),
		Password:  rivertypes.Secret(cfg["password"].(string)),
		Mechanism: cfg["mechanism"].(string),
		Version:   cfg["version"].(int),
		AWSMSK:    toKafkaAWSMSK(encodeMapstruct(cfg["aws_msk"])),
	}
}

func toKafkaAWSMSK(cfg map[string]any) kafka.AWSMSKArguments {
	if cfg == nil {
		return kafka.AWSMSKArguments{}
	}

	return kafka.AWSMSKArguments{
		Region:     cfg["region"].(string),
		BrokerAddr: cfg["broker_addr"].(string),
	}
}

func toKafkaTLSClientArguments(cfg map[string]any) *otelcol.TLSClientArguments {
	if cfg == nil {
		return nil
	}

	// Convert cfg to configtls.TLSClientSetting
	var tlsSettings configtls.TLSClientSetting
	if err := mapstructure.Decode(cfg, &tlsSettings); err != nil {
		panic(err)
	}

	res := toTLSClientArguments(tlsSettings)
	return &res
}

func toKafkaKerberos(cfg map[string]any) *kafka.KerberosArguments {
	if cfg == nil {
		return nil
	}

	return &kafka.KerberosArguments{
		ServiceName: cfg["service_name"].(string),
		Realm:       cfg["realm"].(string),
		UseKeyTab:   cfg["use_keytab"].(bool),
		Username:    cfg["username"].(string),
		Password:    rivertypes.Secret(cfg["password"].(string)),
		ConfigPath:  cfg["config_file"].(string),
		KeyTabPath:  cfg["keytab_file"].(string),
	}
}

func toKafkaMetadata(cfg kafkaexporter.Metadata) kafka.MetadataArguments {
	return kafka.MetadataArguments{
		IncludeAllTopics: cfg.Full,
		Retry:            toKafkaRetry(cfg.Retry),
	}
}

func toKafkaRetry(cfg kafkaexporter.MetadataRetry) kafka.MetadataRetryArguments {
	return kafka.MetadataRetryArguments{
		MaxRetries: cfg.Max,
		Backoff:    cfg.Backoff,
	}
}

func toKafkaAutoCommit(cfg kafkareceiver.AutoCommit) kafka.AutoCommitArguments {
	return kafka.AutoCommitArguments{
		Enable:   cfg.Enable,
		Interval: cfg.Interval,
	}
}

func toKafkaMessageMarking(cfg kafkareceiver.MessageMarking) kafka.MessageMarkingArguments {
	return kafka.MessageMarkingArguments{
		AfterExecution:      cfg.After,
		IncludeUnsuccessful: cfg.OnError,
	}
}

func toKafkaHeaderExtraction(cfg kafkareceiver.HeaderExtraction) kafka.HeaderExtraction {
	// If cfg.Headers is nil, we set it to an empty slice to align with
	// the default of the Flow component; if this isn't done than default headers
	// will be explicitly set as `[]` in the generated Flow configuration file, which
	// may confuse users.
	if cfg.Headers == nil {
		cfg.Headers = []string{}
	}

	return kafka.HeaderExtraction{
		ExtractHeaders: cfg.ExtractHeaders,
		Headers:        cfg.Headers,
	}
}

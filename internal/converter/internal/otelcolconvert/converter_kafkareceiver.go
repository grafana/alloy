package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/kafka"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, kafkaReceiverConverter{})
}

type kafkaReceiverConverter struct{}

func (kafkaReceiverConverter) Factory() component.Factory { return kafkareceiver.NewFactory() }

func (kafkaReceiverConverter) InputComponentName() string { return "" }

func (kafkaReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toKafkaReceiver(state, id, cfg.(*kafkareceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "kafka"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toKafkaReceiver(state *State, id componentstatus.InstanceID, cfg *kafkareceiver.Config) *kafka.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &kafka.Arguments{
		Brokers:           cfg.Brokers,
		ProtocolVersion:   cfg.ProtocolVersion,
		SessionTimeout:    cfg.SessionTimeout,
		HeartbeatInterval: cfg.HeartbeatInterval,
		Topic:             cfg.Topic,
		Encoding:          cfg.Encoding,
		GroupID:           cfg.GroupID,
		ClientID:          cfg.ClientID,
		InitialOffset:     cfg.InitialOffset,

		ResolveCanonicalBootstrapServersOnly: cfg.ResolveCanonicalBootstrapServersOnly,

		Authentication:   toKafkaAuthentication(encodeMapstruct(cfg.Authentication)),
		Metadata:         toKafkaMetadata(cfg.Metadata),
		AutoCommit:       toKafkaAutoCommit(cfg.AutoCommit),
		MessageMarking:   toKafkaMessageMarking(cfg.MessageMarking),
		HeaderExtraction: toKafkaHeaderExtraction(cfg.HeaderExtraction),

		MinFetchSize:     cfg.MinFetchSize,
		DefaultFetchSize: cfg.DefaultFetchSize,
		MaxFetchSize:     cfg.MaxFetchSize,

		DebugMetrics: common.DefaultValue[kafka.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
	}
}

func toKafkaAuthentication(cfg map[string]any) otelcol.KafkaAuthenticationArguments {
	return otelcol.KafkaAuthenticationArguments{
		Plaintext: toKafkaPlaintext(encodeMapstruct(cfg["plain_text"])),
		SASL:      toKafkaSASL(encodeMapstruct(cfg["sasl"])),
		TLS:       toKafkaTLSClientArguments(encodeMapstruct(cfg["tls"])),
		Kerberos:  toKafkaKerberos(encodeMapstruct(cfg["kerberos"])),
	}
}

func toKafkaPlaintext(cfg map[string]any) *otelcol.KafkaPlaintextArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.KafkaPlaintextArguments{
		Username: cfg["username"].(string),
		Password: alloytypes.Secret(cfg["password"].(string)),
	}
}

func toKafkaSASL(cfg map[string]any) *otelcol.KafkaSASLArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.KafkaSASLArguments{
		Username:  cfg["username"].(string),
		Password:  alloytypes.Secret(cfg["password"].(string)),
		Mechanism: cfg["mechanism"].(string),
		Version:   cfg["version"].(int),
		AWSMSK:    toKafkaAWSMSK(encodeMapstruct(cfg["aws_msk"])),
	}
}

func toKafkaAWSMSK(cfg map[string]any) otelcol.KafkaAWSMSKArguments {
	if cfg == nil {
		return otelcol.KafkaAWSMSKArguments{}
	}

	return otelcol.KafkaAWSMSKArguments{
		Region:     cfg["region"].(string),
		BrokerAddr: cfg["broker_addr"].(string),
	}
}

func toKafkaTLSClientArguments(cfg map[string]any) *otelcol.TLSClientArguments {
	if cfg == nil {
		return nil
	}

	// Convert cfg to configtls.TLSClientSetting
	var tlsSettings configtls.ClientConfig
	if err := mapstructure.Decode(cfg, &tlsSettings); err != nil {
		panic(err)
	}

	res := toTLSClientArguments(tlsSettings)
	return &res
}

func toKafkaKerberos(cfg map[string]any) *otelcol.KafkaKerberosArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.KafkaKerberosArguments{
		ServiceName:     cfg["service_name"].(string),
		Realm:           cfg["realm"].(string),
		UseKeyTab:       cfg["use_keytab"].(bool),
		Username:        cfg["username"].(string),
		Password:        alloytypes.Secret(cfg["password"].(string)),
		ConfigPath:      cfg["config_file"].(string),
		KeyTabPath:      cfg["keytab_file"].(string),
		DisablePAFXFAST: cfg["disable_fast_negotiation"].(bool),
	}
}

func toKafkaMetadata(cfg kafkaexporter.Metadata) otelcol.KafkaMetadataArguments {
	return otelcol.KafkaMetadataArguments{
		IncludeAllTopics: cfg.Full,
		Retry:            toKafkaRetry(cfg.Retry),
	}
}

func toKafkaRetry(cfg kafkaexporter.MetadataRetry) otelcol.KafkaMetadataRetryArguments {
	return otelcol.KafkaMetadataRetryArguments{
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
	// the default of the Alloy component; if this isn't done than default headers
	// will be explicitly set as `[]` in the generated Alloy configuration file, which
	// may confuse users.
	if cfg.Headers == nil {
		cfg.Headers = []string{}
	}

	return kafka.HeaderExtraction{
		ExtractHeaders: cfg.ExtractHeaders,
		Headers:        cfg.Headers,
	}
}

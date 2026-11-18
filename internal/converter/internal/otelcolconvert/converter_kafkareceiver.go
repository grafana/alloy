package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/kafka"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configretry"
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
	overrideHook := func(val any) any {
		switch value := val.(type) {
		case extension.ExtensionHandler:
			ext := state.LookupExtension(value.ID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	kafkaCfg := cfg.(*kafkareceiver.Config)
	for _, enc := range []string{kafkaCfg.Logs.Encoding, kafkaCfg.Metrics.Encoding, kafkaCfg.Traces.Encoding} {
		if enc == "azure_resource_logs" {
			diags.Add(
				diag.SeverityLevelWarn,
				fmt.Sprintf("%s: the azure_resource_logs encoding is deprecated, consider using otelcol.encoding.azure with encoding_extension instead", StringifyInstanceID(id)),
			)
			break
		}
	}

	args := toKafkaReceiver(state, id, kafkaCfg)
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "receiver", "kafka"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toKafkaReceiver(state *State, id componentstatus.InstanceID, cfg *kafkareceiver.Config) *kafka.Arguments {
	var (
		defaults    = kafkareceiver.NewFactory().CreateDefaultConfig().(*kafkareceiver.Config)
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	var tlsCfgPtr *otelcol.TLSClientArguments
	if cfg.ClientConfig.TLS != nil {
		tlsCfg := toTLSClientArguments(*cfg.ClientConfig.TLS)
		tlsCfgPtr = &tlsCfg
	}

	rebalanceStrategies := toKafkaRebalance(cfg.ConsumerConfig)
	return &kafka.Arguments{
		Brokers:           cfg.ClientConfig.Brokers,
		ProtocolVersion:   cfg.ClientConfig.ProtocolVersion,
		SessionTimeout:    cfg.ConsumerConfig.SessionTimeout,
		HeartbeatInterval: cfg.ConsumerConfig.HeartbeatInterval,
		GroupID:           cfg.ConsumerConfig.GroupID,
		ClientID:          cfg.ClientConfig.ClientID,
		InitialOffset:     cfg.ConsumerConfig.InitialOffset,
		ConnIdleTimeout:   cfg.ClientConfig.ConnIdleTimeout,
		DialTimeout:       cfg.ClientConfig.DialTimeout,

		// ResolveCanonicalBootstrapServersOnly is deprecated and no longer exists upstream to read back.

		Authentication:      toKafkaAuthentication(encodeMapstruct(cfg.ClientConfig.Authentication)),
		Metadata:            toKafkaMetadata(cfg.ClientConfig.Metadata),
		AutoCommit:          toKafkaAutoCommit(cfg.ConsumerConfig.AutoCommit),
		MessageMarking:      toKafkaMessageMarking(cfg.MessageMarking),
		HeaderExtraction:    toKafkaHeaderExtraction(cfg.HeaderExtraction),
		PartitionProcessing: toKafkaPartitionProcessing(cfg.PartitionProcessing),

		TLS: tlsCfgPtr,

		Logs:    toKafkaTopicEncodingConfig(state, cfg.Logs, defaults.Logs),
		Metrics: toKafkaTopicEncodingConfig(state, cfg.Metrics, defaults.Metrics),
		Traces:  toKafkaTopicEncodingConfig(state, cfg.Traces, defaults.Traces),

		MinFetchSize:             cfg.ConsumerConfig.MinFetchSize,
		MaxFetchSize:             cfg.ConsumerConfig.MaxFetchSize,
		MaxPartitionFetchSize:    cfg.ConsumerConfig.MaxPartitionFetchSize,
		MaxFetchWait:             cfg.ConsumerConfig.MaxFetchWait,
		MaxDecompressBatchBytes:  cfg.ConsumerConfig.MaxDecompressBatchBytes,
		RackID:                   cfg.ClientConfig.RackID,
		UseLeaderEpoch:           cfg.ClientConfig.UseLeaderEpoch,
		GroupRebalanceStrategies: rebalanceStrategies,
		GroupInstanceID:          cfg.ConsumerConfig.GroupInstanceID,

		ErrorBackOff: toKafkaErrorBackOff(cfg.ErrorBackOff),

		DebugMetrics: common.DefaultValue[kafka.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
	}
}

func toKafkaErrorBackOff(cfg configretry.BackOffConfig) kafka.ErrorBackOffArguments {
	return kafka.ErrorBackOffArguments{
		Enabled:             cfg.Enabled,
		InitialInterval:     cfg.InitialInterval,
		RandomizationFactor: cfg.RandomizationFactor,
		Multiplier:          cfg.Multiplier,
		MaxInterval:         cfg.MaxInterval,
		MaxElapsedTime:      cfg.MaxElapsedTime,
	}
}

func toKafkaTopicEncodingConfig(state *State, cfg, defaults kafkareceiver.TopicEncodingConfig) kafka.KafkaReceiverTopicEncodingConfig {
	out := kafka.KafkaReceiverTopicEncodingConfig{
		Topics:        cfg.Topics,
		ExcludeTopics: cfg.ExcludeTopics,
	}

	// Upstream treats an encoding naming a configured extension as that
	// extension, taking precedence over built-in encodings.
	var extID component.ID
	if err := extID.UnmarshalText([]byte(cfg.Encoding)); err == nil {
		if _, ok := state.TryLookupExtension(extID); ok {
			out.EncodingExtension = &extension.ExtensionHandler{ID: extID}
			return out
		}
	}

	// Leave the upstream default unset to keep the output minimal.
	if cfg.Encoding != defaults.Encoding {
		out.Encoding = &cfg.Encoding
	}
	return out
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
		// Version is deprecated and no longer exists upstream to read back.
		AWSMSK: toKafkaAWSMSK(encodeMapstruct(cfg["aws_msk"])),
	}
}

func toKafkaAWSMSK(cfg map[string]any) otelcol.KafkaAWSMSKArguments {
	if cfg == nil {
		return otelcol.KafkaAWSMSKArguments{}
	}

	return otelcol.KafkaAWSMSKArguments{
		Region: cfg["region"].(string),
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

func toKafkaMetadata(cfg configkafka.MetadataConfig) otelcol.KafkaMetadataArguments {
	return otelcol.KafkaMetadataArguments{
		Full:            cfg.Full,
		RefreshInterval: cfg.RefreshInterval,
		Retry:           toKafkaRetry(cfg.Retry),
	}
}

func toKafkaRetry(cfg configkafka.MetadataRetryConfig) otelcol.KafkaMetadataRetryArguments {
	return otelcol.KafkaMetadataRetryArguments{
		MaxRetries: cfg.Max,
		Backoff:    cfg.Backoff,
	}
}

func toKafkaAutoCommit(cfg configkafka.AutoCommitConfig) kafka.AutoCommitArguments {
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

func toKafkaPartitionProcessing(cfg kafkareceiver.PartitionProcessing) kafka.PartitionProcessingArguments {
	// Start from the defaults so the deprecated max_buffered_batches keeps its default.
	var args kafka.PartitionProcessingArguments
	args.SetToDefault()
	args.Independent = cfg.Independent
	args.MaxPrefetch = kafka.MaxPrefetchArguments{
		Batches: cfg.MaxPrefetch.Batches,
		Bytes:   cfg.MaxPrefetch.Bytes,
	}
	args.MaxInFlight = kafka.MaxInFlightArguments{
		Records: cfg.MaxInFlight.Records,
		Bytes:   cfg.MaxInFlight.Bytes,
		Window:  cfg.MaxInFlight.Window,
	}
	return args
}

// toKafkaRebalance always returns the plural strategies form; upstream removed the singular
// GroupRebalanceStrategy field, so the deprecated singular Alloy argument is never populated here.
func toKafkaRebalance(cfg configkafka.ConsumerConfig) []string {
	if len(cfg.GroupRebalanceStrategies) > 0 {
		strategies := make([]string, 0, len(cfg.GroupRebalanceStrategies))
		for _, s := range cfg.GroupRebalanceStrategies {
			strategies = append(strategies, string(s))
		}
		return strategies
	}
	return []string{string(configkafka.CooperativeStickyBalanceStrategy)}
}

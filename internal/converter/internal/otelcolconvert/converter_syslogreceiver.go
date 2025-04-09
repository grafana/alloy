package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/syslog"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, syslogReceiverConverter{})
}

type syslogReceiverConverter struct{}

func (syslogReceiverConverter) Factory() component.Factory {
	return syslogreceiver.NewFactory()
}

func (syslogReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.syslog"
}

func (syslogReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOtelcolReceiversyslog(cfg.(*syslogreceiver.SysLogConfig))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "syslog"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolReceiversyslog(cfg *syslogreceiver.SysLogConfig) *syslog.Arguments {
	args := &syslog.Arguments{
		Protocol:            config.SysLogFormat(cfg.InputConfig.Protocol),
		Location:            cfg.InputConfig.Location,
		EnableOctetCounting: cfg.InputConfig.EnableOctetCounting,
		AllowSkipPriHeader:  cfg.InputConfig.AllowSkipPriHeader,
		MaxOctets:           cfg.InputConfig.MaxOctets,
		DebugMetrics:        common.DefaultValue[syslog.Arguments]().DebugMetrics,
		OnError:             cfg.InputConfig.OnError,
	}

	if cfg.InputConfig.NonTransparentFramingTrailer != nil {
		trailer := syslog.FramingTrailer(*cfg.InputConfig.NonTransparentFramingTrailer)
		args.NonTransparentFramingTrailer = &trailer
	}

	if cfg.InputConfig.TCP != nil {
		args.TCP = &syslog.TCP{
			MaxLogSize:      units.Base2Bytes(cfg.InputConfig.TCP.MaxLogSize),
			ListenAddress:   cfg.InputConfig.TCP.ListenAddress,
			TLS:             toTLSServerArguments(cfg.InputConfig.TCP.TLS),
			AddAttributes:   cfg.InputConfig.TCP.AddAttributes,
			OneLogPerPacket: cfg.InputConfig.TCP.OneLogPerPacket,
			Encoding:        cfg.InputConfig.TCP.Encoding,
			MultilineConfig: toOtelcolMultilineConfig(cfg.InputConfig.TCP.SplitConfig),
			TrimConfig:      toOtelcolTrimConfig(cfg.InputConfig.TCP.TrimConfig),
		}
	}

	if cfg.InputConfig.UDP != nil {
		args.UDP = &syslog.UDP{
			ListenAddress:   cfg.InputConfig.UDP.ListenAddress,
			OneLogPerPacket: cfg.InputConfig.UDP.OneLogPerPacket,
			AddAttributes:   cfg.InputConfig.UDP.AddAttributes,
			Encoding:        cfg.InputConfig.UDP.Encoding,
			MultilineConfig: toOtelcolMultilineConfig(cfg.InputConfig.UDP.SplitConfig),
			TrimConfig:      toOtelcolTrimConfig(cfg.InputConfig.UDP.TrimConfig),
			Async:           toSyslogAsyncConfig(cfg.InputConfig.UDP.AsyncConfig),
		}
	}

	// This isn't done in a function because the type is not exported
	args.ConsumerRetry = otelcol.ConsumerRetryArguments{
		Enabled:         cfg.RetryOnFailure.Enabled,
		InitialInterval: cfg.RetryOnFailure.InitialInterval,
		MaxInterval:     cfg.RetryOnFailure.MaxInterval,
		MaxElapsedTime:  cfg.RetryOnFailure.MaxElapsedTime,
	}

	return args
}

func toOtelcolMultilineConfig(cfg split.Config) *otelcol.MultilineConfig {
	return &otelcol.MultilineConfig{
		LineStartPattern: cfg.LineStartPattern,
		LineEndPattern:   cfg.LineEndPattern,
		OmitPattern:      cfg.OmitPattern,
	}
}

func toOtelcolTrimConfig(cfg trim.Config) *otelcol.TrimConfig {
	return &otelcol.TrimConfig{
		PreserveLeadingWhitespace:  cfg.PreserveLeading,
		PreserveTrailingWhitespace: cfg.PreserveTrailing,
	}
}

func toSyslogAsyncConfig(cfg *udp.AsyncConfig) *syslog.AsyncConfig {
	if cfg == nil {
		return nil
	}

	return &syslog.AsyncConfig{
		Readers:        cfg.Readers,
		Processors:     cfg.Processors,
		MaxQueueLength: cfg.MaxQueueLength,
	}
}

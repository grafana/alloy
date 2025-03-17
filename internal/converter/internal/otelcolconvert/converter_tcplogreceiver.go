package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/tcplog"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, tcplogReceiverConverter{})
}

type tcplogReceiverConverter struct{}

func (tcplogReceiverConverter) Factory() component.Factory {
	return tcplogreceiver.NewFactory()
}

func (tcplogReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.tcplog"
}

func (tcplogReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOtelcolReceivertcplog(cfg.(*tcplogreceiver.TCPLogConfig))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "tcplog"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolReceivertcplog(cfg *tcplogreceiver.TCPLogConfig) *tcplog.Arguments {
	args := &tcplog.Arguments{
		ListenAddress:   cfg.InputConfig.ListenAddress,
		MaxLogSize:      units.Base2Bytes(cfg.InputConfig.MaxLogSize),
		TLS:             toTLSServerArguments(cfg.InputConfig.TLS),
		AddAttributes:   cfg.InputConfig.AddAttributes,
		OneLogPerPacket: cfg.InputConfig.OneLogPerPacket,
		Encoding:        cfg.InputConfig.Encoding,
		MultilineConfig: totcplogMultilineConfig(cfg.InputConfig.SplitConfig),
		DebugMetrics:    common.DefaultValue[tcplog.Arguments]().DebugMetrics,
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

func totcplogMultilineConfig(cfg split.Config) *tcplog.MultilineConfig {
	return &tcplog.MultilineConfig{
		LineStartPattern: cfg.LineStartPattern,
		LineEndPattern:   cfg.LineEndPattern,
		OmitPattern:      cfg.OmitPattern,
	}
}

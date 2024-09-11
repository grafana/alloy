package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/extension/jaeger_remote_sampling"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, jaegerRemoteSamplingExtensionConverter{})
}

type jaegerRemoteSamplingExtensionConverter struct{}

func (jaegerRemoteSamplingExtensionConverter) Factory() component.Factory {
	return jaegerremotesampling.NewFactory()
}

func (jaegerRemoteSamplingExtensionConverter) InputComponentName() string {
	return "otelcol.extension.jaeger_remote_sampling"
}

func (jaegerRemoteSamplingExtensionConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toJaegerRemoteSamplingExtension(cfg.(*jaegerremotesampling.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "extension", "jaeger_remote_sampling"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toJaegerRemoteSamplingExtension(cfg *jaegerremotesampling.Config) *jaeger_remote_sampling.Arguments {
	if cfg == nil {
		return nil
	}

	var grpc *jaeger_remote_sampling.GRPCServerArguments
	if cfg.GRPCServerConfig != nil {
		grpc = (*jaeger_remote_sampling.GRPCServerArguments)(toGRPCServerArguments(cfg.GRPCServerConfig))
	}
	var http *jaeger_remote_sampling.HTTPServerArguments
	if cfg.HTTPServerConfig != nil {
		http = (*jaeger_remote_sampling.HTTPServerArguments)(toHTTPServerArguments(cfg.HTTPServerConfig))
	}
	var remote *jaeger_remote_sampling.GRPCClientArguments
	if cfg.Source.Remote != nil {
		r := toGRPCClientArguments(*cfg.Source.Remote)
		remote = (*jaeger_remote_sampling.GRPCClientArguments)(&r)
	}

	return &jaeger_remote_sampling.Arguments{
		GRPC: grpc,
		HTTP: http,
		Source: jaeger_remote_sampling.ArgumentsSource{
			Content:        "",
			Remote:         remote,
			File:           cfg.Source.File,
			ReloadInterval: cfg.Source.ReloadInterval,
		},
		DebugMetrics: common.DefaultValue[jaeger_remote_sampling.Arguments]().DebugMetrics,
	}
}

package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/auth/sigv4"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, sigV4AuthExtensionConverter{})
}

type sigV4AuthExtensionConverter struct{}

func (sigV4AuthExtensionConverter) Factory() component.Factory {
	return sigv4authextension.NewFactory()
}

func (sigV4AuthExtensionConverter) InputComponentName() string {
	return "otelcol.auth.sigv4"
}

func (sigV4AuthExtensionConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toSigV4AuthExtension(cfg.(*sigv4authextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "auth", "sigv4"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)

	return diags
}

func toSigV4AuthExtension(cfg *sigv4authextension.Config) *sigv4.Arguments {
	return &sigv4.Arguments{
		Region:  cfg.Region,
		Service: cfg.Service,
		AssumeRole: sigv4.AssumeRole{
			ARN:         cfg.AssumeRole.ARN,
			SessionName: cfg.AssumeRole.SessionName,
			STSRegion:   cfg.AssumeRole.STSRegion,
		},
		DebugMetrics: common.DefaultValue[sigv4.Arguments]().DebugMetrics,
	}
}

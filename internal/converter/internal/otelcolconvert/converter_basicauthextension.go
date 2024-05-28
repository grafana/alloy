package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/auth/basic"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, basicAuthConverterConverter{})
}

type basicAuthConverterConverter struct{}

func (basicAuthConverterConverter) Factory() component.Factory {
	return basicauthextension.NewFactory()
}

func (basicAuthConverterConverter) InputComponentName() string { return "otelcol.auth.basic" }

func (basicAuthConverterConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toBasicAuthExtension(cfg.(*basicauthextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "auth", "basic"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toBasicAuthExtension(cfg *basicauthextension.Config) *basic.Arguments {
	return &basic.Arguments{
		Username:     cfg.ClientAuth.Username,
		Password:     alloytypes.Secret(string(cfg.ClientAuth.Password)),
		DebugMetrics: common.DefaultValue[basic.Arguments]().DebugMetrics,
	}
}

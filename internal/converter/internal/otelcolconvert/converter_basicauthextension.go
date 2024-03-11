package otelcolconvert

import (
	"fmt"

	"github.com/grafana/agent/internal/component/otelcol/auth/basic"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/river/rivertypes"
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

func (basicAuthConverterConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toBasicAuthExtension(cfg.(*basicauthextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "auth", "basic"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toBasicAuthExtension(cfg *basicauthextension.Config) *basic.Arguments {
	return &basic.Arguments{
		Username: cfg.ClientAuth.Username,
		Password: rivertypes.Secret(string(cfg.ClientAuth.Password)),
	}
}

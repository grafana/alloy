package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/auth/headers"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, headersSetterExtensionConverter{})
}

type headersSetterExtensionConverter struct{}

func (headersSetterExtensionConverter) Factory() component.Factory {
	return headerssetterextension.NewFactory()
}

func (headersSetterExtensionConverter) InputComponentName() string { return "otelcol.auth.headers" }

func (headersSetterExtensionConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toHeadersSetterExtension(cfg.(*headerssetterextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "auth", "headers"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toHeadersSetterExtension(cfg *headerssetterextension.Config) *headers.Arguments {
	res := make([]headers.Header, 0, len(cfg.HeadersConfig))
	for _, h := range cfg.HeadersConfig {
		var val *alloytypes.OptionalSecret
		if h.Value != nil {
			val = &alloytypes.OptionalSecret{
				IsSecret: false, // we default to non-secret so that the converted configuration includes the actual value instead of (secret).
				Value:    *h.Value,
			}
		}

		res = append(res, headers.Header{
			Key:           *h.Key, // h.Key cannot be nil or it's not valid configuration for the upstream component.
			Value:         val,
			FromContext:   h.FromContext,
			FromAttribute: h.FromAttribute,
			Action:        headers.Action(h.Action),
		})
	}

	return &headers.Arguments{
		Headers:      res,
		DebugMetrics: common.DefaultValue[headers.Arguments]().DebugMetrics,
	}
}

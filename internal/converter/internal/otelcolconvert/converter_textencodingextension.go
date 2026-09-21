package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/encoding/text"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, textEncodingExtensionConverter{})
}

type textEncodingExtensionConverter struct{}

func (textEncodingExtensionConverter) Factory() component.Factory {
	return textencodingextension.NewFactory()
}

func (textEncodingExtensionConverter) InputComponentName() string {
	return "otelcol.encoding.text"
}

func (textEncodingExtensionConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := text.ArgumentsFromConfig(cfg.(*textencodingextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "encoding", "text"}, label, &args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/encoding/azure"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, azureEncodingExtensionConverter{})
}

type azureEncodingExtensionConverter struct{}

func (azureEncodingExtensionConverter) Factory() component.Factory {
	return azureencodingextension.NewFactory()
}

func (azureEncodingExtensionConverter) InputComponentName() string {
	return "otelcol.encoding.azure"
}

func (azureEncodingExtensionConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := azure.ArgumentsFromConfig(cfg.(*azureencodingextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "encoding", "azure"}, label, &args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

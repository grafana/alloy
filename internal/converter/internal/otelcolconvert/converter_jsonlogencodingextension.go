package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/encoding/jsonlog"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, jsonLogEncodingExtensionConverter{})
}

type jsonLogEncodingExtensionConverter struct{}

func (jsonLogEncodingExtensionConverter) Factory() component.Factory {
	return jsonlogencodingextension.NewFactory()
}

func (jsonLogEncodingExtensionConverter) InputComponentName() string {
	return "otelcol.encoding.jsonlog"
}

func (jsonLogEncodingExtensionConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := jsonlog.ArgumentsFromConfig(cfg.(*jsonlogencodingextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "encoding", "jsonlog"}, label, &args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

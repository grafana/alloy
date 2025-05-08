// converter_influxdbreceiver.go
package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/influxdb"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

// Assume State and other dependencies are defined elsewhere

func init() {
	// Register the influxdb receiver converter
	converters = append(converters, influxdbReceiverConverter{})
}

// influxdbReceiverConverter implements the logic to convert influxdb configurations
type influxdbReceiverConverter struct{}

// Factory returns the factory for the influxdb receiver
func (influxdbReceiverConverter) Factory() component.Factory {
	return influxdbreceiver.NewFactory()
}

// InputComponentName returns an empty string since no specific input component name is required
func (influxdbReceiverConverter) InputComponentName() string {
	return ""
}

// ConvertAndAppend converts the influxdb receiver configuration and appends it to the state
func (influxdbReceiverConverter) ConvertAndAppend(
	state *State,
	id componentstatus.InstanceID,
	cfg component.Config,
) diag.Diagnostics {

	var diags diag.Diagnostics

	// Generate a label for the converted component
	label := state.AlloyComponentLabel()

	// Convert the config into Arguments format
	args := toInfluxdbReceiver(state, id, cfg.(*influxdbreceiver.Config))

	// // Create a block with the converted arguments
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "influxdb"}, label, args)
	// Append the block to the state directly
	state.Body().AppendBlock(block)

	// Add a diagnostic log entry for the conversion
	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	return diags
}

// toInfluxdbReceiver converts the influxdbreceiver.Config to influxdb.Arguments
func toInfluxdbReceiver(
	state *State,
	id componentstatus.InstanceID,
	cfg *influxdbreceiver.Config,
) *influxdb.Arguments {

	metricsConsumers := ToTokenizedConsumers(state.Next(id, pipeline.SignalMetrics))

	args := &influxdb.Arguments{
		HTTPServer:   *toHTTPServerArguments(&cfg.ServerConfig),
		DebugMetrics: common.DefaultValue[influxdb.Arguments]().DebugMetrics,
		Output: &otelcol.ConsumerArguments{
			Metrics: metricsConsumers,
		},
	}
	return args
}

package otelcolconvert

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/awss3"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func init() {
	converters = append(converters, awss3ReceiverConverter{})
}

type awss3ReceiverConverter struct{}

func (awss3ReceiverConverter) Factory() component.Factory {
	return awss3receiver.NewFactory()
}

func (awss3ReceiverConverter) InputComponentName() string { return "" }

func (awss3ReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	label := state.AlloyComponentLabel()

	args, diags := toAWSS3Receiver(state, id, cfg.(*awss3receiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "awss3"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toAWSS3Receiver(state *State, id componentstatus.InstanceID, cfg *awss3receiver.Config) (*awss3.Arguments, diag.Diagnostics) {
	var diags diag.Diagnostics
	nextTraces := state.Next(id, pipeline.SignalTraces)

	// TODO(x1unix): remove warning when Encodings support will be implemented (#4938).
	if len(cfg.Encodings) > 0 {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("%s: encodings are not supported at this moment", StringifyInstanceID(id)),
		)
	}

	if cfg.Notifications.OpAMP != nil {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("%s: notifications.opampextension field is not supported", StringifyInstanceID(id)),
		)
	}

	args := awss3.ArgumentsFromConfig(cfg)
	args.Output = &otelcol.ConsumerArguments{
		Traces: ToTokenizedConsumers(nextTraces),
	}

	return &args, diags
}

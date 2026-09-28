package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
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
	overrideHook := func(val any) any {
		switch value := val.(type) {
		case extension.ExtensionHandler:
			ext := state.LookupExtension(value.ID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args, diags := toAWSS3Receiver(state, id, cfg.(*awss3receiver.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "receiver", "awss3"}, label, args, overrideHook)

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
	nextLogs := state.Next(id, pipeline.SignalLogs)
	nextMetrics := state.Next(id, pipeline.SignalMetrics)

	if cfg.Notifications.OpAMP != nil {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("%s: notifications.opampextension field is not supported", StringifyInstanceID(id)),
		)
	}

	args := awss3.ArgumentsFromConfig(cfg)
	args.Output = &otelcol.ConsumerArguments{
		Traces:  ToTokenizedConsumers(nextTraces),
		Metrics: ToTokenizedConsumers(nextMetrics),
		Logs:    ToTokenizedConsumers(nextLogs),
	}

	return &args, diags
}

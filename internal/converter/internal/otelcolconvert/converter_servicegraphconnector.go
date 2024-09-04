package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/connector/servicegraph"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, servicegraphConnectorConverter{})
}

type servicegraphConnectorConverter struct{}

func (servicegraphConnectorConverter) Factory() component.Factory {
	return servicegraphconnector.NewFactory()
}

func (servicegraphConnectorConverter) InputComponentName() string {
	return "otelcol.connector.servicegraph"
}

func (servicegraphConnectorConverter) ConvertAndAppend(state *State, id *componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toServicegraphConnector(state, id, cfg.(*servicegraphconnector.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "connector", "servicegraph"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toServicegraphConnector(state *State, id *componentstatus.InstanceID, cfg *servicegraphconnector.Config) *servicegraph.Arguments {
	if cfg == nil {
		return nil
	}
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
	)

	return &servicegraph.Arguments{
		LatencyHistogramBuckets: cfg.LatencyHistogramBuckets,
		Dimensions:              cfg.Dimensions,
		Store: servicegraph.StoreConfig{
			MaxItems: cfg.Store.MaxItems,
			TTL:      cfg.Store.TTL,
		},
		CacheLoop:             cfg.CacheLoop,
		StoreExpirationLoop:   cfg.StoreExpirationLoop,
		MetricsFlushInterval:  cfg.MetricsFlushInterval,
		DatabaseNameAttribute: cfg.DatabaseNameAttribute,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},
		DebugMetrics: common.DefaultValue[servicegraph.Arguments]().DebugMetrics,
	}
}

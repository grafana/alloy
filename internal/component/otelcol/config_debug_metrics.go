package otelcol

// DebugMetricsArguments configures internal metrics of the components
type DebugMetricsArguments struct {
	DisableHighCardinalityMetrics bool `alloy:"disable_high_cardinality_metrics,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *DebugMetricsArguments) SetToDefault() {
	*args = DebugMetricsArguments{
		DisableHighCardinalityMetrics: true,
	}
}

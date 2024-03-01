package otelcol

// DebugMetricsArguments configures internal metrics of the components
type DebugMetricsArguments struct {
	DisableHighCardinalityMetrics bool `river:"disable_high_cardinality_metrics,attr,optional"`
}

// DefaultDebugMetricsArguments holds default settings for DebugMetricsArguments.
var DefaultDebugMetricsArguments = DebugMetricsArguments{
	DisableHighCardinalityMetrics: true,
}

// SetToDefault implements river.Defaulter.
func (args *DebugMetricsArguments) SetToDefault() {
	*args = DefaultDebugMetricsArguments
}

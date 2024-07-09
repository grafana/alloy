package otelfeaturegatefix

import "go.opentelemetry.io/collector/featuregate"

func init() {
	// Override the default behavior of the feature gate to not panic when a gate is already registered.
	// TODO: Remove this once https://github.com/prometheus/prometheus/issues/13842 is completed and we upgraded Prometheus.
	featuregate.GlobalRegistry().SetAlreadyRegisteredErrHandler(
		func(g *featuregate.Gate, err error) *featuregate.Gate {
			return g
		},
	)
}

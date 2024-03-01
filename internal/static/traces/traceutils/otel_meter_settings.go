package traceutils

import (
	"github.com/prometheus/client_golang/prometheus"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/sdk/metric"
)

// This function is used by both production code and unit tests.
// It makes sure that uint tests use the same conventions for metric readers as production code.
func PrometheusExporter(reg prometheus.Registerer) (otelmetric.Reader, error) {
	return otelprom.New(
		otelprom.WithRegisterer(reg),
		otelprom.WithoutUnits(),
		otelprom.WithoutScopeInfo(),
		otelprom.WithoutTargetInfo(),
		otelprom.WithNamespace("traces"))
}

package batch_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/internal/shapecheck"
	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"go.opentelemetry.io/collector/processor/batchprocessor"
)

// Fails when upstream adds, drops or renames a field Arguments doesn't account for.
func TestArgumentsShape(t *testing.T) {
	shapecheck.Check(t, batchprocessor.Config{}, batch.Arguments{}, shapecheck.Allow{
		AlloyOnly: map[string]string{
			"output":        "Alloy-only",
			"debug_metrics": "Alloy-only",
		},
	})
}

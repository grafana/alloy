package tail_sampling_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/internal/shapecheck"
	"github.com/grafana/alloy/internal/component/otelcol/processor/tail_sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
)

// Fails when upstream adds, drops or renames a field Arguments doesn't account for.
func TestArgumentsShape(t *testing.T) {
	const notInAnd = "Alloy doesn't expose a not policy inside and or drop sub-policies"
	shapecheck.Check(t, tailsamplingprocessor.Config{}, tail_sampling.Arguments{}, shapecheck.Allow{
		Rename: map[string]string{"policies": "policy"},
		Skip: map[string]string{
			"tail_storage":      "Alloy doesn't expose tail storage extensions",
			"sampling_strategy": "Alloy doesn't expose sampling_strategy",

			"policies.and.and_sub_policy.not":                                notInAnd,
			"policies.drop.drop_sub_policy.not":                              notInAnd,
			"policies.composite.composite_sub_policy.and.and_sub_policy.not": notInAnd,
		},
		AlloyOnly: map[string]string{
			"output":        "Alloy-only",
			"debug_metrics": "Alloy-only",
		},
	})
}

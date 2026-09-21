package otelcol

import "github.com/grafana/alloy/internal/featuregate"

// StabilityValidator is implemented by component Arguments that gate part of
// their configuration behind a minimum stability level. This is currently
// only wired up for processors.
type StabilityValidator interface {
	ValidateStabilityLevel(minStability featuregate.Stability) error
}

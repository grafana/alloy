package otelcol

import "github.com/grafana/alloy/internal/featuregate"

// StabilityValidator is implemented by component Arguments that gate part of
// their configuration behind a minimum stability level, for example an
// attribute mirroring an experimental upstream feature. The processor
// wrapper checks for it in its shared Update method, so it fires on both
// initial start and config changes with no other wiring needed. The
// receiver, exporter, connector, auth, and extension wrappers don't check
// for it yet.
type StabilityValidator interface {
	// ValidateStabilityLevel reports an error if the Arguments use any
	// setting that requires a higher stability level than minStability.
	ValidateStabilityLevel(minStability featuregate.Stability) error
}

package gather

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
)

// File is a single entry in a support bundle.
type File struct {
	Path    string
	Content []byte
}

// Options controls how a gatherer collects diagnostics.
type Options struct {
	// Duration is the collection window for windowed collectors, such as the CPU
	// profile and mutex sampling. A zero value skips the CPU profile. The mutex
	// and block profiles are always emitted.
	Duration time.Duration

	// BuildInfo describes the running collector build.
	BuildInfo component.BuildInfo

	// ResourceAttributes holds the collector's telemetry resource attributes.
	ResourceAttributes map[string]string

	// StartTime is the time the extension started.
	StartTime time.Time
}

// Gatherer collects a set of diagnostic files at a single point in time.
type Gatherer interface {
	// Name returns a short identifier used in error reports.
	Name() string

	// Gather collects the diagnostic files.
	Gather(ctx context.Context, opts Options) ([]File, error)
}

// FinishFunc stops an async collection and returns the collected files.
type FinishFunc func(ctx context.Context) ([]File, error)

// AsyncGatherer collects diagnostics over a time window. The orchestrator
// starts every async gatherer, waits for one shared window, then calls each
// returned FinishFunc. This lets windowed collectors, such as the CPU profile
// and log capture, run at the same time over the same window.
type AsyncGatherer interface {
	// Name returns a short identifier used in error reports.
	Name() string

	// Start begins collection and returns a function to finish it.
	Start(ctx context.Context, opts Options) (FinishFunc, error)
}

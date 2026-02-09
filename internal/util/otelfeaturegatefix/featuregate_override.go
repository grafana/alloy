package otelfeaturegatefix

// This package previously overrode the feature gate registry's error handler
// to avoid panics when a gate was already registered. This was needed because
// Prometheus used to bundle its own copy of the featuregate package.
//
// The upstream issue (prometheus/prometheus#13842) has been resolved, and
// Prometheus now uses the standard go.opentelemetry.io/collector/featuregate
// package directly. This file is kept as an empty placeholder; it can be
// removed entirely in a future cleanup.

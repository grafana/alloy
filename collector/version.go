package main

import (
	"github.com/grafana/alloy/internal/build"
)

// CollectorVersion returns the version reported by the OTel Collector
// distribution. It follows the Alloy build version so every version source stays
// consistent — including builds with a pinned VERSION (e.g. integration tests),
// where native otelcol components then report that same version. build.Version is
// already normalized to include a "v" prefix.
func CollectorVersion() string {
	return build.Version
}

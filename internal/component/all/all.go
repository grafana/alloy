// Package all imports all known component packages.
package all

import (
	// Trimmed to the components actually used by this distribution.
	// See docs/superpowers/specs/2026-06-16-slim-collector-distro-design.md
	_ "github.com/grafana/alloy/internal/component/discovery/relabel"        // Import discovery.relabel
	_ "github.com/grafana/alloy/internal/component/local/file_match"         // Import local.file_match
	_ "github.com/grafana/alloy/internal/component/loki/process"             // Import loki.process
	_ "github.com/grafana/alloy/internal/component/loki/source/file"         // Import loki.source.file
	_ "github.com/grafana/alloy/internal/component/loki/write"               // Import loki.write
	_ "github.com/grafana/alloy/internal/component/prometheus/exporter/self" // Import prometheus.exporter.self
	_ "github.com/grafana/alloy/internal/component/prometheus/exporter/unix" // Import prometheus.exporter.unix
	_ "github.com/grafana/alloy/internal/component/prometheus/relabel"       // Import prometheus.relabel
	_ "github.com/grafana/alloy/internal/component/prometheus/remotewrite"   // Import prometheus.remote_write
	_ "github.com/grafana/alloy/internal/component/prometheus/scrape"        // Import prometheus.scrape
)

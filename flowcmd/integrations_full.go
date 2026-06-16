//go:build !slim

package flowcmd

import (
	// Register grafana-agent static-mode integrations for the full build.
	// Excluded from slim builds to drop their heavy dependency trees.
	_ "github.com/grafana/alloy/internal/static/integrations/install"

	// Register all Prometheus service-discovery mechanisms (kubernetes, ec2,
	// azure, gce, consul, ...). Excluded from slim builds: it pulls k8s
	// client-go and cloud SDKs, and slim only scrapes static targets.
	_ "github.com/prometheus/prometheus/discovery/install"
)

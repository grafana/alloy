//go:build !alloy_custom_components

package flowcmd

import (
	// Register Prometheus SD components used by legacy configuration.
	_ "github.com/prometheus/prometheus/discovery/install"

	_ "github.com/grafana/alloy/internal/loki/promtail/discovery/consulagent"

	// Register legacy integrations.
	_ "github.com/grafana/alloy/internal/static/integrations/install"
)

// Package logs holds types for the logging subsystem of Grafana Agent static
// mode.
package logs

import (
	_ "time/tzdata" // embed timezone data

	"github.com/grafana/alloy/internal/loki/promtail/config"
	"github.com/grafana/alloy/internal/loki/promtail/server"
	"github.com/grafana/alloy/internal/loki/promtail/tracing"
	"github.com/grafana/alloy/internal/loki/promtail/wal"
	"github.com/grafana/loki/v3/clients/pkg/promtail/client"

	"github.com/grafana/alloy/internal/useragent"
	_ "github.com/grafana/alloy/internal/util/otelfeaturegatefix" // Gracefully handle duplicate OTEL feature gates
)

func init() {
	client.UserAgent = useragent.Get()
}

// DefaultConfig returns a default config for a Logs instance.
func DefaultConfig() config.Config {
	return config.Config{
		ServerConfig: server.Config{Disable: true},
		Tracing:      tracing.Config{Enabled: false},
		WAL:          wal.Config{Enabled: false},
	}
}

// Package logs holds types for the logging subsystem of Grafana Agent static
// mode.
package logs

import (
	_ "time/tzdata" // embed timezone data

	"github.com/grafana/alloy/internal/loki/promtail/config"
	"github.com/grafana/alloy/internal/loki/promtail/server"
	"github.com/grafana/alloy/internal/loki/promtail/tracing"
	"github.com/grafana/alloy/internal/loki/promtail/wal"
)

// DefaultConfig returns a default config for a Logs instance.
func DefaultConfig() config.Config {
	return config.Config{
		ServerConfig: server.Config{Disable: true},
		Tracing:      tracing.Config{Enabled: false},
		WAL:          wal.Config{Enabled: false},
	}
}

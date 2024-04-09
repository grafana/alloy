// Package logs holds types for the logging subsystem of Grafana Agent static
// mode.
package logs

import (
	_ "time/tzdata" // embed timezone data

	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/loki/clients/pkg/promtail/client"
	"github.com/grafana/loki/clients/pkg/promtail/config"
	"github.com/grafana/loki/clients/pkg/promtail/server"
	"github.com/grafana/loki/clients/pkg/promtail/wal"
	"github.com/grafana/loki/pkg/tracing"
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

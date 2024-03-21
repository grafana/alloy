package receiver

import (
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/river"
	"github.com/grafana/river/rivertypes"
)

// Arguments configures the app_agent_receiver component.
type Arguments struct {
	LogLabels map[string]string `river:"extra_log_labels,attr,optional"`

	Server     ServerArguments     `river:"server,block,optional"`
	SourceMaps SourceMapsArguments `river:"sourcemaps,block,optional"`
	Output     OutputArguments     `river:"output,block"`
}

var _ river.Defaulter = (*Arguments)(nil)

// SetToDefault applies default settings.
func (args *Arguments) SetToDefault() {
	args.Server.SetToDefault()
	args.SourceMaps.SetToDefault()
}

// ServerArguments configures the HTTP server where telemetry information will
// be sent from Faro clients.
type ServerArguments struct {
	Host                  string            `river:"listen_address,attr,optional"`
	Port                  int               `river:"listen_port,attr,optional"`
	CORSAllowedOrigins    []string          `river:"cors_allowed_origins,attr,optional"`
	APIKey                rivertypes.Secret `river:"api_key,attr,optional"`
	MaxAllowedPayloadSize units.Base2Bytes  `river:"max_allowed_payload_size,attr,optional"`

	RateLimiting RateLimitingArguments `river:"rate_limiting,block,optional"`
}

func (s *ServerArguments) SetToDefault() {
	*s = ServerArguments{
		Host:                  "127.0.0.1",
		Port:                  12347,
		MaxAllowedPayloadSize: 5 * units.MiB,
	}
	s.RateLimiting.SetToDefault()
}

// RateLimitingArguments configures rate limiting for the HTTP server.
type RateLimitingArguments struct {
	Enabled   bool    `river:"enabled,attr,optional"`
	Rate      float64 `river:"rate,attr,optional"`
	BurstSize float64 `river:"burst_size,attr,optional"`
}

func (r *RateLimitingArguments) SetToDefault() {
	*r = RateLimitingArguments{
		Enabled:   true,
		Rate:      50,
		BurstSize: 100,
	}
}

// SourceMapsArguments configures how app_agent_receiver will retrieve source
// maps for transforming stack traces.
type SourceMapsArguments struct {
	Download            bool                `river:"download,attr,optional"`
	DownloadFromOrigins []string            `river:"download_from_origins,attr,optional"`
	DownloadTimeout     time.Duration       `river:"download_timeout,attr,optional"`
	Locations           []LocationArguments `river:"location,block,optional"`
}

func (s *SourceMapsArguments) SetToDefault() {
	*s = SourceMapsArguments{
		Download:            true,
		DownloadFromOrigins: []string{"*"},
		DownloadTimeout:     time.Second,
	}
}

// LocationArguments specifies an individual location where source maps will be loaded.
type LocationArguments struct {
	Path               string `river:"path,attr"`
	MinifiedPathPrefix string `river:"minified_path_prefix,attr"`
}

// OutputArguments configures where to send emitted logs and traces. Metrics
// emitted by app_agent_receiver are exported as targets to be scraped.
type OutputArguments struct {
	Logs   []loki.LogsReceiver `river:"logs,attr,optional"`
	Traces []otelcol.Consumer  `river:"traces,attr,optional"`
}

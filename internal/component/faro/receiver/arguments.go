package receiver

import (
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// Arguments configures the app_agent_receiver component.
type Arguments struct {
	LogLabels map[string]string `alloy:"extra_log_labels,attr,optional"`

	Server     ServerArguments     `alloy:"server,block,optional"`
	SourceMaps SourceMapsArguments `alloy:"sourcemaps,block,optional"`
	Output     OutputArguments     `alloy:"output,block"`
}

var _ syntax.Defaulter = (*Arguments)(nil)

// SetToDefault applies default settings.
func (args *Arguments) SetToDefault() {
	args.Server.SetToDefault()
	args.SourceMaps.SetToDefault()
}

// ServerArguments configures the HTTP server where telemetry information will
// be sent from Faro clients.
type ServerArguments struct {
	Host                  string            `alloy:"listen_address,attr,optional"`
	Port                  int               `alloy:"listen_port,attr,optional"`
	CORSAllowedOrigins    []string          `alloy:"cors_allowed_origins,attr,optional"`
	APIKey                alloytypes.Secret `alloy:"api_key,attr,optional"`
	MaxAllowedPayloadSize units.Base2Bytes  `alloy:"max_allowed_payload_size,attr,optional"`

	RateLimiting    RateLimitingArguments `alloy:"rate_limiting,block,optional"`
	IncludeMetadata bool                  `alloy:"include_metadata,attr,optional"`
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
	Enabled   bool    `alloy:"enabled,attr,optional"`
	Rate      float64 `alloy:"rate,attr,optional"`
	BurstSize float64 `alloy:"burst_size,attr,optional"`
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
	Download            bool                `alloy:"download,attr,optional"`
	DownloadFromOrigins []string            `alloy:"download_from_origins,attr,optional"`
	DownloadTimeout     time.Duration       `alloy:"download_timeout,attr,optional"`
	Locations           []LocationArguments `alloy:"location,block,optional"`
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
	Path               string `alloy:"path,attr"`
	MinifiedPathPrefix string `alloy:"minified_path_prefix,attr"`
}

// OutputArguments configures where to send emitted logs and traces. Metrics
// emitted by app_agent_receiver are exported as targets to be scraped.
type OutputArguments struct {
	Logs   []loki.LogsReceiver `alloy:"logs,attr,optional"`
	Traces []otelcol.Consumer  `alloy:"traces,attr,optional"`
}

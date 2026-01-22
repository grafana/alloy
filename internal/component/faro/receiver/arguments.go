package receiver

import (
	"encoding"
	"fmt"
	"math"
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
	LogFormat LogFormat         `alloy:"log_format,attr,optional"`

	Server     ServerArguments     `alloy:"server,block,optional"`
	SourceMaps SourceMapsArguments `alloy:"sourcemaps,block,optional"`
	Output     OutputArguments     `alloy:"output,block"`
}

var _ syntax.Defaulter = (*Arguments)(nil)

// SetToDefault applies default settings.
func (args *Arguments) SetToDefault() {
	args.LogFormat = FormatDefault
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
	Enabled   bool                 `alloy:"enabled,attr,optional"`
	Strategy  RateLimitingStrategy `alloy:"strategy,attr,optional"`
	Rate      float64              `alloy:"rate,attr,optional"`
	BurstSize float64              `alloy:"burst_size,attr,optional"`
}

func (r *RateLimitingArguments) SetToDefault() {
	*r = RateLimitingArguments{
		Enabled:   true,
		Strategy:  RateLimitingStrategyGlobal,
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
	Cache               *CacheArguments     `alloy:"cache,block,optional"`
	Locations           []LocationArguments `alloy:"location,block,optional"`
}

func (s *SourceMapsArguments) SetToDefault() {
	*s = SourceMapsArguments{
		Download:            true,
		DownloadFromOrigins: []string{"*"},
		DownloadTimeout:     time.Second,
		Cache:               &CacheArguments{},
	}
	s.Cache.SetToDefault()
}

// CacheArguments configures sourcemap caching behavior.
type CacheArguments struct {
	TTL                  time.Duration `alloy:"ttl,attr,optional"`
	ErrorCleanupInterval time.Duration `alloy:"error_cleanup_interval,attr,optional"`
	CleanupCheckInterval time.Duration `alloy:"cleanup_check_interval,attr,optional"`
}

func (c *CacheArguments) SetToDefault() {
	*c = CacheArguments{
		TTL:                  time.Duration(math.MaxInt64),
		ErrorCleanupInterval: time.Hour,
		CleanupCheckInterval: time.Second * 30,
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

type LogFormat string

const (
	FormatLogfmt LogFormat = "logfmt"
	FormatJSON   LogFormat = "json"

	FormatDefault = FormatLogfmt
)

var (
	_ encoding.TextMarshaler   = FormatDefault
	_ encoding.TextUnmarshaler = (*LogFormat)(nil)
)

func (ll LogFormat) MarshalText() (text []byte, err error) {
	return []byte(ll), nil
}

func (ll *LogFormat) UnmarshalText(text []byte) error {
	switch LogFormat(text) {
	case "":
		*ll = FormatDefault
	case FormatLogfmt, FormatJSON:
		*ll = LogFormat(text)
	default:
		return fmt.Errorf("unrecognized log format %q", string(text))
	}
	return nil
}

type RateLimitingStrategy string

const (
	RateLimitingStrategyGlobal RateLimitingStrategy = "global"
	RateLimitingStrategyPerApp RateLimitingStrategy = "per_app"

	RateLimitingStrategyDefault = RateLimitingStrategyGlobal
)

var (
	_ encoding.TextMarshaler   = RateLimitingStrategyDefault
	_ encoding.TextUnmarshaler = (*RateLimitingStrategy)(nil)
)

func (ll RateLimitingStrategy) MarshalText() (text []byte, err error) {
	return []byte(ll), nil
}

func (ll *RateLimitingStrategy) UnmarshalText(text []byte) error {
	switch RateLimitingStrategy(text) {
	case "":
		*ll = RateLimitingStrategyDefault
	case RateLimitingStrategyGlobal, RateLimitingStrategyPerApp:
		*ll = RateLimitingStrategy(text)
	default:
		return fmt.Errorf("unrecognized rate limiting strategy %q", string(text))
	}
	return nil
}

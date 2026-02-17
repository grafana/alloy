package app_agent_receiver

import (
	"time"

	"github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
)

const (
	// DefaultRateLimitingRPS is the default value of Requests Per Second
	// for ratelimiting
	DefaultRateLimitingRPS = 100
	// DefaultRateLimitingBurstiness is the default burstiness factor of the
	// token bucket algorithm
	DefaultRateLimitingBurstiness = 50
	// DefaultMaxPayloadSize is the max payload size in bytes
	DefaultMaxPayloadSize = 5e6
)

// DefaultConfig holds the default configuration of the receiver
var DefaultConfig = Config{
	// Default JS agent port

	Server: ServerConfig{
		Host: "127.0.0.1",
		Port: 12347,
		RateLimiting: RateLimitingConfig{
			Enabled:    true,
			RPS:        DefaultRateLimitingRPS,
			Burstiness: DefaultRateLimitingBurstiness,
		},
		MaxAllowedPayloadSize: DefaultMaxPayloadSize,
	},
	LogsLabels:      map[string]string{},
	LogsSendTimeout: time.Second * 2,
	SourceMaps: SourceMapConfig{
		DownloadFromOrigins: []string{"*"},
		DownloadTimeout:     time.Second,
	},
}

// ServerConfig holds the receiver http server configuration
type ServerConfig struct {
	Host                  string             `yaml:"host,omitempty"`
	Port                  int                `yaml:"port,omitempty"`
	CORSAllowedOrigins    []string           `yaml:"cors_allowed_origins,omitempty"`
	RateLimiting          RateLimitingConfig `yaml:"rate_limiting,omitempty"`
	APIKey                string             `yaml:"api_key,omitempty"`
	MaxAllowedPayloadSize int64              `yaml:"max_allowed_payload_size,omitempty"`
}

// RateLimitingConfig holds the configuration of the rate limiter
type RateLimitingConfig struct {
	Enabled    bool    `yaml:"enabled,omitempty"`
	RPS        float64 `yaml:"rps,omitempty"`
	Burstiness int     `yaml:"burstiness,omitempty"`
}

// SourceMapFileLocation holds sourcemap location on file system
type SourceMapFileLocation struct {
	Path               string `yaml:"path"`
	MinifiedPathPrefix string `yaml:"minified_path_prefix,omitempty"`
}

// SourceMapConfig configure source map locations
type SourceMapConfig struct {
	Download            bool                    `yaml:"download"`
	DownloadFromOrigins []string                `yaml:"download_origins,omitempty"`
	DownloadTimeout     time.Duration           `yaml:"download_timeout,omitempty"`
	FileSystem          []SourceMapFileLocation `yaml:"filesystem,omitempty"`
}

// Config is the configuration struct of the
// integration
type Config struct {
	Common          common.MetricsConfig `yaml:",inline"`
	Server          ServerConfig         `yaml:"server,omitempty"`
	TracesInstance  string               `yaml:"traces_instance,omitempty"`
	LogsInstance    string               `yaml:"logs_instance,omitempty"`
	LogsLabels      map[string]string    `yaml:"logs_labels,omitempty"`
	LogsSendTimeout time.Duration        `yaml:"logs_send_timeout,omitempty"`
	SourceMaps      SourceMapConfig      `yaml:"sourcemaps,omitempty"`
}

// UnmarshalYAML implements the Unmarshaler interface
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig
	c.LogsLabels = make(map[string]string)
	type plain Config
	return unmarshal((*plain)(c))
}

// IntegrationName is the name of this integration
var IntegrationName = "app_agent_receiver"

// ApplyDefaults applies runtime-specific defaults to c.
func (c *Config) ApplyDefaults(globals integrations.Globals) error {
	c.Common.ApplyDefaults(globals.SubsystemOpts.Metrics.Autoscrape)
	if id, err := c.Identifier(globals); err == nil {
		c.Common.InstanceKey = &id
	}
	return nil
}

// Name returns the name of the integration that this config represents
func (c *Config) Name() string { return IntegrationName }

// Identifier uniquely identifies the app agent receiver integration
func (c *Config) Identifier(globals integrations.Globals) (string, error) {
	if c.Common.InstanceKey != nil {
		return *c.Common.InstanceKey, nil
	}
	return globals.AgentIdentifier, nil
}

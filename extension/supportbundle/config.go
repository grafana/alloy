package supportbundle

import (
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// Path is the HTTP path that serves the support bundle.
	Path string `mapstructure:"path"`

	// DefaultCollectionDuration is the default window for windowed collectors,
	// such as the CPU profile, when a request does not set ?duration=.
	DefaultCollectionDuration time.Duration `mapstructure:"default_collection_duration"`

	// MaxCollectionDuration is the upper bound for a requested collection window.
	MaxCollectionDuration time.Duration `mapstructure:"max_collection_duration"`

	// EnvironmentVariables lists extra environment variable names to capture,
	// beyond the built-in allowlist. The operator sets this. A request cannot
	// add names, so a caller cannot read arbitrary environment variables.
	EnvironmentVariables []string `mapstructure:"environment_variables"`

	// LogBufferSize is the size, in bytes, of the ring buffer that retains the
	// most recent collector logs for the bundle. A zero value disables log
	// capture. Capture is always on when enabled, so it adds a small per-line
	// cost to logging; keep it disabled unless you need recent logs in bundles.
	LogBufferSize int `mapstructure:"log_buffer_size"`
}

var (
	errPathPrefix       = errors.New(`path must start with "/"`)
	errPathPattern      = errors.New(`path must not contain whitespace, "{", or "}"`)
	errDurationPositive = errors.New("default_collection_duration must be positive")
	errMaxPositive      = errors.New("max_collection_duration must be positive")
	errDurationOverMax  = errors.New("default_collection_duration must not exceed max_collection_duration")
	errNegativeBuffer   = errors.New("log_buffer_size must not be negative")
	errWriteTimeout     = errors.New("write_timeout must be 0 (no limit) or greater than max_collection_duration, or the bundle download will be cut off")
)

func (cfg *Config) Validate() error {
	if !strings.HasPrefix(cfg.Path, "/") {
		return errPathPrefix
	}
	// The path is registered on an http.ServeMux, which treats "{...}" as a
	// wildcard and panics on malformed patterns. Keep it a literal path.
	if strings.ContainsAny(cfg.Path, "{} \t\n") {
		return errPathPattern
	}
	if cfg.DefaultCollectionDuration <= 0 {
		return errDurationPositive
	}
	if cfg.MaxCollectionDuration <= 0 {
		return errMaxPositive
	}
	if cfg.DefaultCollectionDuration > cfg.MaxCollectionDuration {
		return errDurationOverMax
	}
	if cfg.LogBufferSize < 0 {
		return errNegativeBuffer
	}
	// The handler writes the zip only after the collection window. A positive
	// write timeout shorter than the window truncates the download.
	if cfg.WriteTimeout != 0 && cfg.WriteTimeout <= cfg.MaxCollectionDuration {
		return errWriteTimeout
	}
	return nil
}

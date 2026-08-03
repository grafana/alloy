package logging

import (
	"encoding"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/syntax"
)

// Options is a set of options used to construct and configure a Logger.
type Options struct {
	Level       Level          `alloy:"level,attr,optional"`
	Format      Format         `alloy:"format,attr,optional"`
	Destination LogDestination `alloy:"destination,attr,optional"`

	WriteTo      []loki.LogsReceiver  `alloy:"write_to,attr,optional"`
	RateLimiting *RateLimitingOptions `alloy:"rate_limiting,block,optional"`
}

// LogDestination is where to send the primary log output.
type LogDestination string

// TODO: Add a "none" destination to disable primary output.
const (
	LogDestinationStderr          LogDestination = "stderr"
	LogDestinationWindowsEventLog LogDestination = "windows_event_log"
)

var _ encoding.TextUnmarshaler = (*LogDestination)(nil)

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *LogDestination) UnmarshalText(text []byte) error {
	switch LogDestination(text) {
	case LogDestinationStderr, LogDestinationWindowsEventLog:
		*d = LogDestination(text)
		return nil
	default:
		return fmt.Errorf("unrecognized log destination %q", string(text))
	}
}

// defaultDestination returns the platform-appropriate default log destination.
func defaultDestination() LogDestination {
	if isWindowsService() {
		return LogDestinationWindowsEventLog
	}
	return LogDestinationStderr
}

// defaultRateLimitingOptions returns the default rate-limiting configuration.
func defaultRateLimitingOptions() RateLimitingOptions {
	return RateLimitingOptions{Enabled: true, Tick: 10 * time.Second, Threshold: 10, Rate: 0, MaxSignatures: 1000}
}

// defaultOptions builds a fresh set of Logger defaults, evaluating the
// platform-appropriate destination at call time.
func defaultOptions() Options {
	rl := defaultRateLimitingOptions()
	return Options{
		Level:        LevelDefault,
		Format:       FormatDefault,
		Destination:  defaultDestination(),
		RateLimiting: &rl,
	}
}

// DefaultOptions holds defaults for creating a Logger.
var DefaultOptions = defaultOptions()

var _ syntax.Defaulter = (*Options)(nil)

// SetToDefault implements syntax.Defaulter. It re-evaluates the defaults at
// call time (rather than reusing DefaultOptions) so that tests which stub
// isWindowsService after package init still observe the expected destination.
func (o *Options) SetToDefault() {
	*o = defaultOptions()
}

// Level represents how verbose logging should be.
type Level string

// Supported log levels
const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"

	LevelDefault = LevelInfo
)

var (
	_ encoding.TextMarshaler   = LevelDefault
	_ encoding.TextUnmarshaler = (*Level)(nil)
)

// MarshalText implements encoding.TextMarshaler.
func (ll Level) MarshalText() (text []byte, err error) {
	return []byte(ll), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ll *Level) UnmarshalText(text []byte) error {
	switch Level(text) {
	case "":
		*ll = LevelDefault
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
		*ll = Level(text)
	default:
		return fmt.Errorf("unrecognized log level %q", string(text))
	}
	return nil
}

type slogLevel Level

func (l slogLevel) Level() slog.Level {
	switch Level(l) {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		// Allow all logs.
		return slog.Level(math.MinInt)
	}
}

// Format represents a text format to use when writing logs.
type Format string

// Supported log formats.
const (
	FormatLogfmt Format = "logfmt"
	FormatJSON   Format = "json"

	FormatDefault = FormatLogfmt
)

var (
	_ encoding.TextMarshaler   = FormatDefault
	_ encoding.TextUnmarshaler = (*Format)(nil)
)

// MarshalText implements encoding.TextMarshaler.
func (ll Format) MarshalText() (text []byte, err error) {
	return []byte(ll), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ll *Format) UnmarshalText(text []byte) error {
	switch Format(text) {
	case "":
		*ll = FormatDefault
	case FormatLogfmt, FormatJSON:
		*ll = Format(text)
	default:
		return fmt.Errorf("unrecognized log format %q", string(text))
	}
	return nil
}

// RateLimitingOptions configures log rate limiting per component and
// message. It is backed by github.com/samber/slog-sampling and is enabled
// by default.
type RateLimitingOptions struct {
	Enabled       bool          `alloy:"enabled,attr,optional"`
	Tick          time.Duration `alloy:"tick,attr,optional"`
	Threshold     uint64        `alloy:"threshold,attr,optional"`
	Rate          float64       `alloy:"rate,attr,optional"`
	MaxSignatures int           `alloy:"max_signatures,attr,optional"`
}

var _ syntax.Defaulter = (*RateLimitingOptions)(nil)

// SetToDefault implements syntax.Defaulter.
func (o *RateLimitingOptions) SetToDefault() {
	*o = defaultRateLimitingOptions()
}

var _ syntax.Validator = (*RateLimitingOptions)(nil)

// Validate implements syntax.Validator.
func (o RateLimitingOptions) Validate() error {
	if !o.Enabled {
		return nil
	}
	switch {
	case o.Tick <= 0:
		return fmt.Errorf("logging rate_limiting.tick must be > 0, got %v", o.Tick)
	case o.Threshold == 0:
		return fmt.Errorf("logging rate_limiting.threshold must be > 0")
	case o.Rate < 0 || o.Rate > 1:
		return fmt.Errorf("logging rate_limiting.rate must be in [0,1], got %v", o.Rate)
	case o.MaxSignatures <= 0:
		return fmt.Errorf("logging rate_limiting.max_signatures must be > 0, got %d", o.MaxSignatures)
	}
	return nil
}

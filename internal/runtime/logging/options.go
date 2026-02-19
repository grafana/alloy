package logging

import (
	"encoding"
	"fmt"
	"log/slog"
	"math"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/syntax"
)

// Options is a set of options used to construct and configure a Logger.
type Options struct {
	Level           Level  `alloy:"level,attr,optional"`
	Format          Format `alloy:"format,attr,optional"`
	WindowsEventLog bool   `alloy:"windows_event_log,attr,optional"`

	WriteTo []loki.LogsReceiver `alloy:"write_to,attr,optional"`
}

// DefaultOptions holds defaults for creating a Logger.
var DefaultOptions = Options{
	Level:  LevelDefault,
	Format: FormatDefault,
}

var _ syntax.Defaulter = (*Options)(nil)

// SetToDefault implements syntax.Defaulter.
func (o *Options) SetToDefault() {
	*o = DefaultOptions
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

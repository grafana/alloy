package config

import (
	"fmt"
	"time"

	promconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

type SyslogFormat string

const (
	// SyslogFormatRFC5424 is a modern Syslog RFC format.
	SyslogFormatRFC5424 = "rfc5424"
	// SyslogFormatRFC3164 is a legacy Syslog RFC format, also known as BSD-syslog.
	SyslogFormatRFC3164 = "rfc3164"

	// SyslogFormatRaw is a raw format.
	//
	// Using this format, skips log label parsing.
	SyslogFormatRaw = "raw"
)

// MarshalText implements encoding.TextMarshaler
func (s SyslogFormat) MarshalText() (text []byte, err error) {
	return []byte(s), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (s *SyslogFormat) UnmarshalText(text []byte) error {
	str := SyslogFormat(text)
	switch str {
	case "rfc5424":
		*s = SyslogFormatRFC5424
	case "rfc3164":
		*s = SyslogFormatRFC3164
	case "raw":
		*s = SyslogFormatRaw
	default:
		return fmt.Errorf("unknown syslog format: %s", str)
	}

	return nil
}

func (s SyslogFormat) Validate() error {
	switch s {
	case SyslogFormatRFC5424,
		SyslogFormatRFC3164,
		SyslogFormatRaw:
		return nil
	}

	return fmt.Errorf("unknown syslog format: %q", s)
}

// RawFormatOptions are options for raw syslog format processing.
type RawFormatOptions struct {
	// UseNullTerminatorDelimiter sets null terminator ('\0') as a log line delimiter for non-transparent framed messages.
	//
	// When set to false, new line character ('\n') is used instead.
	UseNullTerminatorDelimiter bool `yaml:"use_null_terminator_delimiter"`
}

func (opts RawFormatOptions) Delimiter() byte {
	if opts.UseNullTerminatorDelimiter {
		return 0
	}

	return '\n'
}

// SyslogTargetConfig describes a scrape config that listens for log lines over syslog.
type SyslogTargetConfig struct {
	// ListenAddress is the address to listen on for syslog messages.
	ListenAddress string `yaml:"listen_address"`

	// ListenProtocol is the protocol used to listen for syslog messages.
	// Must be either `tcp` (default) or `udp`
	ListenProtocol string `yaml:"listen_protocol"`

	// IdleTimeout is the idle timeout for tcp connections.
	IdleTimeout time.Duration `yaml:"idle_timeout"`

	// LabelStructuredData sets if the structured data part of a syslog message
	// is translated to a label.
	// [example@99999 test="yes"] => {__syslog_message_sd_example_99999_test="yes"}
	LabelStructuredData bool `yaml:"label_structured_data"`

	// Labels optionally holds labels to associate with each record read from syslog.
	Labels model.LabelSet `yaml:"labels"`

	// UseIncomingTimestamp sets the timestamp to the incoming syslog messages
	// timestamp if it's set.
	UseIncomingTimestamp bool `yaml:"use_incoming_timestamp"`

	// UseRFC5424Message defines whether the full RFC5424 formatted syslog
	// message should be pushed to Loki
	UseRFC5424Message bool `yaml:"use_rfc5424_message"`

	// Syslog format used at the target. Acceptable value is rfc5424 or rfc3164.
	// Default is rfc5424.
	SyslogFormat SyslogFormat `yaml:"syslog_format"`

	// RawFormatOptions are options for processing syslog messages in raw mode.
	//
	// Takes effect only if "syslog_format" is set to "raw".
	RawFormatOptions RawFormatOptions `yaml:"raw_format_options"`

	// MaxMessageLength sets the maximum limit to the length of syslog messages
	MaxMessageLength int `yaml:"max_message_length"`

	TLSConfig promconfig.TLSConfig `yaml:"tls_config,omitempty"`

	// When parsing an RFC3164 message, should the year be defaulted to the current year?
	// When false, the year will default to 0.
	RFC3164DefaultToCurrentYear bool `yaml:"rfc3164_default_to_current_year"`
}

func (config SyslogTargetConfig) IsRFC3164Message() bool {
	return config.SyslogFormat == SyslogFormatRFC3164
}

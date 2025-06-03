package config

import (
	"time"

	promconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
)

type SyslogFormat string

const (
	// A modern Syslog RFC
	SyslogFormatRFC5424 = "rfc5424"
	// A legacy Syslog RFC also known as BSD-syslog
	SyslogFormatRFC3164 = "rfc3164"
)

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

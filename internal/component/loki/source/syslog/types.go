package syslog

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/config"
	scrapeconfig "github.com/grafana/alloy/internal/component/loki/source/syslog/config"
	st "github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget"
)

// ListenerConfig defines a syslog listener.
type ListenerConfig struct {
	ListenAddress               string                    `alloy:"address,attr"`
	ListenProtocol              string                    `alloy:"protocol,attr,optional"`
	IdleTimeout                 time.Duration             `alloy:"idle_timeout,attr,optional"`
	LabelStructuredData         bool                      `alloy:"label_structured_data,attr,optional"`
	Labels                      map[string]string         `alloy:"labels,attr,optional"`
	UseIncomingTimestamp        bool                      `alloy:"use_incoming_timestamp,attr,optional"`
	UseRFC5424Message           bool                      `alloy:"use_rfc5424_message,attr,optional"`
	RFC3164DefaultToCurrentYear bool                      `alloy:"rfc3164_default_to_current_year,attr,optional"`
	MaxMessageLength            int                       `alloy:"max_message_length,attr,optional"`
	TLSConfig                   config.TLSConfig          `alloy:"tls_config,block,optional"`
	SyslogFormat                scrapeconfig.SyslogFormat `alloy:"syslog_format,attr,optional"`
	RawFormatOptions            *RawFormatOptions         `alloy:"raw_format_options,block,optional"`
}

// RawFormatOptions is alloy syntax mapping to [scrapeconfig.RawFormatOptions] struct.
type RawFormatOptions struct {
	UseNullTerminatorDelimiter bool `alloy:"use_null_terminator_delimiter,attr,optional"`
}

// DefaultListenerConfig provides the default arguments for a syslog listener.
var DefaultListenerConfig = ListenerConfig{
	ListenProtocol:   st.DefaultProtocol,
	IdleTimeout:      st.DefaultIdleTimeout,
	MaxMessageLength: st.DefaultMaxMessageLength,
	SyslogFormat:     scrapeconfig.SyslogFormatRFC5424,
}

// SetToDefault implements syntax.Defaulter.
func (sc *ListenerConfig) SetToDefault() {
	*sc = DefaultListenerConfig
}

// Validate implements syntax.Validator.
func (sc *ListenerConfig) Validate() error {
	if sc.ListenProtocol != st.ProtocolTCP && sc.ListenProtocol != st.ProtocolUDP {
		return fmt.Errorf("syslog listener protocol should be either 'tcp' or 'udp', got %s", sc.ListenProtocol)
	}

	if err := sc.SyslogFormat.Validate(); err != nil {
		return err
	}

	if sc.SyslogFormat == scrapeconfig.SyslogFormatRaw {
		// mention fields that have no effect for better UX
		if sc.UseRFC5424Message {
			return fmt.Errorf(`"use_rfc5424_message" has no effect when syslog format is set to %q`, sc.SyslogFormat)
		}

		if sc.RFC3164DefaultToCurrentYear {
			return fmt.Errorf(`"rfc3164_default_to_current_year" has no effect when syslog format is set to %q`, sc.SyslogFormat)
		}

		if sc.UseIncomingTimestamp {
			return fmt.Errorf(`"use_incoming_timestamp" has no effect when syslog format is set to %q`, sc.SyslogFormat)
		}

		return nil
	}

	if sc.RawFormatOptions != nil {
		return fmt.Errorf("raw_format_options has no effect when syslog format is not %q", scrapeconfig.SyslogFormatRaw)
	}

	return nil
}

// Convert is used to bridge between the Alloy and Promtail types.
func (sc ListenerConfig) Convert() (*scrapeconfig.SyslogTargetConfig, error) {
	lbls := make(model.LabelSet, len(sc.Labels))
	for k, v := range sc.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	cfg := &scrapeconfig.SyslogTargetConfig{
		ListenAddress:               sc.ListenAddress,
		ListenProtocol:              sc.ListenProtocol,
		IdleTimeout:                 sc.IdleTimeout,
		LabelStructuredData:         sc.LabelStructuredData,
		Labels:                      lbls,
		UseIncomingTimestamp:        sc.UseIncomingTimestamp,
		UseRFC5424Message:           sc.UseRFC5424Message,
		RFC3164DefaultToCurrentYear: sc.RFC3164DefaultToCurrentYear,
		MaxMessageLength:            sc.MaxMessageLength,
		TLSConfig:                   *sc.TLSConfig.Convert(),
		SyslogFormat:                sc.SyslogFormat,
	}

	if sc.RawFormatOptions != nil {
		cfg.RawFormatOptions = scrapeconfig.RawFormatOptions{
			UseNullTerminatorDelimiter: sc.RawFormatOptions.UseNullTerminatorDelimiter,
		}
	}

	return cfg, nil
}

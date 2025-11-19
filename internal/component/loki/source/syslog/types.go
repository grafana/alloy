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
	ListenAddress               string              `alloy:"address,attr"`
	ListenProtocol              string              `alloy:"protocol,attr,optional"`
	IdleTimeout                 time.Duration       `alloy:"idle_timeout,attr,optional"`
	LabelStructuredData         bool                `alloy:"label_structured_data,attr,optional"`
	Labels                      map[string]string   `alloy:"labels,attr,optional"`
	UseIncomingTimestamp        bool                `alloy:"use_incoming_timestamp,attr,optional"`
	UseRFC5424Message           bool                `alloy:"use_rfc5424_message,attr,optional"`
	RFC3164DefaultToCurrentYear bool                `alloy:"rfc3164_default_to_current_year,attr,optional"`
	MaxMessageLength            int                 `alloy:"max_message_length,attr,optional"`
	TLSConfig                   config.TLSConfig    `alloy:"tls_config,block,optional"`
	SyslogFormat                config.SysLogFormat `alloy:"syslog_format,attr,optional"`
}

// DefaultListenerConfig provides the default arguments for a syslog listener.
var DefaultListenerConfig = ListenerConfig{
	ListenProtocol:   st.DefaultProtocol,
	IdleTimeout:      st.DefaultIdleTimeout,
	MaxMessageLength: st.DefaultMaxMessageLength,
	SyslogFormat:     config.SyslogFormatRFC5424,
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

	_, err := convertSyslogFormat(sc.SyslogFormat)
	if err != nil {
		return err
	}

	return nil
}

// Convert is used to bridge between the Alloy and Promtail types.
func (sc ListenerConfig) Convert() (*scrapeconfig.SyslogTargetConfig, error) {
	lbls := make(model.LabelSet, len(sc.Labels))
	for k, v := range sc.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	syslogFormat, err := convertSyslogFormat(sc.SyslogFormat)
	if err != nil {
		return nil, err
	}

	return &scrapeconfig.SyslogTargetConfig{
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
		SyslogFormat:                syslogFormat,
	}, nil
}

func convertSyslogFormat(format config.SysLogFormat) (scrapeconfig.SyslogFormat, error) {
	switch format {
	case config.SyslogFormatRFC3164:
		return scrapeconfig.SyslogFormatRFC3164, nil
	case config.SyslogFormatRFC5424:
		return scrapeconfig.SyslogFormatRFC5424, nil
	default:
		return "", fmt.Errorf("unknown syslog format %q", format)
	}
}

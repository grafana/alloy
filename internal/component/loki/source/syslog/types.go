package syslog

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component/common/config"
	st "github.com/grafana/agent/internal/component/loki/source/syslog/internal/syslogtarget"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/prometheus/common/model"
)

// ListenerConfig defines a syslog listener.
type ListenerConfig struct {
	ListenAddress        string            `river:"address,attr"`
	ListenProtocol       string            `river:"protocol,attr,optional"`
	IdleTimeout          time.Duration     `river:"idle_timeout,attr,optional"`
	LabelStructuredData  bool              `river:"label_structured_data,attr,optional"`
	Labels               map[string]string `river:"labels,attr,optional"`
	UseIncomingTimestamp bool              `river:"use_incoming_timestamp,attr,optional"`
	UseRFC5424Message    bool              `river:"use_rfc5424_message,attr,optional"`
	MaxMessageLength     int               `river:"max_message_length,attr,optional"`
	TLSConfig            config.TLSConfig  `river:"tls_config,block,optional"`
}

// DefaultListenerConfig provides the default arguments for a syslog listener.
var DefaultListenerConfig = ListenerConfig{
	ListenProtocol:   st.DefaultProtocol,
	IdleTimeout:      st.DefaultIdleTimeout,
	MaxMessageLength: st.DefaultMaxMessageLength,
}

// SetToDefault implements river.Defaulter.
func (sc *ListenerConfig) SetToDefault() {
	*sc = DefaultListenerConfig
}

// Validate implements river.Validator.
func (sc *ListenerConfig) Validate() error {
	if sc.ListenProtocol != "tcp" && sc.ListenProtocol != "udp" {
		return fmt.Errorf("syslog listener protocol should be either 'tcp' or 'udp', got %s", sc.ListenProtocol)
	}

	return nil
}

// Convert is used to bridge between the River and Promtail types.
func (sc ListenerConfig) Convert() *scrapeconfig.SyslogTargetConfig {
	lbls := make(model.LabelSet, len(sc.Labels))
	for k, v := range sc.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	return &scrapeconfig.SyslogTargetConfig{
		ListenAddress:        sc.ListenAddress,
		ListenProtocol:       sc.ListenProtocol,
		IdleTimeout:          sc.IdleTimeout,
		LabelStructuredData:  sc.LabelStructuredData,
		Labels:               lbls,
		UseIncomingTimestamp: sc.UseIncomingTimestamp,
		UseRFC5424Message:    sc.UseRFC5424Message,
		MaxMessageLength:     sc.MaxMessageLength,
		TLSConfig:            *sc.TLSConfig.Convert(),
	}
}

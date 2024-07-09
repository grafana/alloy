package syslog

import (
	"fmt"
	"time"

	"github.com/grafana/loki/v3/clients/pkg/promtail/scrapeconfig"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/config"
	st "github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget"
)

// ListenerConfig defines a syslog listener.
type ListenerConfig struct {
	ListenAddress        string            `alloy:"address,attr"`
	ListenProtocol       string            `alloy:"protocol,attr,optional"`
	IdleTimeout          time.Duration     `alloy:"idle_timeout,attr,optional"`
	LabelStructuredData  bool              `alloy:"label_structured_data,attr,optional"`
	Labels               map[string]string `alloy:"labels,attr,optional"`
	UseIncomingTimestamp bool              `alloy:"use_incoming_timestamp,attr,optional"`
	UseRFC5424Message    bool              `alloy:"use_rfc5424_message,attr,optional"`
	MaxMessageLength     int               `alloy:"max_message_length,attr,optional"`
	TLSConfig            config.TLSConfig  `alloy:"tls_config,block,optional"`
}

// DefaultListenerConfig provides the default arguments for a syslog listener.
var DefaultListenerConfig = ListenerConfig{
	ListenProtocol:   st.DefaultProtocol,
	IdleTimeout:      st.DefaultIdleTimeout,
	MaxMessageLength: st.DefaultMaxMessageLength,
}

// SetToDefault implements syntax.Defaulter.
func (sc *ListenerConfig) SetToDefault() {
	*sc = DefaultListenerConfig
}

// Validate implements syntax.Validator.
func (sc *ListenerConfig) Validate() error {
	if sc.ListenProtocol != "tcp" && sc.ListenProtocol != "udp" {
		return fmt.Errorf("syslog listener protocol should be either 'tcp' or 'udp', got %s", sc.ListenProtocol)
	}

	return nil
}

// Convert is used to bridge between the Alloy and Promtail types.
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

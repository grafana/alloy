package build

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/syslog"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/loki/v3/clients/pkg/promtail/scrapeconfig"
)

func (s *ScrapeConfigBuilder) AppendSyslogConfig() {
	if s.cfg.SyslogConfig == nil {
		return
	}

	syslogFormat, err := convertSyslogFormat(s.cfg.SyslogConfig.SyslogFormat)
	if err != nil {
		s.diags.Add(diag.SeverityLevelCritical, err.Error())
		return
	}

	listenerConfig := syslog.ListenerConfig{
		ListenAddress:        s.cfg.SyslogConfig.ListenAddress,
		ListenProtocol:       s.cfg.SyslogConfig.ListenProtocol,
		IdleTimeout:          s.cfg.SyslogConfig.IdleTimeout,
		LabelStructuredData:  s.cfg.SyslogConfig.LabelStructuredData,
		Labels:               convertPromLabels(s.cfg.SyslogConfig.Labels),
		UseIncomingTimestamp: s.cfg.SyslogConfig.UseIncomingTimestamp,
		UseRFC5424Message:    s.cfg.SyslogConfig.UseRFC5424Message,
		MaxMessageLength:     s.cfg.SyslogConfig.MaxMessageLength,
		TLSConfig:            *common.ToTLSConfig(&s.cfg.SyslogConfig.TLSConfig),
		SyslogFormat:         syslogFormat,
	}

	// If the syslog format is not set, use the default.
	if listenerConfig.SyslogFormat == "" {
		listenerConfig.SyslogFormat = syslog.DefaultListenerConfig.SyslogFormat
	}

	args := syslog.Arguments{
		SyslogListeners: []syslog.ListenerConfig{
			listenerConfig,
		},
		ForwardTo:    s.getOrNewProcessStageReceivers(),
		RelabelRules: make(relabel.Rules, 0),
	}

	override := func(val interface{}) interface{} {
		switch val.(type) {
		case relabel.Rules:
			return common.CustomTokenizer{Expr: s.getOrNewDiscoveryRelabelRules()}
		default:
			return val
		}
	}
	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"loki", "source", "syslog"},
		compLabel,
		args,
		override,
	))
}

func convertSyslogFormat(format scrapeconfig.SyslogFormat) (config.SysLogFormat, error) {
	switch format {
	case "":
		return syslog.DefaultListenerConfig.SyslogFormat, nil
	case scrapeconfig.SyslogFormatRFC3164:
		return config.SyslogFormatRFC3164, nil
	case scrapeconfig.SyslogFormatRFC5424:
		return config.SyslogFormatRFC5424, nil
	default:
		return "", fmt.Errorf("unknown syslog format %q", format)
	}
}

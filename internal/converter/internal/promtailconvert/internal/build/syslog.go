package build

import (
	"github.com/grafana/agent/internal/component/common/relabel"
	"github.com/grafana/agent/internal/component/loki/source/syslog"
	"github.com/grafana/agent/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendSyslogConfig() {
	if s.cfg.SyslogConfig == nil {
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

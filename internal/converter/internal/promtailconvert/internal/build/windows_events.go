package build

import (
	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/agent/internal/component/loki/source/windowsevent"
	"github.com/grafana/agent/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendWindowsEventsConfig() {
	if s.cfg.WindowsConfig == nil {
		return
	}
	winCfg := s.cfg.WindowsConfig
	args := windowsevent.Arguments{
		Locale:               int(winCfg.Locale),
		EventLogName:         winCfg.EventlogName,
		XPathQuery:           winCfg.Query,
		BookmarkPath:         winCfg.BookmarkPath,
		PollInterval:         winCfg.PollInterval,
		ExcludeEventData:     winCfg.ExcludeEventData,
		ExcludeUserdata:      winCfg.ExcludeUserData,
		ExcludeEventMessage:  winCfg.ExcludeEventMessage,
		UseIncomingTimestamp: winCfg.UseIncomingTimestamp,
		ForwardTo:            make([]loki.LogsReceiver, 0),
		Labels:               convertPromLabels(winCfg.Labels),
	}

	override := func(val interface{}) interface{} {
		switch val.(type) {
		case []loki.LogsReceiver:
			return common.CustomTokenizer{Expr: s.getOrNewLokiRelabel()}
		default:
			return val
		}
	}
	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"loki", "source", "windowsevent"},
		compLabel,
		args,
		override,
	))
}

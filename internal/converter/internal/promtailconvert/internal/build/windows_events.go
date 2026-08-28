package build

import (
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/windowsevent"
	"github.com/grafana/alloy/internal/converter/internal/common"
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
		LegacyBookmarkPath:   winCfg.BookmarkPath,
		PollInterval:         winCfg.PollInterval,
		ExcludeEventData:     winCfg.ExcludeEventData,
		ExcludeUserdata:      winCfg.ExcludeUserData,
		ExcludeEventMessage:  winCfg.ExcludeEventMessage,
		UseIncomingTimestamp: winCfg.UseIncomingTimestamp,
		ForwardTo:            make([]loki.LogsReceiver, 0),
		Labels:               convertPromLabels(winCfg.Labels),
	}

	override := func(val any) any {
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

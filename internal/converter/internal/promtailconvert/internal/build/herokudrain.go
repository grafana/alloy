package build

import (
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/heroku"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendHerokuDrainConfig() {
	if s.cfg.HerokuDrainConfig == nil {
		return
	}
	hCfg := s.cfg.HerokuDrainConfig
	args := heroku.Arguments{
		Server:               common.WeaveworksServerToAlloyServer(hCfg.Server),
		Labels:               convertPromLabels(hCfg.Labels),
		UseIncomingTimestamp: hCfg.UseIncomingTimestamp,
		ForwardTo:            s.getOrNewProcessStageReceivers(),
		RelabelRules:         relabel.Rules{},
	}
	override := func(val any) any {
		switch val.(type) {
		case relabel.Rules:
			return common.CustomTokenizer{Expr: s.getOrNewDiscoveryRelabelRules()}
		default:
			return val
		}
	}
	compLabel := common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName)
	s.f.Body().AppendBlock(common.NewBlockWithOverrideFn(
		[]string{"loki", "source", "heroku"},
		compLabel,
		args,
		override,
	))
}

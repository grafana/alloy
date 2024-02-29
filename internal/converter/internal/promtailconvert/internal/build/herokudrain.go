package build

import (
	"github.com/grafana/agent/internal/component/common/relabel"
	"github.com/grafana/agent/internal/component/loki/source/heroku"
	"github.com/grafana/agent/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendHerokuDrainConfig() {
	if s.cfg.HerokuDrainConfig == nil {
		return
	}
	hCfg := s.cfg.HerokuDrainConfig
	args := heroku.Arguments{
		Server:               common.WeaveWorksServerToFlowServer(hCfg.Server),
		Labels:               convertPromLabels(hCfg.Labels),
		UseIncomingTimestamp: hCfg.UseIncomingTimestamp,
		ForwardTo:            s.getOrNewProcessStageReceivers(),
		RelabelRules:         relabel.Rules{},
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
		[]string{"loki", "source", "heroku"},
		compLabel,
		args,
		override,
	))
}

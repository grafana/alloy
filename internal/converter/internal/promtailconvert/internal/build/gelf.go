package build

import (
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/gelf"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendGelfConfig() {
	if s.cfg.GelfConfig == nil {
		return
	}
	gCfg := s.cfg.GelfConfig
	args := gelf.Arguments{
		ListenAddress:        gCfg.ListenAddress,
		UseIncomingTimestamp: gCfg.UseIncomingTimestamp,
		RelabelRules:         relabel.Rules{},
		Receivers:            s.getOrNewProcessStageReceivers(),
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
		[]string{"loki", "source", "gelf"},
		compLabel,
		args,
		override,
	))
}

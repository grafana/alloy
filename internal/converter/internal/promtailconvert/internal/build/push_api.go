package build

import (
	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/api"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendPushAPI() {
	if s.cfg.PushConfig == nil {
		return
	}
	s.diags.AddAll(common.ValidateWeaveWorksServerCfg(s.cfg.PushConfig.Server))
	args := toLokiApiArguments(s.cfg.PushConfig, s.getOrNewProcessStageReceivers())
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
		[]string{"loki", "source", "api"},
		compLabel,
		args,
		override,
	))
}

func toLokiApiArguments(config *scrapeconfig.PushTargetConfig, forwardTo []loki.LogsReceiver) api.Arguments {
	return api.Arguments{
		ForwardTo:            forwardTo,
		RelabelRules:         make(relabel.Rules, 0),
		Labels:               convertPromLabels(config.Labels),
		UseIncomingTimestamp: config.KeepTimestamp,
		Server:               common.WeaveworksServerToAlloyServer(config.Server),
		MaxSendMessageSize:   units.Base2Bytes(config.MaxSendMsgSize),
	}
}

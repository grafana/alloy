package build

import (
	"github.com/grafana/agent/internal/component/common/relabel"
	"github.com/grafana/agent/internal/component/loki/source/azure_event_hubs"
	"github.com/grafana/agent/internal/converter/internal/common"
)

func (s *ScrapeConfigBuilder) AppendAzureEventHubs() {
	if s.cfg.AzureEventHubsConfig == nil {
		return
	}
	aCfg := s.cfg.AzureEventHubsConfig
	args := azure_event_hubs.Arguments{
		FullyQualifiedNamespace: aCfg.FullyQualifiedNamespace,
		EventHubs:               aCfg.EventHubs,
		Authentication: azure_event_hubs.AzureEventHubsAuthentication{
			ConnectionString: aCfg.ConnectionString,
		},
		GroupID:                aCfg.GroupID,
		UseIncomingTimestamp:   aCfg.UseIncomingTimestamp,
		DisallowCustomMessages: aCfg.DisallowCustomMessages,
		RelabelRules:           relabel.Rules{},
		Labels:                 convertPromLabels(aCfg.Labels),
		ForwardTo:              s.getOrNewProcessStageReceivers(),
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
		[]string{"loki", "source", "azure_event_hubs"},
		compLabel,
		args,
		override,
	))
}

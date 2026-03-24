package build

import (
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/azure_event_hubs"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
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
			ConnectionString: alloytypes.Secret(aCfg.ConnectionString),
		},
		GroupID:                aCfg.GroupID,
		UseIncomingTimestamp:   aCfg.UseIncomingTimestamp,
		DisallowCustomMessages: aCfg.DisallowCustomMessages,
		RelabelRules:           relabel.Rules{},
		Labels:                 convertPromLabels(aCfg.Labels),
		ForwardTo:              s.getOrNewProcessStageReceivers(),
	}
	override := func(val any) any {
		switch value := val.(type) {
		case relabel.Rules:
			return common.CustomTokenizer{Expr: s.getOrNewDiscoveryRelabelRules()}
		case alloytypes.Secret:
			return string(value)
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

package build

import (
	"time"

	promtail_consulagent "github.com/grafana/alloy/internal/loki/promtail/discovery/consulagent"

	"github.com/grafana/alloy/internal/component/discovery/consulagent"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (s *ScrapeConfigBuilder) AppendConsulAgentSDs() {
	if len(s.cfg.ServiceDiscoveryConfig.ConsulAgentSDConfigs) == 0 {
		return
	}

	for i, sd := range s.cfg.ServiceDiscoveryConfig.ConsulAgentSDConfigs {
		args := toDiscoveryAgentConsul(sd, s.diags)
		compLabel := common.LabelWithIndex(i, s.globalCtx.LabelPrefix, s.cfg.JobName)
		s.f.Body().AppendBlock(common.NewBlockWithOverride(
			[]string{"discovery", "consulagent"},
			compLabel,
			args,
		))
		s.allTargetsExps = append(s.allTargetsExps, "discovery.consulagent."+compLabel+".targets")
	}
}

func toDiscoveryAgentConsul(sdConfig *promtail_consulagent.SDConfig, diags *diag.Diagnostics) *consulagent.Arguments {
	if sdConfig == nil {
		return nil
	}

	// Also unused promtail.
	if len(sdConfig.NodeMeta) != 0 {
		diags.Add(
			diag.SeverityLevelWarn,
			"node_meta is not used by discovery.consulagent and will be ignored",
		)
	}

	return &consulagent.Arguments{
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
		Server:          sdConfig.Server,
		Token:           alloytypes.Secret(sdConfig.Token),
		Datacenter:      sdConfig.Datacenter,
		TagSeparator:    sdConfig.TagSeparator,
		Scheme:          sdConfig.Scheme,
		Username:        sdConfig.Username,
		Password:        alloytypes.Secret(sdConfig.Password),
		Services:        sdConfig.Services,
		ServiceTags:     sdConfig.ServiceTags,
		TLSConfig:       *common.ToTLSConfig(&sdConfig.TLSConfig),
	}
}

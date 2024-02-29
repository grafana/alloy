package build

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery/consulagent"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	promtail_consulagent "github.com/grafana/loki/clients/pkg/promtail/discovery/consulagent"
	"github.com/grafana/river/rivertypes"
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
		Token:           rivertypes.Secret(sdConfig.Token),
		Datacenter:      sdConfig.Datacenter,
		TagSeparator:    sdConfig.TagSeparator,
		Scheme:          sdConfig.Scheme,
		Username:        sdConfig.Username,
		Password:        rivertypes.Secret(sdConfig.Password),
		Services:        sdConfig.Services,
		ServiceTags:     sdConfig.ServiceTags,
		TLSConfig:       *common.ToTLSConfig(&sdConfig.TLSConfig),
	}
}

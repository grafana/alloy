package build

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"

	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	"github.com/prometheus/common/model"
	prom_discover "github.com/prometheus/prometheus/discovery"
)

func (s *ScrapeConfigBuilder) AppendSDs() {
	sdConfigs := toDiscoveryConfig(s.cfg)
	if len(sdConfigs) == 0 {
		return
	}

	pb := build.NewPrometheusBlocks()
	targets := prometheusconvert.AppendServiceDiscoveryConfigs(pb, sdConfigs, common.LabelForParts(s.globalCtx.LabelPrefix, s.cfg.JobName))
	pb.AppendToBody(s.f.Body())

	targetLiterals := make([]discovery.Target, 0)
	for _, target := range targets {
		if expr, ok := target.Get("__expr__"); ok {
			// use the expression if __expr__ is set
			s.allTargetsExps = append(s.allTargetsExps, expr)
		} else {
			// otherwise, use the target as a literal
			targetLiterals = append(targetLiterals, target)
		}
	}

	// write the target literals as a string if there are any
	if len(targetLiterals) != 0 {
		literalsStr, err := toAlloyExpression(targetLiterals)
		if err != nil { // should not happen, unless we have a bug
			s.diags.Add(
				diag.SeverityLevelCritical,
				"failed to write static SD targets as valid Alloy expression: "+err.Error(),
			)
		}
		s.allTargetsExps = append(s.allTargetsExps, literalsStr)
	}

	s.diags.AddAll(prometheusconvert.ValidateServiceDiscoveryConfigs(sdConfigs))
}

func toDiscoveryConfig(cfg *scrapeconfig.Config) prom_discover.Configs {
	sdConfigs := make(prom_discover.Configs, 0)

	for _, sd := range cfg.ServiceDiscoveryConfig.AzureSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.ConsulSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.DigitalOceanSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.DockerSwarmSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.DNSSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.EC2SDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.FileSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.GCESDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.KubernetesSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.MarathonSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.NerveSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.OpenstackSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.ServersetSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	if len(cfg.ServiceDiscoveryConfig.StaticConfigs) != 0 {
		// Add source and default 'localhost' target to match Promtail's behaviour.
		// See https://github.com/grafana/loki/blob/4a564456861c0ab7441b815aa49a7c0f22a05f4c/clients/pkg/promtail/targets/file/filetargetmanager.go#L102
		for i, targetGroup := range cfg.ServiceDiscoveryConfig.StaticConfigs {
			targetGroup.Source = fmt.Sprintf("%d", i)
			if len(targetGroup.Targets) == 0 {
				targetGroup.Targets = []model.LabelSet{
					{model.AddressLabel: "localhost"},
				}
			}
		}
		sdConfigs = append(sdConfigs, cfg.ServiceDiscoveryConfig.StaticConfigs)
	}

	for _, sd := range cfg.ServiceDiscoveryConfig.TritonSDConfigs {
		sdConfigs = append(sdConfigs, sd)
	}

	return sdConfigs
}

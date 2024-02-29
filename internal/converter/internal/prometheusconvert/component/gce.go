package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/gce"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_gce "github.com/prometheus/prometheus/discovery/gce"
)

func appendDiscoveryGCE(pb *build.PrometheusBlocks, label string, sdConfig *prom_gce.SDConfig) discovery.Exports {
	discoveryGCEArgs := toDiscoveryGCE(sdConfig)
	name := []string{"discovery", "gce"}
	block := common.NewBlockWithOverride(name, label, discoveryGCEArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.gce." + label + ".targets")
}

func ValidateDiscoveryGCE(sdConfig *prom_gce.SDConfig) diag.Diagnostics {
	return make(diag.Diagnostics, 0)
}

func toDiscoveryGCE(sdConfig *prom_gce.SDConfig) *gce.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &gce.Arguments{
		Project:         sdConfig.Project,
		Zone:            sdConfig.Zone,
		Filter:          sdConfig.Filter,
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
		Port:            sdConfig.Port,
		TagSeparator:    sdConfig.TagSeparator,
	}
}

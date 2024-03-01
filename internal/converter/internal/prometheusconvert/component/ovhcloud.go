package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/ovhcloud"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/river/rivertypes"
	prom_discovery "github.com/prometheus/prometheus/discovery/ovhcloud"
)

func appendDiscoveryOvhcloud(pb *build.PrometheusBlocks, label string, sdConfig *prom_discovery.SDConfig) discovery.Exports {
	discoveryOvhcloudArgs := toDiscoveryOvhcloud(sdConfig)
	name := []string{"discovery", "ovhcloud"}
	block := common.NewBlockWithOverride(name, label, discoveryOvhcloudArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.ovhcloud." + label + ".targets")
}

func ValidateDiscoveryOvhcloud(sdConfig *prom_discovery.SDConfig) diag.Diagnostics {
	return nil
}

func toDiscoveryOvhcloud(sdConfig *prom_discovery.SDConfig) *ovhcloud.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &ovhcloud.Arguments{
		Endpoint:          sdConfig.Endpoint,
		ApplicationKey:    sdConfig.ApplicationKey,
		ApplicationSecret: rivertypes.Secret(sdConfig.ApplicationSecret),
		ConsumerKey:       rivertypes.Secret(sdConfig.ConsumerKey),
		RefreshInterval:   time.Duration(sdConfig.RefreshInterval),
		Service:           sdConfig.Service,
	}
}

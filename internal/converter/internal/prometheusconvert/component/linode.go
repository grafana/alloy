package component

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/discovery/linode"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	prom_linode "github.com/prometheus/prometheus/discovery/linode"
)

func appendDiscoveryLinode(pb *build.PrometheusBlocks, label string, sdConfig *prom_linode.SDConfig) discovery.Exports {
	discoveryLinodeArgs := toDiscoveryLinode(sdConfig)
	name := []string{"discovery", "linode"}
	block := common.NewBlockWithOverride(name, label, discoveryLinodeArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.linode." + label + ".targets")
}

func ValidateDiscoveryLinode(sdConfig *prom_linode.SDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryLinode(sdConfig *prom_linode.SDConfig) *linode.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &linode.Arguments{
		RefreshInterval:  time.Duration(sdConfig.RefreshInterval),
		Port:             sdConfig.Port,
		TagSeparator:     sdConfig.TagSeparator,
		Region:           sdConfig.Region,
		HTTPClientConfig: *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
	}
}

package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/kuma"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_kuma "github.com/prometheus/prometheus/discovery/xds"
)

func appendDiscoveryKuma(pb *build.PrometheusBlocks, label string, sdConfig *prom_kuma.SDConfig) discovery.Exports {
	discoveryKumaArgs := toDiscoveryKuma(sdConfig)
	name := []string{"discovery", "kuma"}
	block := common.NewBlockWithOverride(name, label, discoveryKumaArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.kuma." + label + ".targets")
}

func ValidateDiscoveryKuma(sdConfig *prom_kuma.SDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryKuma(sdConfig *prom_kuma.SDConfig) *kuma.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &kuma.Arguments{
		Server:          sdConfig.Server,
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
		FetchTimeout:    time.Duration(sdConfig.FetchTimeout),

		HTTPClientConfig: *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
	}
}

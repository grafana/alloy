package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/triton"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_triton "github.com/prometheus/prometheus/discovery/triton"
)

func appendDiscoveryTriton(pb *build.PrometheusBlocks, label string, sdConfig *prom_triton.SDConfig) discovery.Exports {
	discoveryTritonArgs := toDiscoveryTriton(sdConfig)
	name := []string{"discovery", "triton"}
	block := common.NewBlockWithOverride(name, label, discoveryTritonArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.triton." + label + ".targets")
}

func ValidateDiscoveryTriton(sdConfig *prom_triton.SDConfig) diag.Diagnostics {
	return nil
}

func toDiscoveryTriton(sdConfig *prom_triton.SDConfig) *triton.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &triton.Arguments{
		Account:         sdConfig.Account,
		Role:            sdConfig.Role,
		DNSSuffix:       sdConfig.DNSSuffix,
		Endpoint:        sdConfig.Endpoint,
		Groups:          sdConfig.Groups,
		Port:            sdConfig.Port,
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
		Version:         sdConfig.Version,
		TLSConfig:       *common.ToTLSConfig(&sdConfig.TLSConfig),
	}
}

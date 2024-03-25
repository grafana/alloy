package component

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/discovery/marathon"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/alloy/syntax/alloytypes"
	prom_marathon "github.com/prometheus/prometheus/discovery/marathon"
)

func appendDiscoveryMarathon(pb *build.PrometheusBlocks, label string, sdConfig *prom_marathon.SDConfig) discovery.Exports {
	discoveryMarathonArgs := toDiscoveryMarathon(sdConfig)
	name := []string{"discovery", "marathon"}
	block := common.NewBlockWithOverride(name, label, discoveryMarathonArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.marathon." + label + ".targets")
}

func ValidateDiscoveryMarathon(sdConfig *prom_marathon.SDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryMarathon(sdConfig *prom_marathon.SDConfig) *marathon.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &marathon.Arguments{
		Servers:          sdConfig.Servers,
		AuthToken:        alloytypes.Secret(sdConfig.AuthToken),
		AuthTokenFile:    sdConfig.AuthTokenFile,
		RefreshInterval:  time.Duration(sdConfig.RefreshInterval),
		HTTPClientConfig: *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
	}
}

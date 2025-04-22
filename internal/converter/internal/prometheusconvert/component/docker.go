package component

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/discovery/docker"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	prom_moby "github.com/prometheus/prometheus/discovery/moby"
)

func appendDiscoveryDocker(pb *build.PrometheusBlocks, label string, sdConfig *prom_moby.DockerSDConfig) discovery.Exports {
	discoveryDockerArgs := toDiscoveryDocker(sdConfig)
	name := []string{"discovery", "docker"}
	block := common.NewBlockWithOverride(name, label, discoveryDockerArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.docker." + label + ".targets")
}

func ValidateDiscoveryDocker(sdConfig *prom_moby.DockerSDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryDocker(sdConfig *prom_moby.DockerSDConfig) *docker.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &docker.Arguments{
		Host:               sdConfig.Host,
		Port:               sdConfig.Port,
		HostNetworkingHost: sdConfig.HostNetworkingHost,
		RefreshInterval:    time.Duration(sdConfig.RefreshInterval),
		Filters:            toDockerFilters(sdConfig.Filters),
		HTTPClientConfig:   *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
		MatchFirstNetwork:  sdConfig.MatchFirstNetwork,
	}
}

func toDockerFilters(filtersConfig []prom_moby.Filter) []docker.Filter {
	filters := make([]docker.Filter, 0)

	for _, filter := range filtersConfig {
		filters = append(filters, docker.Filter{
			Name:   filter.Name,
			Values: filter.Values,
		})
	}

	return filters
}

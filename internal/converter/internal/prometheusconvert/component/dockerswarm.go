package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/dockerswarm"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_moby "github.com/prometheus/prometheus/discovery/moby"
)

func appendDiscoveryDockerswarm(pb *build.PrometheusBlocks, label string, sdConfig *prom_moby.DockerSwarmSDConfig) discovery.Exports {
	discoveryDockerswarmArgs := toDiscoveryDockerswarm(sdConfig)
	name := []string{"discovery", "dockerswarm"}
	block := common.NewBlockWithOverride(name, label, discoveryDockerswarmArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.dockerswarm." + label + ".targets")
}

func ValidateDiscoveryDockerswarm(sdConfig *prom_moby.DockerSwarmSDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryDockerswarm(sdConfig *prom_moby.DockerSwarmSDConfig) *dockerswarm.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &dockerswarm.Arguments{
		Host:             sdConfig.Host,
		Role:             sdConfig.Role,
		Port:             sdConfig.Port,
		Filters:          convertFilters(sdConfig.Filters),
		RefreshInterval:  time.Duration(sdConfig.RefreshInterval),
		HTTPClientConfig: *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
	}
}

func convertFilters(mobyFilters []prom_moby.Filter) []dockerswarm.Filter {
	riverFilters := make([]dockerswarm.Filter, len(mobyFilters))
	for i, mobyFilter := range mobyFilters {
		riverFilters[i] = convertFilter(&mobyFilter)
	}
	return riverFilters
}

func convertFilter(mobyFilter *prom_moby.Filter) dockerswarm.Filter {
	values := make([]string, len(mobyFilter.Values))
	copy(values, mobyFilter.Values)

	return dockerswarm.Filter{
		Name:   mobyFilter.Name,
		Values: values,
	}
}

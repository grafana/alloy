package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/aws"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/river/rivertypes"
	prom_aws "github.com/prometheus/prometheus/discovery/aws"
)

func appendDiscoveryEC2(pb *build.PrometheusBlocks, label string, sdConfig *prom_aws.EC2SDConfig) discovery.Exports {
	discoveryec2Args := toDiscoveryEC2(sdConfig)
	name := []string{"discovery", "ec2"}
	block := common.NewBlockWithOverride(name, label, discoveryec2Args)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.ec2." + label + ".targets")
}

func ValidateDiscoveryEC2(sdConfig *prom_aws.EC2SDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryEC2(sdConfig *prom_aws.EC2SDConfig) *aws.EC2Arguments {
	if sdConfig == nil {
		return nil
	}

	return &aws.EC2Arguments{
		Endpoint:         sdConfig.Endpoint,
		Region:           sdConfig.Region,
		AccessKey:        sdConfig.AccessKey,
		SecretKey:        rivertypes.Secret(sdConfig.SecretKey),
		Profile:          sdConfig.Profile,
		RoleARN:          sdConfig.RoleARN,
		RefreshInterval:  time.Duration(sdConfig.RefreshInterval),
		Port:             sdConfig.Port,
		Filters:          toEC2Filters(sdConfig.Filters),
		HTTPClientConfig: *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
	}
}

func toEC2Filters(filtersConfig []*prom_aws.EC2Filter) []*aws.EC2Filter {
	filters := make([]*aws.EC2Filter, 0)

	for _, filter := range filtersConfig {
		filters = append(filters, &aws.EC2Filter{
			Name:   filter.Name,
			Values: filter.Values,
		})
	}

	return filters
}

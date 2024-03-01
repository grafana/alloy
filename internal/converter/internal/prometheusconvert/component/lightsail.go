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

func appendDiscoveryLightsail(pb *build.PrometheusBlocks, label string, sdConfig *prom_aws.LightsailSDConfig) discovery.Exports {
	discoverylightsailArgs := toDiscoveryLightsail(sdConfig)
	name := []string{"discovery", "lightsail"}
	block := common.NewBlockWithOverride(name, label, discoverylightsailArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.lightsail." + label + ".targets")
}

func ValidateDiscoveryLightsail(sdConfig *prom_aws.LightsailSDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryLightsail(sdConfig *prom_aws.LightsailSDConfig) *aws.LightsailArguments {
	if sdConfig == nil {
		return nil
	}

	return &aws.LightsailArguments{
		Endpoint:         sdConfig.Endpoint,
		Region:           sdConfig.Region,
		AccessKey:        sdConfig.AccessKey,
		SecretKey:        rivertypes.Secret(sdConfig.SecretKey),
		Profile:          sdConfig.Profile,
		RoleARN:          sdConfig.RoleARN,
		RefreshInterval:  time.Duration(sdConfig.RefreshInterval),
		Port:             sdConfig.Port,
		HTTPClientConfig: *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
	}
}

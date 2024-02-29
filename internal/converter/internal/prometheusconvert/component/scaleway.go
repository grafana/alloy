package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/scaleway"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/river/rivertypes"
	prom_scaleway "github.com/prometheus/prometheus/discovery/scaleway"
)

func appendDiscoveryScaleway(pb *build.PrometheusBlocks, label string, sdConfig *prom_scaleway.SDConfig) discovery.Exports {
	discoveryScalewayArgs := toDiscoveryScaleway(sdConfig)
	name := []string{"discovery", "scaleway"}
	block := common.NewBlockWithOverride(name, label, discoveryScalewayArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.scaleway." + label + ".targets")
}

func ValidateDiscoveryScaleway(sdConfig *prom_scaleway.SDConfig) diag.Diagnostics {
	return nil
}

func toDiscoveryScaleway(sdConfig *prom_scaleway.SDConfig) *scaleway.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &scaleway.Arguments{
		Project:         sdConfig.Project,
		Role:            scaleway.Role(sdConfig.Role),
		APIURL:          sdConfig.APIURL,
		Zone:            sdConfig.Zone,
		AccessKey:       sdConfig.AccessKey,
		SecretKey:       rivertypes.Secret(sdConfig.SecretKey),
		SecretKeyFile:   sdConfig.SecretKeyFile,
		NameFilter:      sdConfig.NameFilter,
		TagsFilter:      sdConfig.TagsFilter,
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
		Port:            sdConfig.Port,
		ProxyConfig:     common.ToProxyConfig(sdConfig.HTTPClientConfig.ProxyConfig),
		TLSConfig:       *common.ToTLSConfig(&sdConfig.HTTPClientConfig.TLSConfig),
		FollowRedirects: sdConfig.HTTPClientConfig.FollowRedirects,
		EnableHTTP2:     sdConfig.HTTPClientConfig.EnableHTTP2,
	}
}

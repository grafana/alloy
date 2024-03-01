package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/digitalocean"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/river/rivertypes"
	prom_config "github.com/prometheus/common/config"
	prom_digitalocean "github.com/prometheus/prometheus/discovery/digitalocean"
)

func appendDiscoveryDigitalOcean(pb *build.PrometheusBlocks, label string, sdConfig *prom_digitalocean.SDConfig) discovery.Exports {
	discoveryDigitalOceanArgs := toDiscoveryDigitalOcean(sdConfig)
	name := []string{"discovery", "digitalocean"}
	block := common.NewBlockWithOverride(name, label, discoveryDigitalOceanArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.digitalocean." + label + ".targets")
}

func ValidateDiscoveryDigitalOcean(sdConfig *prom_digitalocean.SDConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	var nilBasicAuth *prom_config.BasicAuth
	var nilAuthorization *prom_config.Authorization
	var nilOAuth2 *prom_config.OAuth2

	diags.AddAll(common.ValidateSupported(common.NotEquals, sdConfig.HTTPClientConfig.BasicAuth, nilBasicAuth, "digitalocean_sd_configs basic_auth", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, sdConfig.HTTPClientConfig.Authorization, nilAuthorization, "digitalocean_sd_configs authorization", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, sdConfig.HTTPClientConfig.OAuth2, nilOAuth2, "digitalocean_sd_configs oauth2", ""))
	diags.AddAll(common.ValidateSupported(common.NotDeepEquals, sdConfig.HTTPClientConfig.TLSConfig, prom_config.TLSConfig{}, "digitalocean_sd_configs tls_config", ""))

	diags.AddAll(common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig))

	return diags
}

func toDiscoveryDigitalOcean(sdConfig *prom_digitalocean.SDConfig) *digitalocean.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &digitalocean.Arguments{
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
		Port:            sdConfig.Port,
		BearerToken:     rivertypes.Secret(sdConfig.HTTPClientConfig.BearerToken),
		BearerTokenFile: sdConfig.HTTPClientConfig.BearerTokenFile,
		ProxyConfig:     common.ToProxyConfig(sdConfig.HTTPClientConfig.ProxyConfig),
		FollowRedirects: sdConfig.HTTPClientConfig.FollowRedirects,
		EnableHTTP2:     sdConfig.HTTPClientConfig.EnableHTTP2,
	}
}

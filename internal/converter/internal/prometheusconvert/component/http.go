package component

import (
	"net/url"
	"time"

	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/http"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_http "github.com/prometheus/prometheus/discovery/http"
)

func appendDiscoveryHttp(pb *build.PrometheusBlocks, label string, sdConfig *prom_http.SDConfig) discovery.Exports {
	discoveryFileArgs := toDiscoveryHttp(sdConfig)
	name := []string{"discovery", "http"}
	block := common.NewBlockWithOverride(name, label, discoveryFileArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.http." + label + ".targets")
}

func ValidateDiscoveryHttp(sdConfig *prom_http.SDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryHttp(sdConfig *prom_http.SDConfig) *http.Arguments {
	if sdConfig == nil {
		return nil
	}

	url, err := url.Parse(sdConfig.URL)
	if err != nil {
		panic("invalid http_sd_configs url provided")
	}

	return &http.Arguments{
		HTTPClientConfig: *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
		RefreshInterval:  time.Duration(sdConfig.RefreshInterval),
		URL:              config.URL{URL: url},
	}
}

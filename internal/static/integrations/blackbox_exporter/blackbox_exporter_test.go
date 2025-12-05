package blackbox_exporter

import (
	"net/url"
	"testing"

	integrations "github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestBlackboxConfig(t *testing.T) {
	t.Run("scrape configs", func(t *testing.T) {
		var config Config
		strConfig := `---
blackbox_targets:
- name: icmp_cloudflare
  address: 1.1.1.1
  module: icmp_ipv4
- name: http_cloudflare
  address: https://www.cloudflare.com
  module: http_2xx_ipv4
blackbox_config:
  modules:
    http_2xx_ipv4:
      prober: http
      timeout: 5s
      http:
        preferred_ip_protocol: "ip4"
    icmp_ipv4:
      prober: "icmp"
      timeout: 5s
      icmp:
        preferred_ip_protocol: "ip4"
`
		require.NoError(t, yaml.UnmarshalStrict([]byte(strConfig), &config), "unmarshal config")

		integration, err := New(nil, &config)
		require.NoError(t, err)
		expectedScrapeConfigs := []integrations.ScrapeConfig{
			{
				JobName:     "blackbox/icmp_cloudflare",
				MetricsPath: "/metrics",
				QueryParams: url.Values{"target": []string{"1.1.1.1"}, "module": []string{"icmp_ipv4"}},
			},
			{
				JobName:     "blackbox/http_cloudflare",
				MetricsPath: "/metrics",
				QueryParams: url.Values{"target": []string{"https://www.cloudflare.com"}, "module": []string{"http_2xx_ipv4"}},
			},
		}
		require.Equal(t, integration.ScrapeConfigs(), expectedScrapeConfigs)
	})
}

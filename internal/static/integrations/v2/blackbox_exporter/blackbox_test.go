package blackbox_exporter_v2

import (
	"testing"

	"github.com/grafana/alloy/internal/static/integrations/blackbox_exporter"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	autoscrape "github.com/grafana/alloy/internal/static/integrations/v2/autoscrape"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/require"
)

func TestBlackbox(t *testing.T) {
	t.Run("targets", func(t *testing.T) {
		key := "blackbox-test"
		aEnabled := true

		c := Config{
			ProbeTimeoutOffset: 0.5,
			Common: common.MetricsConfig{
				InstanceKey: &key,
				Autoscrape: autoscrape.Config{
					Enable: &aEnabled,
				},
			},
			BlackboxTargets: []blackbox_exporter.BlackboxTarget{{
				Name:   "icmp_cloudflare",
				Target: "1.1.1.1",
				Module: "icmp_ipv4",
			}},
		}
		integration, err := c.NewIntegration(nil, integrations_v2.Globals{})
		require.NoError(t, err)

		i := integration.(integrations_v2.MetricsIntegration)
		actual := i.Targets(integrations_v2.Endpoint{Host: "test", Prefix: "/test/"})
		expect := []*targetgroup.Group{{
			Source: "blackbox/blackbox",
			Labels: model.LabelSet{
				"instance":       "blackbox-test",
				"job":            "integrations/blackbox",
				"agent_hostname": "",

				"__meta_agent_integration_name":       "blackbox",
				"__meta_agent_integration_instance":   "blackbox",
				"__meta_agent_integration_autoscrape": "1",
			},
			Targets: []model.LabelSet{{
				model.AddressLabel:     "test",
				model.MetricsPathLabel: "/test/metrics",
				"blackbox_target":      "1.1.1.1",
				"__param_target":       "1.1.1.1",
				"__param_module":       "icmp_ipv4",
			}},
		}}
		require.Equal(t, expect, actual)
	})
}

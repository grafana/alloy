package metricsutils

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestMetricsHandlerIntegration_Targets(t *testing.T) {
	globals := integrations.Globals{
		AgentIdentifier: "testagent",
		AgentBaseURL: func() *url.URL {
			u, err := url.Parse("http://testagent/")
			require.NoError(t, err)
			return u
		}(),
		SubsystemOpts: integrations.DefaultSubsystemOptions,
	}

	t.Run("Targets", func(t *testing.T) {
		var cfg common.MetricsConfig
		cfg.ApplyDefaults(globals.SubsystemOpts.Metrics.Autoscrape)

		i, err := NewMetricsHandlerIntegration(nil, fakeConfig{}, cfg, globals, http.NotFoundHandler())
		require.NoError(t, err)

		actual := i.Targets(integrations.Endpoint{Host: "test", Prefix: "/test/"})
		expect := []*targetgroup.Group{{
			Source: "fake/testagent",
			Labels: model.LabelSet{
				"instance":       "testagent",
				"job":            "integrations/fake",
				"agent_hostname": "testagent",

				"__meta_agent_integration_name":       "fake",
				"__meta_agent_integration_instance":   "testagent",
				"__meta_agent_integration_autoscrape": "1",
			},
			Targets: []model.LabelSet{{
				"__address__":      "test",
				"__metrics_path__": "/test/metrics",
			}},
		}}
		require.Equal(t, expect, actual)

		t.Run("Extra labels", func(t *testing.T) {
			cfg := common.MetricsConfig{
				ExtraLabels: labels.FromMap(map[string]string{"foo": "bar", "fizz": "buzz"}),
			}
			cfg.ApplyDefaults(globals.SubsystemOpts.Metrics.Autoscrape)

			i, err := NewMetricsHandlerIntegration(nil, fakeConfig{}, cfg, globals, http.NotFoundHandler())
			require.NoError(t, err)
			actual := i.Targets(integrations.Endpoint{Host: "test", Prefix: "/test/"})
			require.Len(t, actual, 1)

			cfg.ExtraLabels.Range(func(lbl labels.Label) {
				val, ok := actual[0].Labels[model.LabelName(lbl.Name)]
				require.True(t, ok, "target does not have extra label %s", lbl.Name)
				require.Equal(t, lbl.Value, string(val), "extra label %s does not match expectation", lbl.Name)
			})
		})
	})
}

type fakeConfig struct{}

func (fakeConfig) Name() string                                      { return "fake" }
func (fakeConfig) ApplyDefaults(_ integrations.Globals) error        { return nil }
func (fakeConfig) Identifier(g integrations.Globals) (string, error) { return g.AgentIdentifier, nil }
func (fakeConfig) NewIntegration(_ log.Logger, _ integrations.Globals) (integrations.Integration, error) {
	return nil, fmt.Errorf("not implemented")
}

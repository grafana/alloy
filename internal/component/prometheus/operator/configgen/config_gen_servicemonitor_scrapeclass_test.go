package configgen

import (
	"testing"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	alloy_config "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
)

func scrapeClassTestGenerator() *ConfigGenerator {
	return &ConfigGenerator{
		Client: &kubernetes.ClientArguments{},
		ScrapeClasses: []operator.ScrapeClass{
			{
				Name:          "secure",
				Default:       true,
				TLSConfig:     &alloy_config.TLSConfig{InsecureSkipVerify: true},
				Authorization: &alloy_config.Authorization{Type: "Bearer", Credentials: "class-token"},
				Relabelings: []*alloy_relabel.Config{
					{TargetLabel: "from_class", Replacement: "yes"},
				},
				MetricRelabelings: []*alloy_relabel.Config{
					{TargetLabel: "metric_from_class", Replacement: "yes"},
				},
				AttachMetadata: &operator.AttachMetadataConfig{Node: true},
			},
		},
	}
}

func indexOfTargetLabel(cfgs []*relabel.Config, label string) int {
	for i, c := range cfgs {
		if c.TargetLabel == label {
			return i
		}
	}
	return -1
}

func TestGenerateServiceMonitorConfigScrapeClassDefaultApplied(t *testing.T) {
	cg := scrapeClassTestGenerator()
	m := &promopv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sm"}}
	ep := promopv1.Endpoint{
		RelabelConfigs:       []promopv1.RelabelConfig{{TargetLabel: "from_endpoint", Replacement: ptr.To("yes")}},
		MetricRelabelConfigs: []promopv1.RelabelConfig{{TargetLabel: "metric_from_endpoint", Replacement: ptr.To("yes")}},
	}

	cfg, err := cg.GenerateServiceMonitorConfig(m, ep, 0, promk8s.RoleEndpoint)
	require.NoError(t, err)

	// TLS and authorization come from the default scrape class.
	assert.True(t, cfg.HTTPClientConfig.TLSConfig.InsecureSkipVerify)
	require.NotNil(t, cfg.HTTPClientConfig.Authorization)
	assert.Equal(t, "class-token", string(cfg.HTTPClientConfig.Authorization.Credentials))

	// attach_metadata from the class flows into the service discovery config.
	sd, ok := cfg.ServiceDiscoveryConfigs[0].(*promk8s.SDConfig)
	require.True(t, ok)
	assert.True(t, sd.AttachMetadata.Node)

	// Class relabelings are prepended (before the endpoint's own).
	classIdx := indexOfTargetLabel(cfg.RelabelConfigs, "from_class")
	epIdx := indexOfTargetLabel(cfg.RelabelConfigs, "from_endpoint")
	require.NotEqual(t, -1, classIdx)
	require.NotEqual(t, -1, epIdx)
	assert.Less(t, classIdx, epIdx, "class relabelings should be prepended before endpoint relabelings")

	// Class metric relabelings are appended (after the endpoint's own).
	classMetricIdx := indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_class")
	epMetricIdx := indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_endpoint")
	require.NotEqual(t, -1, classMetricIdx)
	require.NotEqual(t, -1, epMetricIdx)
	assert.Greater(t, classMetricIdx, epMetricIdx, "class metric relabelings should be appended after endpoint metric relabelings")
}

func TestGenerateServiceMonitorConfigScrapeClassEndpointOverridesTLS(t *testing.T) {
	cg := scrapeClassTestGenerator()
	m := &promopv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sm"}}
	// Endpoint sets its own (empty) TLS config, so the class TLS must not apply.
	ep := promopv1.Endpoint{TLSConfig: &promopv1.TLSConfig{}}

	cfg, err := cg.GenerateServiceMonitorConfig(m, ep, 0, promk8s.RoleEndpoint)
	require.NoError(t, err)

	assert.False(t, cfg.HTTPClientConfig.TLSConfig.InsecureSkipVerify, "endpoint TLS should take precedence over the class")
}

func TestGenerateServiceMonitorConfigScrapeClassNotDefined(t *testing.T) {
	cg := scrapeClassTestGenerator()
	m := &promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sm"},
		Spec:       promopv1.ServiceMonitorSpec{ScrapeClassName: ptr.To("does-not-exist")},
	}

	_, err := cg.GenerateServiceMonitorConfig(m, promopv1.Endpoint{}, 0, promk8s.RoleEndpoint)
	require.Error(t, err)
}

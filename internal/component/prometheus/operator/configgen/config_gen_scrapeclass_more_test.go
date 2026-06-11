package configgen

import (
	"testing"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestGeneratePodMonitorConfigScrapeClass(t *testing.T) {
	cg := scrapeClassTestGenerator()
	m := &promopv1.PodMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pm"}}
	ep := promopv1.PodMetricsEndpoint{
		RelabelConfigs:       []promopv1.RelabelConfig{{TargetLabel: "from_endpoint", Replacement: ptr.To("yes")}},
		MetricRelabelConfigs: []promopv1.RelabelConfig{{TargetLabel: "metric_from_endpoint", Replacement: ptr.To("yes")}},
	}

	cfg, err := cg.GeneratePodMonitorConfig(m, ep, 0)
	require.NoError(t, err)

	assert.True(t, cfg.HTTPClientConfig.TLSConfig.InsecureSkipVerify)
	require.NotNil(t, cfg.HTTPClientConfig.Authorization)
	assert.Equal(t, "class-token", string(cfg.HTTPClientConfig.Authorization.Credentials))

	sd, ok := cfg.ServiceDiscoveryConfigs[0].(*promk8s.SDConfig)
	require.True(t, ok)
	assert.True(t, sd.AttachMetadata.Node)

	assert.Less(t, indexOfTargetLabel(cfg.RelabelConfigs, "from_class"), indexOfTargetLabel(cfg.RelabelConfigs, "from_endpoint"))
	assert.Greater(t, indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_class"), indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_endpoint"))
}

func TestGenerateProbeConfigScrapeClass(t *testing.T) {
	cg := scrapeClassTestGenerator()
	m := &promopv1.Probe{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pr"},
		Spec: promopv1.ProbeSpec{
			ProberSpec: promopv1.ProberSpec{URL: "blackbox:9115"},
			Targets: promopv1.ProbeTargets{
				StaticConfig: &promopv1.ProbeTargetStaticConfig{
					Targets:        []string{"http://example.com"},
					RelabelConfigs: []promopv1.RelabelConfig{{TargetLabel: "from_endpoint", Replacement: ptr.To("yes")}},
				},
			},
			MetricRelabelConfigs: []promopv1.RelabelConfig{{TargetLabel: "metric_from_endpoint", Replacement: ptr.To("yes")}},
		},
	}

	cfg, err := cg.GenerateProbeConfig(m)
	require.NoError(t, err)

	assert.True(t, cfg.HTTPClientConfig.TLSConfig.InsecureSkipVerify)
	require.NotNil(t, cfg.HTTPClientConfig.Authorization)
	assert.Equal(t, "class-token", string(cfg.HTTPClientConfig.Authorization.Credentials))

	assert.Less(t, indexOfTargetLabel(cfg.RelabelConfigs, "from_class"), indexOfTargetLabel(cfg.RelabelConfigs, "from_endpoint"))
	assert.Greater(t, indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_class"), indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_endpoint"))
}

func TestGenerateScrapeConfigConfigScrapeClass(t *testing.T) {
	cg := scrapeClassTestGenerator()
	m := &promopv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sc"},
		Spec: promopv1alpha1.ScrapeConfigSpec{
			StaticConfigs:        []promopv1alpha1.StaticConfig{{Targets: []promopv1alpha1.Target{"1.2.3.4:9090"}}},
			RelabelConfigs:       []promopv1.RelabelConfig{{TargetLabel: "from_endpoint", Replacement: ptr.To("yes")}},
			MetricRelabelConfigs: []promopv1.RelabelConfig{{TargetLabel: "metric_from_endpoint", Replacement: ptr.To("yes")}},
		},
	}

	cfgs, errs := cg.GenerateScrapeConfigConfigs(m)
	require.Empty(t, errs)
	require.Len(t, cfgs, 1)
	cfg := cfgs[0]

	assert.True(t, cfg.HTTPClientConfig.TLSConfig.InsecureSkipVerify)
	require.NotNil(t, cfg.HTTPClientConfig.Authorization)
	assert.Equal(t, "class-token", string(cfg.HTTPClientConfig.Authorization.Credentials))

	assert.Less(t, indexOfTargetLabel(cfg.RelabelConfigs, "from_class"), indexOfTargetLabel(cfg.RelabelConfigs, "from_endpoint"))
	assert.Greater(t, indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_class"), indexOfTargetLabel(cfg.MetricRelabelConfigs, "metric_from_endpoint"))
}

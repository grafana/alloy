package configgen

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	commonConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/util"
)

// see https://github.com/prometheus-operator/prometheus-operator/blob/aa8222d7e9b66e9293ed11c9291ea70173021029/pkg/prometheus/promcfg_test.go#L423
func TestGenerateProbeConfig(t *testing.T) {
	suite := []struct {
		name                   string
		m                      *promopv1.Probe
		ep                     promopv1.Endpoint
		expectedRelabels       string
		expectedMetricRelabels string
		expected               *config.ScrapeConfig
	}{
		{
			name: "basic ingress",
			m: &promopv1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "myprobe",
				},
				Spec: promopv1.ProbeSpec{
					ProberSpec: promopv1.ProberSpec{
						URL: "foo.bar",
					},
					Targets: promopv1.ProbeTargets{
						Ingress: &promopv1.ProbeTargetIngress{
							Selector: metav1.LabelSelector{
								MatchLabels: map[string]string{"foo": "bar"},
							},
						},
					},
				},
			},
			expectedRelabels: util.Untab(`
- target_label: __meta_foo
  replacement: bar
- source_labels: [job]
  target_label: __tmp_prometheus_job_name
- source_labels: [__meta_kubernetes_ingress_label_foo, __meta_kubernetes_ingress_labelpresent_foo]
  regex: (bar);true
  action: keep
- source_labels: [__meta_kubernetes_ingress_scheme, __address__, __meta_kubernetes_ingress_path]
  regex: (.+);(.+);(.+)
  target_label: __param_target
  replacement: ${1}://${2}${3}
- source_labels: [__meta_kubernetes_namespace]
  target_label: namespace
- source_labels: [__meta_kubernetes_ingress_name]
  target_label: ingress
- source_labels: [__address__]
  regex: (.+)
  target_label: __tmp_ingress_address
- source_labels: [__param_target]
  target_label: instance
- target_label: __address__
  replacement: foo.bar
`),
			expected: &config.ScrapeConfig{
				JobName:           "probe/operator/myprobe",
				HonorTimestamps:   true,
				ScrapeInterval:    model.Duration(time.Minute),
				ScrapeTimeout:     model.Duration(10 * time.Second),
				ScrapeProtocols:   config.DefaultScrapeProtocols,
				EnableCompression: true,
				MetricsPath:       "",
				Scheme:            "http",
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					&promk8s.SDConfig{
						Role: "ingress",
						NamespaceDiscovery: promk8s.NamespaceDiscovery{
							IncludeOwnNamespace: false,
							Names:               []string{"operator"},
						},
					},
				},
			},
		},
		{
			name: "static targets",
			m: &promopv1.Probe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testprobe1",
					Namespace: "default",
					Labels: map[string]string{
						"group": "group1",
					},
				},
				Spec: promopv1.ProbeSpec{
					ProberSpec: promopv1.ProberSpec{
						Scheme:   "http",
						URL:      "blackbox.exporter.io",
						Path:     "/probe",
						ProxyURL: "socks://myproxy:9095",
					},
					Module: "http_2xx",
					Targets: promopv1.ProbeTargets{
						StaticConfig: &promopv1.ProbeTargetStaticConfig{
							Targets: []string{
								"prometheus.io",
								"promcon.io",
							},
							Labels: map[string]string{
								"static": "label",
							},
							RelabelConfigs: []*promopv1.RelabelConfig{
								{
									TargetLabel: "foo",
									Replacement: "bar",
									Action:      "replace",
								},
							},
						},
					},
				},
			},
			expectedRelabels: util.Untab(`
- target_label: __meta_foo
  replacement: bar
- source_labels:
  - job
  target_label: __tmp_prometheus_job_name
- source_labels:
  - __address__
  target_label: __param_target
- source_labels:
  - __param_target
  target_label: instance
- target_label: __address__
  replacement: blackbox.exporter.io
- target_label: foo
  replacement: bar
  action: replace
`),
			expected: &config.ScrapeConfig{
				JobName:           "probe/default/testprobe1",
				HonorTimestamps:   true,
				ScrapeInterval:    model.Duration(time.Minute),
				ScrapeTimeout:     model.Duration(10 * time.Second),
				ScrapeProtocols:   config.DefaultScrapeProtocols,
				EnableCompression: true,
				MetricsPath:       "/probe",
				Scheme:            "http",
				Params:            url.Values{"module": []string{"http_2xx"}},
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
					ProxyConfig: commonConfig.ProxyConfig{
						ProxyURL: commonConfig.URL{URL: &url.URL{Scheme: "socks", Host: "myproxy:9095"}},
					},
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					discovery.StaticConfig{
						{
							Targets: []model.LabelSet{
								{"__address__": "prometheus.io"},
								{"__address__": "promcon.io"},
							},
							Labels: model.LabelSet{
								"static": "label",
								"namespace": "default",
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range suite {
		t.Run(tc.name, func(t *testing.T) {
			cg := &ConfigGenerator{
				Client: &kubernetes.ClientArguments{},
				AdditionalRelabelConfigs: []*alloy_relabel.Config{
					{TargetLabel: "__meta_foo", Replacement: "bar"},
				},
			}
			cfg, err := cg.GenerateProbeConfig(tc.m)
			require.NoError(t, err)
			// check relabel configs separately
			rlcs := cfg.RelabelConfigs
			mrlcs := cfg.MetricRelabelConfigs
			cfg.RelabelConfigs = nil
			cfg.MetricRelabelConfigs = nil
			require.NoError(t, err)

			assert.Equal(t, tc.expected, cfg)

			checkRelabels := func(actual []*relabel.Config, expected string) {
				// load the expected relabel rules as yaml so we get the defaults put in there.
				ex := []*relabel.Config{}
				err := yaml.Unmarshal([]byte(expected), &ex)
				require.NoError(t, err)
				y, err := yaml.Marshal(ex)
				require.NoError(t, err)
				expected = string(y)

				y, err = yaml.Marshal(actual)
				require.NoError(t, err)

				if !assert.YAMLEq(t, expected, string(y)) {
					fmt.Fprintln(os.Stderr, string(y))
				}
			}
			checkRelabels(rlcs, tc.expectedRelabels)
			checkRelabels(mrlcs, tc.expectedMetricRelabels)
		})
	}
}

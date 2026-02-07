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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/util"
)

func TestGeneratePodMonitorConfig(t *testing.T) {
	var (
		falsePtr = ptr.To(false)
		proxyURL = "https://proxy:8080"
	)
	suite := []struct {
		name                   string
		m                      *promopv1.PodMonitor
		ep                     promopv1.PodMetricsEndpoint
		expectedRelabels       string
		expectedMetricRelabels string
		expected               *config.ScrapeConfig
	}{
		{
			name: "default",
			m: &promopv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "podmonitor",
				},
			},
			ep: promopv1.PodMetricsEndpoint{},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- source_labels: [__meta_kubernetes_pod_phase]
				  regex: (Failed|Succeeded)
				  action: drop
				- source_labels: [__meta_kubernetes_namespace]
				  target_label: namespace
				- source_labels: [__meta_kubernetes_pod_container_name]
				  target_label: container
				- source_labels: [__meta_kubernetes_pod_name]
				  target_label: pod
				- target_label: job
				  replacement: operator/podmonitor
				
			`),
			expected: &config.ScrapeConfig{
				JobName:                "podMonitor/operator/podmonitor/1",
				HonorTimestamps:        true,
				ScrapeInterval:         model.Duration(time.Hour),
				ScrapeTimeout:          model.Duration(42 * time.Second),
				ScrapeProtocols:        config.DefaultScrapeProtocols,
				ScrapeFallbackProtocol: config.PrometheusText0_0_4,
				EnableCompression:      true,
				MetricsPath:            "/metrics",
				Scheme:                 "http",
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					&promk8s.SDConfig{
						Role: "pod",

						NamespaceDiscovery: promk8s.NamespaceDiscovery{
							IncludeOwnNamespace: false,
							Names:               []string{"operator"},
						},
					},
				},
				ScrapeNativeHistograms:         falsePtr,
				SampleLimit:                    18,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
			},
		},
		{
			name: "targetport_string",
			m: &promopv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "podmonitor",
				},
			},
			ep: promopv1.PodMetricsEndpoint{
				TargetPort: &intstr.IntOrString{StrVal: "http_metrics", Type: intstr.String},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- source_labels: [__meta_kubernetes_pod_phase]
				  regex: (Failed|Succeeded)
				  action: drop
				- source_labels: ["__meta_kubernetes_pod_container_port_name"]
				  regex: "http_metrics"
				  action: "keep"
				- source_labels: [__meta_kubernetes_namespace]
				  target_label: namespace
				- source_labels: [__meta_kubernetes_pod_container_name]
				  target_label: container
				- source_labels: [__meta_kubernetes_pod_name]
				  target_label: pod
				- target_label: job
				  replacement: operator/podmonitor
				- target_label: endpoint
				  replacement: http_metrics
			`),
			expected: &config.ScrapeConfig{
				JobName:                "podMonitor/operator/podmonitor/1",
				HonorTimestamps:        true,
				ScrapeInterval:         model.Duration(time.Hour),
				ScrapeTimeout:          model.Duration(42 * time.Second),
				ScrapeProtocols:        config.DefaultScrapeProtocols,
				ScrapeFallbackProtocol: config.PrometheusText0_0_4,
				EnableCompression:      true,
				MetricsPath:            "/metrics",
				Scheme:                 "http",
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					&promk8s.SDConfig{
						Role: "pod",

						NamespaceDiscovery: promk8s.NamespaceDiscovery{
							IncludeOwnNamespace: false,
							Names:               []string{"operator"},
						},
					},
				},
				SampleLimit:                    18,
				ScrapeNativeHistograms:         falsePtr,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
			},
		},
		{
			name: "targetport_int",
			m: &promopv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "podmonitor",
				},
			},
			ep: promopv1.PodMetricsEndpoint{
				TargetPort: &intstr.IntOrString{IntVal: 8080, Type: intstr.Int},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- source_labels: [__meta_kubernetes_pod_phase]
				  regex: (Failed|Succeeded)
				  action: drop
				- source_labels: ["__meta_kubernetes_pod_container_port_number"]
				  regex: "8080"
				  action: "keep"
				- source_labels: [__meta_kubernetes_namespace]
				  target_label: namespace
				- source_labels: [__meta_kubernetes_pod_container_name]
				  target_label: container
				- source_labels: [__meta_kubernetes_pod_name]
				  target_label: pod
				- target_label: job
				  replacement: operator/podmonitor
				- target_label: endpoint
				  replacement: "8080"
			`),
			expected: &config.ScrapeConfig{
				JobName:                "podMonitor/operator/podmonitor/1",
				HonorTimestamps:        true,
				ScrapeInterval:         model.Duration(time.Hour),
				ScrapeTimeout:          model.Duration(42 * time.Second),
				ScrapeProtocols:        config.DefaultScrapeProtocols,
				ScrapeFallbackProtocol: config.PrometheusText0_0_4,
				EnableCompression:      true,
				MetricsPath:            "/metrics",
				Scheme:                 "http",
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					&promk8s.SDConfig{
						Role: "pod",

						NamespaceDiscovery: promk8s.NamespaceDiscovery{
							IncludeOwnNamespace: false,
							Names:               []string{"operator"},
						},
					},
				},
				SampleLimit:                    18,
				ScrapeNativeHistograms:         falsePtr,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
			},
		},
		{
			name: "portnumber",
			m: &promopv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "podmonitor",
				},
			},
			ep: promopv1.PodMetricsEndpoint{
				PortNumber: ptr.To[int32](2000),
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- source_labels: [__meta_kubernetes_pod_phase]
				  regex: (Failed|Succeeded)
				  action: drop
				- source_labels: ["__meta_kubernetes_pod_container_port_number"]
				  regex: "2000"
				  action: "keep"
				- source_labels: [__meta_kubernetes_namespace]
				  target_label: namespace
				- source_labels: [__meta_kubernetes_pod_container_name]
				  target_label: container
				- source_labels: [__meta_kubernetes_pod_name]
				  target_label: pod
				- target_label: job
				  replacement: operator/podmonitor
			`),
			expected: &config.ScrapeConfig{
				JobName:                "podMonitor/operator/podmonitor/1",
				HonorTimestamps:        true,
				ScrapeInterval:         model.Duration(time.Hour),
				ScrapeTimeout:          model.Duration(42 * time.Second),
				ScrapeProtocols:        config.DefaultScrapeProtocols,
				ScrapeFallbackProtocol: config.PrometheusText0_0_4,
				EnableCompression:      true,
				MetricsPath:            "/metrics",
				Scheme:                 "http",
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					&promk8s.SDConfig{
						Role: "pod",

						NamespaceDiscovery: promk8s.NamespaceDiscovery{
							IncludeOwnNamespace: false,
							Names:               []string{"operator"},
						},
					},
				},
				SampleLimit:                    18,
				ScrapeNativeHistograms:         falsePtr,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
			},
		},
		{
			name: "defaults_from_scrapeoptions",
			m: &promopv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "podmonitor",
				},
			},
			ep: promopv1.PodMetricsEndpoint{
				TargetPort: &intstr.IntOrString{IntVal: 8080, Type: intstr.Int},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- source_labels: [__meta_kubernetes_pod_phase]
				  regex: (Failed|Succeeded)
				  action: drop
				- source_labels: ["__meta_kubernetes_pod_container_port_number"]
				  regex: "8080"
				  action: "keep"
				- source_labels: [__meta_kubernetes_namespace]
				  target_label: namespace
				- source_labels: [__meta_kubernetes_pod_container_name]
				  target_label: container
				- source_labels: [__meta_kubernetes_pod_name]
				  target_label: pod
				- target_label: job
				  replacement: operator/podmonitor
				- target_label: endpoint
				  replacement: "8080"
			`),
			expected: &config.ScrapeConfig{
				JobName:                "podMonitor/operator/podmonitor/1",
				HonorTimestamps:        true,
				ScrapeInterval:         model.Duration(time.Hour),
				ScrapeTimeout:          model.Duration(42 * time.Second),
				ScrapeProtocols:        config.DefaultScrapeProtocols,
				ScrapeFallbackProtocol: config.PrometheusText0_0_4,
				EnableCompression:      true,
				MetricsPath:            "/metrics",
				Scheme:                 "http",
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					&promk8s.SDConfig{
						Role: "pod",

						NamespaceDiscovery: promk8s.NamespaceDiscovery{
							IncludeOwnNamespace: false,
							Names:               []string{"operator"},
						},
					},
				},
				SampleLimit:                    18,
				ScrapeNativeHistograms:         falsePtr,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
			},
		},
		{
			name: "everything",
			m: &promopv1.PodMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "podmonitor",
				},
				Spec: promopv1.PodMonitorSpec{
					JobLabel:        "abc",
					PodTargetLabels: []string{"label_a", "label_b"},
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "key",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{"val0", "val1"},
							},
							{
								Key:      "key",
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{"val0", "val1"},
							},
							{
								Key:      "key",
								Operator: metav1.LabelSelectorOpExists,
							},
							{
								Key:      "key",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							},
						},
					},
					ScrapeProtocols:       []promopv1.ScrapeProtocol{promopv1.ScrapeProtocol(config.PrometheusProto)},
					NamespaceSelector:     promopv1.NamespaceSelector{Any: false, MatchNames: []string{"ns_a", "ns_b"}},
					SampleLimit:           ptr.To(uint64(101)),
					TargetLimit:           ptr.To(uint64(102)),
					LabelLimit:            ptr.To(uint64(103)),
					LabelNameLengthLimit:  ptr.To(uint64(104)),
					LabelValueLengthLimit: ptr.To(uint64(105)),
					AttachMetadata:        &promopv1.AttachMetadata{Node: boolPtr(true)},
				},
			},
			ep: promopv1.PodMetricsEndpoint{
				Port:            stringPtr("metrics"),
				Path:            "/foo",
				Params:          map[string][]string{"a": {"b"}},
				Scheme:          "https",
				ScrapeTimeout:   "17s",
				Interval:        "12m",
				HonorLabels:     true,
				HonorTimestamps: falsePtr,
				FilterRunning:   falsePtr,
				RelabelConfigs: []promopv1.RelabelConfig{
					{
						SourceLabels: []promopv1.LabelName{"foo"},
						TargetLabel:  "bar",
					},
				},
				HTTPConfig: promopv1.HTTPConfig{
					EnableHTTP2:     falsePtr,
					FollowRedirects: falsePtr,
					ProxyConfig: promopv1.ProxyConfig{
						ProxyURL: &proxyURL,
					},
					TLSConfig: &promopv1.SafeTLSConfig{
						ServerName:         stringPtr("foo.com"),
						InsecureSkipVerify: boolPtr(true),
					},
				},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- action: keep
				  regex: (bar);true
				  source_labels: [__meta_kubernetes_pod_label_foo,__meta_kubernetes_pod_labelpresent_foo]
				- source_labels: [__meta_kubernetes_pod_label_key,__meta_kubernetes_pod_labelpresent_key]
				  regex: "(val0|val1);true"
				  action: keep
				  replacement: "$1"
				  separator: ";"
				- source_labels: [__meta_kubernetes_pod_label_key,__meta_kubernetes_pod_labelpresent_key]
				  regex: "(val0|val1);true"
				  replacement: "$1"
				  action: drop
				  separator: ";"
				- source_labels: [__meta_kubernetes_pod_labelpresent_key]
				  regex: true
				  action: keep
				  replacement: "$1"
				  separator: ";"
				- source_labels: [__meta_kubernetes_pod_labelpresent_key]
				  regex: true
				  action: drop
				  replacement: "$1"
				  separator: ";"
				- source_labels: [__meta_kubernetes_pod_container_port_name]
				  regex: metrics
				  action: keep
				- source_labels: [__meta_kubernetes_namespace]
				  target_label: namespace
				- source_labels: [__meta_kubernetes_pod_container_name]
				  target_label: container
				- source_labels: [__meta_kubernetes_pod_name]
				  target_label: pod
				- source_labels: [__meta_kubernetes_pod_label_label_a]
				  target_label: label_a
				  replacement: "${1}"
				  regex: "(.+)"
				- source_labels: [__meta_kubernetes_pod_label_label_b]
				  target_label: label_b
				  replacement: "${1}"
				  regex: "(.+)"
				- target_label: job
				  replacement: operator/podmonitor
				- source_labels: [__meta_kubernetes_pod_label_abc]
				  replacement: "${1}"
				  regex: "(.+)"
				  target_label: job
				- target_label: endpoint
				  replacement: metrics
				- target_label: bar
				  source_labels: [foo]
			`),
			expected: &config.ScrapeConfig{
				JobName:                "podMonitor/operator/podmonitor/1",
				HonorTimestamps:        false,
				HonorLabels:            true,
				ScrapeInterval:         model.Duration(12 * time.Minute),
				ScrapeTimeout:          model.Duration(17 * time.Second),
				ScrapeProtocols:        []config.ScrapeProtocol{config.PrometheusProto},
				ScrapeFallbackProtocol: config.PrometheusText0_0_4,
				EnableCompression:      true,
				MetricsPath:            "/foo",
				Scheme:                 "https",
				Params: url.Values{
					"a": []string{"b"},
				},
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: false,
					EnableHTTP2:     false,
					TLSConfig: commonConfig.TLSConfig{
						ServerName:         "foo.com",
						InsecureSkipVerify: true,
					},
					ProxyConfig: commonConfig.ProxyConfig{
						ProxyURL: commonConfig.URL{URL: &url.URL{Scheme: "https", Host: "proxy:8080"}},
					},
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					&promk8s.SDConfig{
						Role:           "pod",
						AttachMetadata: promk8s.AttachMetadataConfig{Node: true},
						NamespaceDiscovery: promk8s.NamespaceDiscovery{
							IncludeOwnNamespace: false,
							Names:               []string{"ns_a", "ns_b"},
						},
					},
				},
				SampleLimit:                    101,
				TargetLimit:                    102,
				LabelLimit:                     103,
				LabelNameLengthLimit:           104,
				LabelValueLengthLimit:          105,
				ScrapeNativeHistograms:         falsePtr,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
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
				ScrapeOptions: operator.ScrapeOptions{
					DefaultScrapeInterval:  time.Hour,
					DefaultScrapeTimeout:   42 * time.Second,
					ScrapeNativeHistograms: false,
					DefaultSampleLimit:     18,
				},
			}
			cfg, err := cg.GeneratePodMonitorConfig(tc.m, tc.ep, 1)
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
					fmt.Fprintln(os.Stderr, expected)
				}
			}
			checkRelabels(rlcs, tc.expectedRelabels)
			checkRelabels(mrlcs, tc.expectedMetricRelabels)
		})
	}
}

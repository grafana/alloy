package configgen

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	commonConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/prometheus/prometheus/discovery/aws"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/util"
)

func TestGenerateStaticScrapeConfigConfig(t *testing.T) {
	HTTPS := "HTTPS"
	var falsePtr = ptr.To(false)
	suite := []struct {
		name                   string
		m                      *promopv1alpha1.ScrapeConfig
		ep                     promopv1alpha1.StaticConfig
		expectedRelabels       string
		expectedMetricRelabels string
		expected               *config.ScrapeConfig
	}{
		{
			name: "default",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ep: promopv1alpha1.StaticConfig{
				Targets: []promopv1alpha1.Target{"foo", "bar"},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
				- source_labels: [__address__]
				  target_label: instance
			`),
			expected: &config.ScrapeConfig{
				JobName:                        "scrapeConfig/operator/scrapeconfig/static/1",
				HonorTimestamps:                true,
				ScrapeInterval:                 model.Duration(time.Hour),
				ScrapeTimeout:                  model.Duration(42 * time.Second),
				ScrapeProtocols:                config.DefaultScrapeProtocols,
				ScrapeFallbackProtocol:         config.PrometheusText0_0_4,
				ScrapeNativeHistograms:         falsePtr,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				EnableCompression:              true,
				MetricsPath:                    "/metrics",
				Scheme:                         "http",
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					discovery.StaticConfig{
						&targetgroup.Group{
							Targets: []model.LabelSet{{model.AddressLabel: model.LabelValue("foo")}, {model.AddressLabel: model.LabelValue("bar")}},
							Labels:  model.LabelSet{"foo": "bar"},
							Source:  "scrapeConfig/operator/scrapeconfig/static/1",
						},
					},
				},
			},
		},
		{
			name: "scrape protocols",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
				Spec: promopv1alpha1.ScrapeConfigSpec{
					ScrapeProtocols: []promopv1.ScrapeProtocol{
						promopv1.ScrapeProtocol(config.PrometheusProto),
						promopv1.ScrapeProtocol(config.OpenMetricsText1_0_0),
					},
				},
			},
			ep: promopv1alpha1.StaticConfig{
				Targets: []promopv1alpha1.Target{"foo", "bar"},
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
				- source_labels: [__address__]
				  target_label: instance
			`),
			expected: &config.ScrapeConfig{
				JobName:                       "scrapeConfig/operator/scrapeconfig/static/1",
				HonorTimestamps:               true,
				ScrapeInterval:                model.Duration(time.Hour),
				ScrapeTimeout:                 model.Duration(42 * time.Second),
				ScrapeProtocols:               []config.ScrapeProtocol{config.PrometheusProto, config.OpenMetricsText1_0_0},
				ScrapeFallbackProtocol:        config.PrometheusText0_0_4,
				ScrapeNativeHistograms:        falsePtr,
				AlwaysScrapeClassicHistograms: falsePtr,
				EnableCompression:             true,
				MetricsPath:                   "/metrics",
				Scheme:                        "http",
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					discovery.StaticConfig{
						&targetgroup.Group{
							Targets: []model.LabelSet{{model.AddressLabel: model.LabelValue("foo")}, {model.AddressLabel: model.LabelValue("bar")}},
							Labels:  model.LabelSet{"foo": "bar"},
							Source:  "scrapeConfig/operator/scrapeconfig/static/1",
						},
					},
				},
				ConvertClassicHistogramsToNHCB: ptr.To(false),
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
			},
		},
		{
			name: "lowercase schema",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
				Spec: promopv1alpha1.ScrapeConfigSpec{
					Scheme: &HTTPS,
				},
			},
			ep: promopv1alpha1.StaticConfig{
				Targets: []promopv1alpha1.Target{"foo", "bar"},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
				- source_labels: [__address__]
				  target_label: instance
			`),
			expected: &config.ScrapeConfig{
				JobName:                        "scrapeConfig/operator/scrapeconfig/static/1",
				HonorTimestamps:                true,
				ScrapeInterval:                 model.Duration(time.Hour),
				ScrapeTimeout:                  model.Duration(42 * time.Second),
				ScrapeProtocols:                config.DefaultScrapeProtocols,
				ScrapeFallbackProtocol:         config.PrometheusText0_0_4,
				ScrapeNativeHistograms:         falsePtr,
				AlwaysScrapeClassicHistograms:  falsePtr,
				ConvertClassicHistogramsToNHCB: falsePtr,
				EnableCompression:              true,
				MetricsPath:                    "/metrics",
				Scheme:                         "https",
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       model.UnderscoreEscaping.String(),
				HTTPClientConfig: commonConfig.HTTPClientConfig{
					FollowRedirects: true,
					EnableHTTP2:     true,
				},
				ServiceDiscoveryConfigs: discovery.Configs{
					discovery.StaticConfig{
						&targetgroup.Group{
							Targets: []model.LabelSet{{model.AddressLabel: model.LabelValue("foo")}, {model.AddressLabel: model.LabelValue("bar")}},
							Labels:  model.LabelSet{},
							Source:  "scrapeConfig/operator/scrapeconfig/static/1",
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
				ScrapeOptions: operator.ScrapeOptions{
					DefaultScrapeInterval:  time.Hour,
					DefaultScrapeTimeout:   42 * time.Second,
					ScrapeNativeHistograms: false,
				},
			}
			cfg, err := cg.generateStaticScrapeConfigConfig(tc.m, tc.ep, 1)
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

func TestGenerateHTTPScrapeConfigConfig(t *testing.T) {
	suite := []struct {
		name     string
		m        *promopv1alpha1.ScrapeConfig
		ep       promopv1alpha1.HTTPSDConfig
		expected *config.ScrapeConfig
	}{
		{
			name: "http service discovery",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-scrapeconfig",
				},
				Spec: promopv1alpha1.ScrapeConfigSpec{
					MetricsPath:    ptr.To("/metrics"),
					ScrapeInterval: ptr.To(promopv1.Duration("60s")),
				},
			},
			ep: promopv1alpha1.HTTPSDConfig{
				URL:             "http://example-service.test-namespace:8080/sd",
				RefreshInterval: ptr.To(promopv1.Duration("15s")),
			},
			expected: &config.ScrapeConfig{
				JobName:         "scrapeConfig/test-namespace/test-scrapeconfig/http/0",
				HonorTimestamps: true,
				ScrapeInterval:  model.Duration(60 * time.Second),
				ScrapeTimeout:   model.Duration(10 * time.Second),
				MetricsPath:     "/metrics",
				Scheme:          "http",
				ServiceDiscoveryConfigs: discovery.Configs{
					&http.SDConfig{
						HTTPClientConfig: commonConfig.DefaultHTTPClientConfig,
						RefreshInterval:  model.Duration(15 * time.Second),
						URL:              "http://example-service.test-namespace:8080/sd",
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
				ScrapeOptions: operator.ScrapeOptions{
					DefaultScrapeInterval: time.Hour,
					DefaultScrapeTimeout:  42 * time.Second,
				},
			}
			got, err := cg.generateHTTPScrapeConfigConfig(tc.m, tc.ep, 0)
			require.NoError(t, err)

			// Check job name
			assert.Equal(t, tc.expected.JobName, got.JobName)

			// Check metrics path
			assert.Equal(t, tc.expected.MetricsPath, got.MetricsPath)

			// Check scrape interval
			assert.Equal(t, tc.expected.ScrapeInterval, got.ScrapeInterval)

			// Check service discovery configs
			require.Len(t, got.ServiceDiscoveryConfigs, 1)
			httpSD, ok := got.ServiceDiscoveryConfigs[0].(*http.SDConfig)
			require.True(t, ok, "Expected HTTP SD config")
			assert.Equal(t, "http://example-service.test-namespace:8080/sd", httpSD.URL)
			assert.Equal(t, model.Duration(15*time.Second), httpSD.RefreshInterval)
		})
	}
}

func TestGenerateEc2ScrapeConfigConfig(t *testing.T) {
	suite := []struct {
		name                   string
		m                      *promopv1alpha1.ScrapeConfig
		ec2Config              promopv1alpha1.EC2SDConfig
		expectedRelabels       string
		expectedMetricRelabels string
		expected               *config.ScrapeConfig
		expectError            bool
	}{
		{
			name: "minimal EC2 config",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region: ptr.To("us-east-1"),
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
			`),
			expected: &config.ScrapeConfig{
				JobName:                "scrapeConfig/operator/scrapeconfig/ec2/1",
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
					&aws.EC2SDConfig{
						Region: "us-east-1",
					},
				},
				ConvertClassicHistogramsToNHCB: ptr.To(false),
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       "underscores",
			},
		},
		{
			name: "EC2 config with port and refresh interval",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region:          ptr.To("eu-west-1"),
				Port:            ptr.To(int32(9090)),
				RefreshInterval: ptr.To(v1.Duration("30s")),
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
			`),
			expected: &config.ScrapeConfig{
				JobName:                "scrapeConfig/operator/scrapeconfig/ec2/1",
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
					&aws.EC2SDConfig{
						Region:          "eu-west-1",
						Port:            9090,
						RefreshInterval: model.Duration(30 * time.Second),
					},
				},
				ConvertClassicHistogramsToNHCB: ptr.To(false),
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       "underscores",
			},
		},
		{
			name: "EC2 config with filters",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region: ptr.To("us-west-2"),
				Filters: []promopv1alpha1.Filter{
					{
						Name:   "instance-state-name",
						Values: []string{"running"},
					},
					{
						Name:   "tag:Environment",
						Values: []string{"production", "staging"},
					},
				},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
			`),
			expected: &config.ScrapeConfig{
				JobName:                "scrapeConfig/operator/scrapeconfig/ec2/1",
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
					&aws.EC2SDConfig{
						Region: "us-west-2",
						Filters: []*aws.EC2Filter{
							{
								Name:   "instance-state-name",
								Values: []string{"running"},
							},
							{
								Name:   "tag:Environment",
								Values: []string{"production", "staging"},
							},
						},
					},
				},
				ConvertClassicHistogramsToNHCB: ptr.To(false),
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       "underscores",
			},
		},
		{
			name: "EC2 config with role ARN",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region:  ptr.To("ap-southeast-1"),
				RoleARN: ptr.To("arn:aws:iam::123456789012:role/PrometheusRole"),
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
			`),
			expected: &config.ScrapeConfig{
				JobName:                "scrapeConfig/operator/scrapeconfig/ec2/1",
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
					&aws.EC2SDConfig{
						Region:  "ap-southeast-1",
						RoleARN: "arn:aws:iam::123456789012:role/PrometheusRole",
					},
				},
				ConvertClassicHistogramsToNHCB: ptr.To(false),
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       "underscores",
			},
		},
		{
			name: "EC2 config with HTTP client settings",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region:          ptr.To("ca-central-1"),
				FollowRedirects: ptr.To(false),
				EnableHTTP2:     ptr.To(false),
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
			`),
			expected: &config.ScrapeConfig{
				JobName:                "scrapeConfig/operator/scrapeconfig/ec2/1",
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
					&aws.EC2SDConfig{
						Region: "ca-central-1",
						HTTPClientConfig: commonConfig.HTTPClientConfig{
							FollowRedirects: false,
							EnableHTTP2:     false,
						},
					},
				},
				ConvertClassicHistogramsToNHCB: ptr.To(false),
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       "underscores",
			},
		},
		{
			name: "EC2 config with proxy settings",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region: ptr.To("eu-central-1"),
				ProxyConfig: v1.ProxyConfig{
					ProxyURL:             ptr.To("http://proxy.example.com:8080"),
					NoProxy:              ptr.To("localhost,127.0.0.1"),
					ProxyFromEnvironment: ptr.To(true),
				},
			},
			expectedRelabels: util.Untab(`
				- target_label: __meta_foo
				  replacement: bar
				- source_labels: [job]
				  target_label: __tmp_prometheus_job_name
				- replacement: operator
				  target_label: __meta_kubernetes_scrapeconfig_namespace
				- replacement: scrapeconfig
				  target_label: __meta_kubernetes_scrapeconfig_name
			`),
			expected: &config.ScrapeConfig{
				JobName:                "scrapeConfig/operator/scrapeconfig/ec2/1",
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
					&aws.EC2SDConfig{
						Region: "eu-central-1",
						HTTPClientConfig: commonConfig.HTTPClientConfig{
							ProxyConfig: commonConfig.ProxyConfig{
								ProxyURL: commonConfig.URL{
									URL: &url.URL{
										Scheme: "http",
										Host:   "proxy.example.com:8080",
									},
								},
								NoProxy:              "localhost,127.0.0.1",
								ProxyFromEnvironment: true,
							},
						},
					},
				},
				ConvertClassicHistogramsToNHCB: ptr.To(false),
				MetricNameValidationScheme:     model.LegacyValidation,
				MetricNameEscapingScheme:       "underscores",
			},
		},
		{
			name: "EC2 config with invalid refresh interval",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region:          ptr.To("us-east-1"),
				RefreshInterval: ptr.To(v1.Duration("invalid")),
			},
			expectError: true,
		},
		{
			name: "EC2 config with invalid proxy URL",
			m: &promopv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "operator",
					Name:      "scrapeconfig",
				},
			},
			ec2Config: promopv1alpha1.EC2SDConfig{
				Region: ptr.To("us-east-1"),
				ProxyConfig: v1.ProxyConfig{
					ProxyURL: ptr.To("://invalid-url"),
				},
			},
			expectError: true,
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
					DefaultScrapeInterval: time.Hour,
					DefaultScrapeTimeout:  42 * time.Second,
				},

				Secrets: &mockSecretStore{},
			}

			cfg, err := cg.generateEc2ScrapeConfigConfig(tc.m, tc.ec2Config, 1)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Check relabel configs separately
			rlcs := cfg.RelabelConfigs
			mrlcs := cfg.MetricRelabelConfigs
			cfg.RelabelConfigs = nil
			cfg.MetricRelabelConfigs = nil

			assert.Equal(t, tc.expected, cfg)

			checkRelabels := func(actual []*relabel.Config, expected string) {
				if expected == "" {
					assert.Empty(t, actual)
					return
				}
				// Load the expected relabel rules as yaml so we get the defaults put in there.
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

// mockSecretStore is a mock implementation of the SecretStore interface for testing
type mockSecretStore struct{}

func (m *mockSecretStore) GetSecretValue(_ string, sec corev1.SecretKeySelector) (string, error) {
	// Return mock values for testing
	switch sec.Name {
	case "aws-access-key":
		return "mock-access-key", nil
	case "aws-secret-key":
		return "mock-secret-key", nil
	default:
		return "mock-value", nil
	}
}

func (m *mockSecretStore) GetConfigMapValue(_ string, _ corev1.ConfigMapKeySelector) (string, error) {
	panic("not implemented yet")
}

func (m *mockSecretStore) SecretOrConfigMapValue(_ string, _ v1.SecretOrConfigMap) (string, error) {
	panic("not implemented yet")
}

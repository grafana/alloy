package configgen

// SEE https://github.com/prometheus-operator/prometheus-operator/blob/aa8222d7e9b66e9293ed11c9291ea70173021029/pkg/prometheus/promcfg.go

import (
	"cmp"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"

	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/namespacelabeler"
	common "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/aws"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/relabel"
)

func (cg *ConfigGenerator) GenerateScrapeConfigConfigs(m *promopv1alpha1.ScrapeConfig) ([]*config.ScrapeConfig, []error) {
	var (
		configs []*config.ScrapeConfig
		errors  []error
	)

	// StaticConfigs
	for i, ep := range m.Spec.StaticConfigs {
		scrapeConfig, err := cg.generateStaticScrapeConfigConfig(m, ep, i)
		if err != nil {
			errors = append(errors, err)
		} else {
			configs = append(configs, scrapeConfig)
		}
	}

	// EC2SDConfigs
	for i, ec2SdConfig := range m.Spec.EC2SDConfigs {
		scrapeConfig, err := cg.generateEc2ScrapeConfigConfig(m, ec2SdConfig, i)
		if err != nil {
			errors = append(errors, err)
		} else {
			configs = append(configs, scrapeConfig)
		}
	}

	return configs, errors
}

func (cg *ConfigGenerator) generateStaticScrapeConfigConfig(m *promopv1alpha1.ScrapeConfig, sc promopv1alpha1.StaticConfig, i int) (*config.ScrapeConfig, error) {
	relabels := cg.initRelabelings()
	metricRelabels := relabeler{}
	cfg, err := cg.commonScrapeConfigConfig(m, &relabels, &metricRelabels)
	cfg.JobName = fmt.Sprintf("scrapeConfig/%s/%s/static/%d", m.Namespace, m.Name, i)
	if err != nil {
		return nil, err
	}
	targets := []model.LabelSet{}
	for _, target := range sc.Targets {
		targets = append(targets, model.LabelSet{
			model.AddressLabel: model.LabelValue(target),
		})
	}
	labels := model.LabelSet{}
	// promote "__address__" to "instance" label.
	relabels.add(&relabel.Config{
		SourceLabels: model.LabelNames{model.AddressLabel},
		TargetLabel:  model.InstanceLabel,
	})
	for k, v := range sc.Labels {
		labels[model.LabelName(k)] = model.LabelValue(v)
	}
	config := discovery.StaticConfig{
		&targetgroup.Group{
			Targets: targets,
			Labels:  labels,
			Source:  cfg.JobName,
		},
	}
	cfg.ServiceDiscoveryConfigs = append(cfg.ServiceDiscoveryConfigs, config)
	cfg.RelabelConfigs = relabels.configs
	cfg.MetricRelabelConfigs = metricRelabels.configs
	return cfg, cfg.Validate(cg.ScrapeOptions.GlobalConfig())
}

func (cg *ConfigGenerator) commonScrapeConfigConfig(m *promopv1alpha1.ScrapeConfig, relabels *relabeler, metricRelabels *relabeler) (cfg *config.ScrapeConfig, err error) {
	cfg = cg.generateDefaultScrapeConfig()
	if m.Spec.HonorLabels != nil {
		cfg.HonorLabels = *m.Spec.HonorLabels
	}
	if m.Spec.HonorTimestamps != nil {
		cfg.HonorTimestamps = *m.Spec.HonorTimestamps
	}
	if m.Spec.ScrapeInterval != nil {
		if cfg.ScrapeInterval, err = model.ParseDuration(string(*m.Spec.ScrapeInterval)); err != nil {
			return nil, fmt.Errorf("parsing interval from scrapeConfig: %w", err)
		}
	}
	if m.Spec.ScrapeTimeout != nil {
		if cfg.ScrapeTimeout, err = model.ParseDuration(string(*m.Spec.ScrapeTimeout)); err != nil {
			return nil, fmt.Errorf("parsing timeout from scrapeConfig: %w", err)
		}
	}
	if m.Spec.MetricsPath != nil {
		cfg.MetricsPath = *m.Spec.MetricsPath
	}
	if m.Spec.Params != nil {
		cfg.Params = m.Spec.Params
	}
	if m.Spec.Scheme != nil {
		// Prometheus Operator ScrapeConfig CRD requires spec.scheme to be uppercase "HTTP" or "HTTPS", but
		// the implementation expects lowercase "http" or "https" in the final scrape configuration. So, we
		// have to lowercase the schema.
		cfg.Scheme = strings.ToLower(*m.Spec.Scheme)
	}
	if m.Spec.TLSConfig != nil {
		if cfg.HTTPClientConfig.TLSConfig, err = cg.generateSafeTLS(*m.Spec.TLSConfig, m.Namespace); err != nil {
			return nil, err
		}
	}
	if m.Spec.BasicAuth != nil {
		cfg.HTTPClientConfig.BasicAuth, err = cg.generateBasicAuth(*m.Spec.BasicAuth, m.Namespace)
		if err != nil {
			return nil, err
		}
	}
	if m.Spec.Authorization != nil {
		cfg.HTTPClientConfig.Authorization, err = cg.generateAuthorization(*m.Spec.Authorization, m.Namespace)
		if err != nil {
			return nil, err
		}
	}
	relabels.add(&relabel.Config{
		Replacement: m.Namespace,
		TargetLabel: "__meta_kubernetes_scrapeconfig_namespace",
	}, &relabel.Config{
		Replacement: m.Name,
		TargetLabel: "__meta_kubernetes_scrapeconfig_name",
	})
	labeler := namespacelabeler.New("", nil, false)
	if err = relabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, m.Spec.RelabelConfigs)...); err != nil {
		return nil, fmt.Errorf("parsing relabel configs: %w", err)
	}
	if err = metricRelabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, m.Spec.MetricRelabelConfigs)...); err != nil {
		return nil, fmt.Errorf("parsing metric relabel configs: %w", err)
	}
	cfg.SampleLimit = uint(defaultIfNil(m.Spec.SampleLimit, 0))
	cfg.TargetLimit = uint(defaultIfNil(m.Spec.TargetLimit, 0))
	cfg.LabelLimit = uint(defaultIfNil(m.Spec.LabelLimit, 0))
	cfg.LabelNameLengthLimit = uint(defaultIfNil(m.Spec.LabelNameLengthLimit, 0))
	cfg.LabelValueLengthLimit = uint(defaultIfNil(m.Spec.LabelValueLengthLimit, 0))
	return cfg, err
}

// generateEc2ScrapeConfigConfig generates a Prometheus scrape config for EC2 service discovery.
// Adapted from https://github.com/prometheus-operator/prometheus-operator/blob/main/pkg/prometheus/promcfg.go#L3590
func (cg *ConfigGenerator) generateEc2ScrapeConfigConfig(m *promopv1alpha1.ScrapeConfig, ec2Sd promopv1alpha1.EC2SDConfig, i int) (cfg *config.ScrapeConfig, err error) {
	relabels := cg.initRelabelings()
	metricRelabels := relabeler{}
	cfg, err = cg.commonScrapeConfigConfig(m, &relabels, &metricRelabels)
	cfg.JobName = fmt.Sprintf("scrapeConfig/%s/%s/ec2/%d", m.Namespace, m.Name, i)
	if err != nil {
		return nil, err
	}

	sdConfig := &aws.EC2SDConfig{}

	if ec2Sd.Region != nil {
		sdConfig.Region = *ec2Sd.Region
	}

	if ec2Sd.AccessKey != nil && ec2Sd.SecretKey != nil {
		accessKey, err := cg.Secrets.GetSecretValue(m.Namespace, *ec2Sd.AccessKey)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve AWS access key from secret %s: %w", ec2Sd.AccessKey.Name, err)
		}
		secretKey, err := cg.Secrets.GetSecretValue(m.Namespace, *ec2Sd.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve AWS secret key from secret %s: %w", ec2Sd.SecretKey.Name, err)
		}

		sdConfig.AccessKey = accessKey
		sdConfig.SecretKey = common.Secret(secretKey)
	}

	if ec2Sd.RoleARN != nil {
		sdConfig.RoleARN = *ec2Sd.RoleARN
	}

	if ec2Sd.RefreshInterval != nil {
		refreshInterval, err := model.ParseDuration(string(*ec2Sd.RefreshInterval))
		if err != nil {
			return nil, fmt.Errorf("failed to parse refresh interval: %w", err)
		}
		sdConfig.RefreshInterval = refreshInterval
	}

	if ec2Sd.Port != nil {
		sdConfig.Port = int(*ec2Sd.Port)
	}

	if len(ec2Sd.Filters) > 0 {
		// Sort the filters by name to generate a deterministic config.
		slices.SortStableFunc(ec2Sd.Filters, func(a, b promopv1alpha1.Filter) int {
			return cmp.Compare(a.Name, b.Name)
		})

		for _, filter := range ec2Sd.Filters {
			sdConfig.Filters = append(sdConfig.Filters, &aws.EC2Filter{
				Name:   filter.Name,
				Values: filter.Values,
			})
		}
	}

	if ec2Sd.FollowRedirects != nil {
		sdConfig.HTTPClientConfig.FollowRedirects = *ec2Sd.FollowRedirects
	}

	if ec2Sd.EnableHTTP2 != nil {
		sdConfig.HTTPClientConfig.EnableHTTP2 = *ec2Sd.EnableHTTP2
	}

	if ec2Sd.TLSConfig != nil {
		if sdConfig.HTTPClientConfig.TLSConfig, err = cg.generateSafeTLS(*ec2Sd.TLSConfig, m.Namespace); err != nil {
			return nil, err
		}
	}

	// Proxy settings
	if !reflect.ValueOf(ec2Sd.ProxyConfig).IsZero() {
		if ec2Sd.ProxyURL != nil {
			u, err := url.Parse(*ec2Sd.ProxyURL)
			if err != nil {
				return nil, fmt.Errorf("failed to parse proxy URL %q: %w", *ec2Sd.ProxyURL, err)
			}
			sdConfig.HTTPClientConfig.ProxyURL = common.URL{URL: u}
		}

		if ec2Sd.NoProxy != nil {
			sdConfig.HTTPClientConfig.NoProxy = *ec2Sd.NoProxy
		}

		if ec2Sd.ProxyFromEnvironment != nil {
			sdConfig.HTTPClientConfig.ProxyFromEnvironment = *ec2Sd.ProxyFromEnvironment
		}

		if ec2Sd.ProxyConnectHeader != nil {
			proxyConnectHeader := make(common.ProxyHeader)
			for k, v := range ec2Sd.ProxyConnectHeader {
				proxyConnectHeader[k] = make([]common.Secret, len(v))
				for _, s := range v {
					value, _ := cg.Secrets.GetSecretValue(m.Namespace, s)
					proxyConnectHeader[k] = append(proxyConnectHeader[k], common.Secret(value))
				}
			}
			sdConfig.HTTPClientConfig.ProxyConnectHeader = proxyConnectHeader
		}
	}

	cfg.ServiceDiscoveryConfigs = append(cfg.ServiceDiscoveryConfigs, sdConfig)
	cfg.RelabelConfigs = relabels.configs
	cfg.MetricRelabelConfigs = metricRelabels.configs
	return cfg, cfg.Validate(cg.ScrapeOptions.GlobalConfig())
}

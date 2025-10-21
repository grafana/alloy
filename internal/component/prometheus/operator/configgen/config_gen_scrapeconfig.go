package configgen

// SEE https://github.com/prometheus-operator/prometheus-operator/blob/aa8222d7e9b66e9293ed11c9291ea70173021029/pkg/prometheus/promcfg.go

import (
	"fmt"
	"strings"

	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/namespacelabeler"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/relabel"
)

func (cg *ConfigGenerator) GenerateScrapeConfigConfigs(m *promopv1alpha1.ScrapeConfig) (cfg []*config.ScrapeConfig, errors []error) {
	cfg, errors = cg.generateStaticScrapeConfigConfigs(m, cfg, errors)
	return
}

func (cg *ConfigGenerator) generateStaticScrapeConfigConfigs(m *promopv1alpha1.ScrapeConfig, cfg []*config.ScrapeConfig, errors []error) ([]*config.ScrapeConfig, []error) {
	for i, ep := range m.Spec.StaticConfigs {
		scrapeConfig, err := cg.generateStaticScrapeConfigConfig(m, ep, i)
		if err != nil {
			errors = append(errors, err)
		} else {
			cfg = append(cfg, scrapeConfig)
		}
	}
	return cfg, errors
}

func (cg *ConfigGenerator) generateStaticScrapeConfigConfig(m *promopv1alpha1.ScrapeConfig, sc promopv1alpha1.StaticConfig, i int) (cfg *config.ScrapeConfig, err error) {
	relabels := cg.initRelabelings()
	metricRelabels := relabeler{}
	cfg, err = cg.commonScrapeConfigConfig(m, i, &relabels, &metricRelabels)
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

func (cg *ConfigGenerator) commonScrapeConfigConfig(m *promopv1alpha1.ScrapeConfig, _ int, relabels *relabeler, metricRelabels *relabeler) (cfg *config.ScrapeConfig, err error) {
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

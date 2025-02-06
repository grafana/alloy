package configgen

// SEE https://github.com/prometheus-operator/prometheus-operator/blob/aa8222d7e9b66e9293ed11c9291ea70173021029/pkg/prometheus/promcfg.go

import (
	"fmt"

	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus-operator/prometheus-operator/pkg/namespacelabeler"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

func (cg *ConfigGenerator) GenerateStaticScrapeConfigConfig(m *promopv1alpha1.ScrapeConfig, sc promopv1alpha1.StaticConfig, i int) (cfg *config.ScrapeConfig, err error) {
	cfg, err = cg.commonScrapeConfigConfig(m, i)
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
	return cfg, cfg.Validate(cg.ScrapeOptions.GlobalConfig())
}

func (cg *ConfigGenerator) commonScrapeConfigConfig(m *promopv1alpha1.ScrapeConfig, i int) (cfg *config.ScrapeConfig, err error) {
	cfg = cg.generateDefaultScrapeConfig()
	cfg.JobName = fmt.Sprintf("scrapeConfig/%s/%s/%d", m.Namespace, m.Name, i)
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
		cfg.Scheme = *m.Spec.Scheme
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
	relabels := cg.initRelabelings()
	labeler := namespacelabeler.New("", nil, false)
	if err = relabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, m.Spec.RelabelConfigs)...); err != nil {
		return nil, fmt.Errorf("parsing relabel configs: %w", err)
	}
	cfg.RelabelConfigs = relabels.configs
	metricRelabels := relabeler{}
	if err = metricRelabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, m.Spec.MetricRelabelConfigs)...); err != nil {
		return nil, fmt.Errorf("parsing metric relabel configs: %w", err)
	}
	cfg.MetricRelabelConfigs = metricRelabels.configs
	cfg.SampleLimit = uint(defaultIfNil(m.Spec.SampleLimit, 0))
	cfg.TargetLimit = uint(defaultIfNil(m.Spec.TargetLimit, 0))
	cfg.LabelLimit = uint(defaultIfNil(m.Spec.LabelLimit, 0))
	cfg.LabelNameLengthLimit = uint(defaultIfNil(m.Spec.LabelNameLengthLimit, 0))
	cfg.LabelValueLengthLimit = uint(defaultIfNil(m.Spec.LabelValueLengthLimit, 0))
	return cfg, err
}

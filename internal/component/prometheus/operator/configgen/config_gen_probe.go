package configgen

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/namespacelabeler"
	commonConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/relabel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// See https://github.com/prometheus-operator/prometheus-operator/blob/aa8222d7e9b66e9293ed11c9291ea70173021029/pkg/prometheus/promcfg.go#L835

func (cg *ConfigGenerator) GenerateProbeConfig(m *promopv1.Probe) (cfg *config.ScrapeConfig, err error) {
	cfg = cg.generateDefaultScrapeConfig()

	cfg.JobName = fmt.Sprintf("probe/%s/%s", m.Namespace, m.Name)
	cfg.HonorTimestamps = true
	cfg.MetricsPath = m.Spec.ProberSpec.Path
	if m.Spec.Interval != "" {
		cfg.ScrapeInterval, _ = model.ParseDuration(string(m.Spec.Interval))
	}
	if m.Spec.ScrapeTimeout != "" {
		cfg.ScrapeInterval, _ = model.ParseDuration(string(m.Spec.ScrapeTimeout))
	}
	if m.Spec.ProberSpec.Scheme != "" {
		cfg.Scheme = m.Spec.ProberSpec.Scheme
	}
	if m.Spec.ProberSpec.ProxyURL != "" {
		if u, err := url.Parse(m.Spec.ProberSpec.ProxyURL); err != nil {
			return nil, fmt.Errorf("parsing ProxyURL from probe: %w", err)
		} else {
			cfg.HTTPClientConfig.ProxyURL = commonConfig.URL{URL: u}
		}
	}
	if m.Spec.Module != "" {
		cfg.Params = url.Values{}
		cfg.Params.Set("module", m.Spec.Module)
	}

	cfg.SampleLimit = uint(m.Spec.SampleLimit)
	cfg.TargetLimit = uint(m.Spec.TargetLimit)
	cfg.LabelLimit = uint(m.Spec.LabelLimit)
	cfg.LabelNameLengthLimit = uint(m.Spec.LabelNameLengthLimit)
	cfg.LabelValueLengthLimit = uint(m.Spec.LabelValueLengthLimit)

	relabels := cg.initRelabelings()
	if m.Spec.JobName != "" {
		relabels.add(&relabel.Config{
			Replacement: m.Spec.JobName,
			TargetLabel: "job",
		})
	}
	labeler := namespacelabeler.New("", nil, false)

	static := m.Spec.Targets.StaticConfig
	if static != nil {
		// Generate static_config section.
		grp := &targetgroup.Group{
			Labels: model.LabelSet{},
		}
		for k, v := range static.Labels {
			grp.Labels[model.LabelName(k)] = model.LabelValue(v)
		}
		// Add a namespace label if not already set. See https://github.com/prometheus-operator/prometheus-operator/commit/ffce8d01ff798135d5bbfcba1a88a87a418ee9fc.
		if _, ok := grp.Labels["namespace"]; !ok {
			grp.Labels["namespace"] = model.LabelValue(m.Namespace)
		}
		for _, t := range static.Targets {
			grp.Targets = append(grp.Targets, model.LabelSet{
				model.AddressLabel: model.LabelValue(t),
			})
		}
		sc := discovery.StaticConfig{grp}
		relabels.add(&relabel.Config{
			SourceLabels: model.LabelNames{"__address__"},
			TargetLabel:  "__param_target",
		})
		relabels.add(&relabel.Config{
			SourceLabels: model.LabelNames{"__param_target"},
			TargetLabel:  "instance",
		})
		relabels.add(&relabel.Config{
			Replacement: m.Spec.ProberSpec.URL,
			TargetLabel: "__address__",
		})
		// Add configured relabelings.
		if err = relabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, m.Spec.Targets.StaticConfig.RelabelConfigs)...); err != nil {
			return nil, fmt.Errorf("parsing relabel configs: %w", err)
		}
		cfg.ServiceDiscoveryConfigs = append(cfg.ServiceDiscoveryConfigs, sc)
	} else if m.Spec.Targets.Ingress != nil {
		// Generate kubernetes_sd_config section for the ingress resources.
		labelKeys := make([]string, 0, len(m.Spec.Targets.Ingress.Selector.MatchLabels))
		for k := range m.Spec.Targets.Ingress.Selector.MatchLabels {
			labelKeys = append(labelKeys, k)
		}
		sort.Strings(labelKeys)
		for _, k := range labelKeys {
			regex, err := relabel.NewRegexp(fmt.Sprintf("(%s);true", m.Spec.Targets.Ingress.Selector.MatchLabels[k]))
			if err != nil {
				return nil, fmt.Errorf("parsing MatchLabels regex: %w", err)
			}
			relabels.add(&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_ingress_label_" + sanitizeLabelName(k), "__meta_kubernetes_ingress_labelpresent_" + sanitizeLabelName(k)},
				Action:       "keep",
				Regex:        regex,
			})
		}
		for _, exp := range m.Spec.Targets.Ingress.Selector.MatchExpressions {
			switch exp.Operator {
			case metav1.LabelSelectorOpIn:
				regex, err := relabel.NewRegexp(fmt.Sprintf("(%s);true", strings.Join(exp.Values, "|")))
				if err != nil {
					return nil, fmt.Errorf("parsing MatchExpressions regex: %w", err)
				}
				relabels.add(&relabel.Config{
					SourceLabels: model.LabelNames{"__meta_kubernetes_ingress_label_" + sanitizeLabelName(exp.Key), "__meta_kubernetes_ingress_labelpresent_" + sanitizeLabelName(exp.Key)},
					Action:       "keep",
					Regex:        regex,
				})
			case metav1.LabelSelectorOpNotIn:
				regex, err := relabel.NewRegexp(fmt.Sprintf("(%s);true", strings.Join(exp.Values, "|")))
				if err != nil {
					return nil, fmt.Errorf("parsing MatchExpressions regex: %w", err)
				}
				relabels.add(&relabel.Config{
					SourceLabels: model.LabelNames{"__meta_kubernetes_ingress_label_" + sanitizeLabelName(exp.Key), "__meta_kubernetes_ingress_labelpresent_" + sanitizeLabelName(exp.Key)},
					Action:       "drop",
					Regex:        regex,
				})
			case metav1.LabelSelectorOpExists:
				relabels.add(&relabel.Config{
					SourceLabels: model.LabelNames{"__meta_kubernetes_ingress_labelpresent_" + sanitizeLabelName(exp.Key)},
					Action:       "keep",
					Regex:        regexTrue,
				})
			case metav1.LabelSelectorOpDoesNotExist:
				relabels.add(&relabel.Config{
					SourceLabels: model.LabelNames{"__meta_kubernetes_ingress_labelpresent_" + sanitizeLabelName(exp.Key)},
					Action:       "drop",
					Regex:        regexTrue,
				})
			}
		}
		dConfig := cg.generateK8SSDConfig(m.Spec.Targets.Ingress.NamespaceSelector, m.Namespace, promk8s.RoleIngress, nil)
		cfg.ServiceDiscoveryConfigs = append(cfg.ServiceDiscoveryConfigs, dConfig)

		// Relabelings for ingress SD.
		regex := relabel.MustNewRegexp("(.+);(.+);(.+)")
		relabels.add(
			&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_ingress_scheme", "__address__", "__meta_kubernetes_ingress_path"},
				TargetLabel:  "__param_target",
				Separator:    ";",
				Regex:        regex,
				Replacement:  "${1}://${2}${3}",
			},
			&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_namespace"},
				TargetLabel:  "namespace",
			},
			&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_ingress_name"},
				TargetLabel:  "ingress",
			})
		// Relabelings for prober.
		relabels.add(
			&relabel.Config{
				SourceLabels: model.LabelNames{"__address__"},
				TargetLabel:  "__tmp_ingress_address",
				Separator:    ";",
				Regex:        regexAnything,
				Replacement:  "$1",
			},
			&relabel.Config{
				SourceLabels: model.LabelNames{"__param_target"},
				TargetLabel:  "instance",
			},
			&relabel.Config{
				Replacement: m.Spec.ProberSpec.URL,
				TargetLabel: "__address__",
			})
		// Add configured relabelings.
		if err = relabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, m.Spec.Targets.Ingress.RelabelConfigs)...); err != nil {
			return nil, fmt.Errorf("parsing relabel configs: %w", err)
		}
	}
	cfg.RelabelConfigs = relabels.configs
	if m.Spec.TLSConfig != nil {
		if cfg.HTTPClientConfig.TLSConfig, err = cg.generateSafeTLS(m.Spec.TLSConfig.SafeTLSConfig, m.Namespace); err != nil {
			return nil, err
		}
	}
	if m.Spec.BearerTokenSecret.Name != "" {
		val, err := cg.Secrets.GetSecretValue(m.Namespace, m.Spec.BearerTokenSecret)
		if err != nil {
			return nil, err
		}
		cfg.HTTPClientConfig.BearerToken = commonConfig.Secret(val)
	}
	if m.Spec.BasicAuth != nil {
		cfg.HTTPClientConfig.BasicAuth, err = cg.generateBasicAuth(*m.Spec.BasicAuth, m.Namespace)
		if err != nil {
			return nil, err
		}
	}
	if m.Spec.OAuth2 != nil {
		cfg.HTTPClientConfig.OAuth2, err = cg.generateOauth2(*m.Spec.OAuth2, m.Namespace)
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

	metricRelabels := relabeler{}
	err = metricRelabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, m.Spec.MetricRelabelConfigs)...)
	if err != nil {
		return nil, err
	}
	cfg.MetricRelabelConfigs = metricRelabels.configs

	return cfg, cfg.Validate(cg.ScrapeOptions.GlobalConfig())
}

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
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cg *ConfigGenerator) GenerateServiceMonitorConfig(m *promopv1.ServiceMonitor, ep promopv1.Endpoint, i int, role promk8s.Role) (cfg *config.ScrapeConfig, err error) {
	cfg = cg.generateDefaultScrapeConfig()

	cfg.JobName = fmt.Sprintf("serviceMonitor/%s/%s/%d", m.Namespace, m.Name, i)
	cfg.HonorLabels = ep.HonorLabels
	if ep.HonorTimestamps != nil {
		cfg.HonorTimestamps = *ep.HonorTimestamps
	}
	dConfig := cg.generateK8SSDConfig(m.Spec.NamespaceSelector, m.Namespace, role, m.Spec.AttachMetadata)
	cfg.ServiceDiscoveryConfigs = append(cfg.ServiceDiscoveryConfigs, dConfig)

	if m.Spec.ScrapeProtocols != nil {
		protocols, err := convertScrapeProtocols(m.Spec.ScrapeProtocols)
		if err != nil {
			return nil, err
		}
		cfg.ScrapeProtocols = protocols
	}

	if ep.Interval != "" {
		if cfg.ScrapeInterval, err = model.ParseDuration(string(ep.Interval)); err != nil {
			return nil, fmt.Errorf("parsing interval from serviceMonitor: %w", err)
		}
	}
	if ep.ScrapeTimeout != "" {
		if cfg.ScrapeTimeout, err = model.ParseDuration(string(ep.ScrapeTimeout)); err != nil {
			return nil, fmt.Errorf("parsing timeout from serviceMonitor: %w", err)
		}
	}
	if ep.Path != "" {
		cfg.MetricsPath = ep.Path
	}
	if ep.ProxyURL != nil {
		if u, err := url.Parse(*ep.ProxyURL); err != nil {
			return nil, fmt.Errorf("parsing ProxyURL from serviceMonitor: %w", err)
		} else {
			cfg.HTTPClientConfig.ProxyURL = commonConfig.URL{URL: u}
		}
	}
	if ep.Params != nil {
		cfg.Params = ep.Params
	}
	if ep.Scheme != "" {
		cfg.Scheme = ep.Scheme
	}
	if ep.FollowRedirects != nil {
		cfg.HTTPClientConfig.FollowRedirects = *ep.FollowRedirects
	}
	if ep.EnableHttp2 != nil {
		cfg.HTTPClientConfig.EnableHTTP2 = *ep.EnableHttp2
	}
	if ep.TLSConfig != nil {
		if cfg.HTTPClientConfig.TLSConfig, err = cg.generateTLSConfig(*ep.TLSConfig, m.Namespace); err != nil {
			return nil, err
		}
	}
	if ep.BearerTokenFile != "" { //nolint:staticcheck
		cfg.HTTPClientConfig.BearerTokenFile = ep.BearerTokenFile //nolint:staticcheck
	} else if ep.BearerTokenSecret != nil && ep.BearerTokenSecret.Name != "" { //nolint:staticcheck
		val, err := cg.Secrets.GetSecretValue(m.Namespace, *ep.BearerTokenSecret) //nolint:staticcheck
		if err != nil {
			return nil, err
		}
		cfg.HTTPClientConfig.BearerToken = commonConfig.Secret(val)
	}
	if ep.BasicAuth != nil {
		cfg.HTTPClientConfig.BasicAuth, err = cg.generateBasicAuth(*ep.BasicAuth, m.Namespace)
		if err != nil {
			return nil, err
		}
	}
	if ep.OAuth2 != nil {
		cfg.HTTPClientConfig.OAuth2, err = cg.generateOauth2(*ep.OAuth2, m.Namespace)
		if err != nil {
			return nil, err
		}
	}
	if ep.Authorization != nil {
		cfg.HTTPClientConfig.Authorization, err = cg.generateAuthorization(*ep.Authorization, m.Namespace)
		if err != nil {
			return nil, err
		}
	}

	relabels := cg.initRelabelings()

	// Filter targets by services selected by the monitor.

	// Exact label matches.
	var labelKeys []string
	for k := range m.Spec.Selector.MatchLabels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)
	for _, k := range labelKeys {
		regex, err := relabel.NewRegexp(fmt.Sprintf("(%s);true", m.Spec.Selector.MatchLabels[k]))
		if err != nil {
			return nil, fmt.Errorf("parsing MatchLabels regex: %w", err)
		}
		relabels.add(&relabel.Config{
			SourceLabels: model.LabelNames{"__meta_kubernetes_service_label_" + sanitizeLabelName(k), "__meta_kubernetes_service_labelpresent_" + sanitizeLabelName(k)},
			Action:       "keep",
			Regex:        regex,
		})
	}

	// Set based label matching. We have to map the valid relations
	//  `In`, `NotIn`, `Exists`, and `DoesNotExist`, into relabeling rules.
	for _, exp := range m.Spec.Selector.MatchExpressions {
		switch exp.Operator {
		case metav1.LabelSelectorOpIn:
			regex, err := relabel.NewRegexp(fmt.Sprintf("(%s);true", strings.Join(exp.Values, "|")))
			if err != nil {
				return nil, fmt.Errorf("parsing MatchExpressions regex: %w", err)
			}
			relabels.add(&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key), "__meta_kubernetes_service_labelpresent_" + sanitizeLabelName(exp.Key)},
				Action:       "keep",
				Regex:        regex,
			})
		case metav1.LabelSelectorOpNotIn:
			regex, err := relabel.NewRegexp(fmt.Sprintf("(%s);true", strings.Join(exp.Values, "|")))
			if err != nil {
				return nil, fmt.Errorf("parsing MatchExpressions regex: %w", err)
			}
			relabels.add(&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_service_label_" + sanitizeLabelName(exp.Key), "__meta_kubernetes_service_labelpresent_" + sanitizeLabelName(exp.Key)},
				Action:       "drop",
				Regex:        regex,
			})
		case metav1.LabelSelectorOpExists:
			relabels.add(&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_service_labelpresent_" + sanitizeLabelName(exp.Key)},
				Action:       "keep",
				Regex:        regexTrue,
			})
		case metav1.LabelSelectorOpDoesNotExist:
			relabels.add(&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_service_labelpresent_" + sanitizeLabelName(exp.Key)},
				Action:       "drop",
				Regex:        regexTrue,
			})
		}
	}

	labelPortName := "__meta_kubernetes_endpoint_port_name"
	if role == promk8s.RoleEndpointSlice {
		labelPortName = "__meta_kubernetes_endpointslice_port_name"
	}
	// Filter targets based on correct port for the endpoint.
	if ep.Port != "" {
		regex, err := relabel.NewRegexp(ep.Port)
		if err != nil {
			return nil, fmt.Errorf("parsing Port as regex: %w", err)
		}
		relabels.add(&relabel.Config{
			SourceLabels: model.LabelNames{model.LabelName(labelPortName)},
			Action:       "keep",
			Regex:        regex,
		})
	} else if ep.TargetPort != nil { //nolint:staticcheck // Ignore SA1019 this field is marked as deprecated.
		if ep.TargetPort.StrVal != "" { //nolint:staticcheck // Ignore SA1019 this field is marked as deprecated.
			regex, err := relabel.NewRegexp(ep.TargetPort.String()) //nolint:staticcheck // Ignore SA1019 this field is marked as deprecated.
			if err != nil {
				return nil, fmt.Errorf("parsing TargetPort as regex: %w", err)
			}
			if ep.TargetPort.StrVal != "" {
				relabels.add(&relabel.Config{
					SourceLabels: model.LabelNames{"__meta_kubernetes_pod_container_port_name"},
					Action:       "keep",
					Regex:        regex,
				})
			}
		} else if ep.TargetPort.IntVal != 0 { //nolint:staticcheck // Ignore SA1019 this field is marked as deprecated.
			regex, err := relabel.NewRegexp(fmt.Sprint(ep.TargetPort.IntValue())) //nolint:staticcheck // Ignore SA1019 this field is marked as deprecated.
			if err != nil {
				return nil, fmt.Errorf("parsing TargetPort as regex: %w", err)
			}
			relabels.add(&relabel.Config{
				SourceLabels: model.LabelNames{"__meta_kubernetes_pod_container_port_number"},
				Action:       "keep",
				Regex:        regex,
			})
		}
	}

	sourceLabels := model.LabelNames{"__meta_kubernetes_endpoint_address_target_kind", "__meta_kubernetes_endpoint_address_target_name"}
	if role == promk8s.RoleEndpointSlice {
		sourceLabels = model.LabelNames{"__meta_kubernetes_endpointslice_address_target_kind", "__meta_kubernetes_endpointslice_address_target_name"}
	}
	// Relabel namespace and pod and service labels into proper labels.
	// Relabel node labels with meta labels available with Prometheus >= v2.3.
	relabels.add(&relabel.Config{
		SourceLabels: sourceLabels,
		Separator:    ";",
		Regex:        regexNode,
		Replacement:  "${1}",
		TargetLabel:  "node",
	})
	// Relabel pod labels for >=v2.3 meta labels
	relabels.add(&relabel.Config{
		SourceLabels: sourceLabels,
		Separator:    ";",
		Regex:        regexPod,
		Replacement:  "${1}",
		TargetLabel:  "pod",
	})
	relabels.add(&relabel.Config{
		SourceLabels: model.LabelNames{"__meta_kubernetes_namespace"},
		TargetLabel:  "namespace",
	}, &relabel.Config{
		SourceLabels: model.LabelNames{"__meta_kubernetes_service_name"},
		TargetLabel:  "service",
	}, &relabel.Config{
		SourceLabels: model.LabelNames{"__meta_kubernetes_pod_container_name"},
		TargetLabel:  "container",
	}, &relabel.Config{
		SourceLabels: model.LabelNames{"__meta_kubernetes_pod_name"},
		TargetLabel:  "pod",
	})

	if ep.FilterRunning == nil || *ep.FilterRunning {
		relabels.add(&relabel.Config{
			SourceLabels: model.LabelNames{"__meta_kubernetes_pod_phase"},
			Action:       "drop",
			Regex:        regexFilterRunning,
		})
	}
	//  Relabel targetLabels from Service onto target.
	for _, l := range m.Spec.TargetLabels {
		relabels.add(&relabel.Config{
			SourceLabels: model.LabelNames{"__meta_kubernetes_service_label_" + sanitizeLabelName(l)},
			Replacement:  "${1}",
			Regex:        regexAnything,
			TargetLabel:  string(sanitizeLabelName(l)),
		})
	}
	for _, l := range m.Spec.PodTargetLabels {
		relabels.add(&relabel.Config{
			SourceLabels: model.LabelNames{"__meta_kubernetes_pod_label_" + sanitizeLabelName(l)},
			Replacement:  "${1}",
			Regex:        regexAnything,
			TargetLabel:  string(sanitizeLabelName(l)),
		})
	}

	// By default, generate a safe job name from the service name.  We also keep
	// this around if a jobLabel is set in case the targets don't actually have a
	// value for it.

	relabels.add(&relabel.Config{
		SourceLabels: model.LabelNames{"__meta_kubernetes_service_name"},
		Replacement:  "${1}",
		TargetLabel:  "job",
	})
	if m.Spec.JobLabel != "" {
		relabels.add(&relabel.Config{
			Replacement:  "${1}",
			TargetLabel:  "job",
			Regex:        regexAnything,
			SourceLabels: model.LabelNames{"__meta_kubernetes_service_label_" + sanitizeLabelName(m.Spec.JobLabel)},
		})
	}

	// A single service may potentially have multiple metrics
	//	endpoints, therefore the endpoints labels is filled with the ports name or
	//	as a fallback the port number.

	if ep.Port != "" {
		relabels.add(&relabel.Config{
			Replacement: ep.Port,
			TargetLabel: "endpoint",
		})
	} else if ep.TargetPort != nil && ep.TargetPort.String() != "" {
		relabels.add(&relabel.Config{
			TargetLabel: "endpoint",
			Replacement: ep.TargetPort.String(),
		})
	}

	labeler := namespacelabeler.New("", nil, false)
	err = relabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, ep.RelabelConfigs)...)
	if err != nil {
		return nil, err
	}
	cfg.RelabelConfigs = relabels.configs

	metricRelabels := relabeler{}
	err = metricRelabels.addFromV1(labeler.GetRelabelingConfigs(m.TypeMeta, m.ObjectMeta, ep.MetricRelabelConfigs)...)
	if err != nil {
		return nil, err
	}
	cfg.MetricRelabelConfigs = metricRelabels.configs

	cfg.SampleLimit = uint(defaultIfNil(m.Spec.SampleLimit, 0))
	cfg.TargetLimit = uint(defaultIfNil(m.Spec.TargetLimit, 0))
	cfg.LabelLimit = uint(defaultIfNil(m.Spec.LabelLimit, 0))
	cfg.LabelNameLengthLimit = uint(defaultIfNil(m.Spec.LabelNameLengthLimit, 0))
	cfg.LabelValueLengthLimit = uint(defaultIfNil(m.Spec.LabelValueLengthLimit, 0))

	return cfg, cfg.Validate(cg.ScrapeOptions.GlobalConfig())
}

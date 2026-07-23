package operator

import (
	"fmt"
	"time"

	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"

	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/storage"
	apiv1 "k8s.io/api/core/v1"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/service/cluster"
)

type Arguments struct {
	// Client settings to connect to Kubernetes.
	Client kubernetes.ClientArguments `alloy:"client,block,optional"`

	ForwardTo []storage.Appendable `alloy:"forward_to,attr"`

	// Namespaces to search for monitor resources. Empty implies All namespaces
	Namespaces []string `alloy:"namespaces,attr,optional"`

	KubernetesRole string `alloy:"kubernetes_role,attr,optional"`

	// LabelSelector allows filtering discovered monitor resources by labels
	LabelSelector *config.LabelSelector `alloy:"selector,block,optional"`

	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`

	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`

	Scrape ScrapeOptions `alloy:"scrape,block,optional"`

	// ScrapeClasses defines a set of named scrape classes that discovered
	// resources can reference through their `scrapeClass` field, mirroring the
	// Prometheus Operator ScrapeClass feature.
	ScrapeClasses []ScrapeClass `alloy:"scrape_class,block,optional"`

	InformerSyncTimeout time.Duration `alloy:"informer_sync_timeout,attr,optional"`
}

// ScrapeClass holds common scrape settings that are applied to discovered
// resources referencing it by name. It mirrors the Prometheus Operator
// ScrapeClass: https://prometheus-operator.dev/docs/developer/scrapeclass/
type ScrapeClass struct {
	// Name is the unique identifier of the scrape class, referenced by a
	// resource's `scrapeClass` field.
	Name string `alloy:"name,attr"`

	// Default marks this class as the one applied to resources that do not
	// reference a scrape class. At most one class may be the default.
	Default bool `alloy:"default,attr,optional"`

	// TLSConfig is applied to a resource's endpoints unless the endpoint sets
	// its own TLS configuration.
	TLSConfig *config.TLSConfig `alloy:"tls_config,block,optional"`

	// Authorization is applied to a resource's endpoints unless the endpoint
	// sets its own authorization.
	Authorization *config.Authorization `alloy:"authorization,block,optional"`

	// Relabelings are prepended to the relabel rules of the resource's
	// endpoints.
	Relabelings []*alloy_relabel.Config `alloy:"relabel_rule,block,optional"`

	// MetricRelabelings are appended to the metric relabel rules of the
	// resource's endpoints.
	MetricRelabelings []*alloy_relabel.Config `alloy:"metric_relabel_rule,block,optional"`

	// AttachMetadata is applied to a resource unless the resource sets its own
	// attach metadata configuration.
	AttachMetadata *AttachMetadataConfig `alloy:"attach_metadata,block,optional"`
}

// AttachMetadataConfig mirrors the Prometheus Operator AttachMetadata type.
type AttachMetadataConfig struct {
	// Node controls whether node metadata is attached to discovered targets.
	Node bool `alloy:"node,attr,optional"`
}

// ScrapeOptions holds values that configure scraping behavior.
type ScrapeOptions struct {
	// DefaultScrapeInterval is the default interval to scrape targets.
	DefaultScrapeInterval time.Duration `alloy:"default_scrape_interval,attr,optional"`

	// DefaultScrapeTimeout is the default timeout to scrape targets.
	DefaultScrapeTimeout time.Duration `alloy:"default_scrape_timeout,attr,optional"`

	// DefaultSampleLimit is the default sample limit per scrape.
	DefaultSampleLimit uint `alloy:"default_sample_limit,attr,optional"`

	// ScrapeNativeHistograms enables scraping of Prometheus native histograms.
	ScrapeNativeHistograms bool `alloy:"scrape_native_histograms,attr,optional"`

	// HonorMetadata controls whether metric metadata should be passed to downstream components.
	HonorMetadata bool `alloy:"honor_metadata,attr,optional"`

	// EnableTypeAndUnitLabels controls whether the metric's type and unit should be added as labels.
	EnableTypeAndUnitLabels bool `alloy:"enable_type_and_unit_labels,attr,optional"`

	// StartTimestampZeroIngestion controls whether the start timestamp is parsed from the
	// scraped metrics and injected as a synthetic zero sample, marking a counter reset.
	StartTimestampZeroIngestion bool `alloy:"start_timestamp_zero_ingestion,attr,optional"`
}

func (s *ScrapeOptions) GlobalConfig() promconfig.GlobalConfig {
	cfg := promconfig.DefaultGlobalConfig
	cfg.ScrapeInterval = model.Duration(s.DefaultScrapeInterval)
	cfg.ScrapeTimeout = model.Duration(s.DefaultScrapeTimeout)
	cfg.SampleLimit = s.DefaultSampleLimit
	// TODO: add support for choosing validation scheme: https://github.com/grafana/alloy/issues/4122
	cfg.MetricNameValidationScheme = model.LegacyValidation
	cfg.MetricNameEscapingScheme = model.EscapeUnderscores
	return cfg
}

var DefaultArguments = Arguments{
	Client: kubernetes.ClientArguments{
		HTTPClientConfig: config.DefaultHTTPClientConfig,
	},
	KubernetesRole:      string(promk8s.RoleEndpoint),
	InformerSyncTimeout: time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if len(args.Namespaces) == 0 {
		args.Namespaces = []string{apiv1.NamespaceAll}
	}
	if args.KubernetesRole != string(promk8s.RoleEndpointSlice) && args.KubernetesRole != string(promk8s.RoleEndpoint) {
		return fmt.Errorf("only endpoints and endpointslice are supported")
	}
	if err := validateScrapeClasses(args.ScrapeClasses); err != nil {
		return err
	}
	return nil
}

func validateScrapeClasses(classes []ScrapeClass) error {
	seen := make(map[string]struct{}, len(classes))
	defaultSet := false
	for _, sc := range classes {
		if sc.Name == "" {
			return fmt.Errorf("scrape_class name must not be empty")
		}
		if _, ok := seen[sc.Name]; ok {
			return fmt.Errorf("found multiple scrape_class blocks with name %q", sc.Name)
		}
		seen[sc.Name] = struct{}{}
		if sc.Default {
			if defaultSet {
				return fmt.Errorf("only one scrape_class may be marked as default")
			}
			defaultSet = true
		}
		if sc.TLSConfig != nil {
			if err := sc.TLSConfig.Validate(); err != nil {
				return fmt.Errorf("scrape_class %q: %w", sc.Name, err)
			}
		}
		if sc.Authorization != nil {
			if err := sc.Authorization.Validate(); err != nil {
				return fmt.Errorf("scrape_class %q: %w", sc.Name, err)
			}
		}
	}
	return nil
}

type DebugInfo struct {
	DiscoveredCRDs []*DiscoveredResource `alloy:"crds,block"`
	Targets        []scrape.TargetStatus `alloy:"targets,block,optional"`
}

type DiscoveredResource struct {
	Namespace        string    `alloy:"namespace,attr"`
	Name             string    `alloy:"name,attr"`
	LastReconcile    time.Time `alloy:"last_reconcile,attr,optional"`
	ReconcileError   string    `alloy:"reconcile_error,attr,optional"`
	ScrapeConfigsURL string    `alloy:"scrape_configs_url,attr,optional"`
}

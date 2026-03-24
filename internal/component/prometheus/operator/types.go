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

	InformerSyncTimeout time.Duration `alloy:"informer_sync_timeout,attr,optional"`
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

package integrations

import (
	"github.com/grafana/alloy/internal/static/integrations/v2/autoscrape"
	"github.com/grafana/alloy/internal/static/metrics"
)

const (
	// IntegrationsSDEndpoint is the API endpoint where the integration HTTP SD
	// API is exposed. The API uses query parameters to customize what gets
	// returned by discovery.
	IntegrationsSDEndpoint = "/agent/api/v1/metrics/integrations/sd"

	// IntegrationsAutoscrapeTargetsEndpoint is the API endpoint where autoscrape
	// integrations targets are exposed.
	IntegrationsAutoscrapeTargetsEndpoint = "/agent/api/v1/metrics/integrations/targets"
)

// DefaultSubsystemOptions holds the default settings for a Controller.
var (
	DefaultSubsystemOptions = SubsystemOptions{
		Metrics: DefaultMetricsSubsystemOptions,
	}

	DefaultMetricsSubsystemOptions = MetricsSubsystemOptions{
		Autoscrape: autoscrape.DefaultGlobal,
	}
)

// SubsystemOptions controls how the integrations subsystem behaves.
type SubsystemOptions struct {
	Metrics MetricsSubsystemOptions `yaml:"metrics,omitempty"`

	// Configs are configurations of integration to create. Unmarshaled through
	// the custom UnmarshalYAML method of Controller.
	Configs Configs `yaml:"-"`
}

// MetricsSubsystemOptions controls how metrics integrations behave.
type MetricsSubsystemOptions struct {
	Autoscrape autoscrape.Global `yaml:"autoscrape,omitempty"`
}

// ApplyDefaults will apply defaults to o.
func (o *SubsystemOptions) ApplyDefaults(mcfg *metrics.Config) error {
	if o.Metrics.Autoscrape.ScrapeInterval == 0 {
		o.Metrics.Autoscrape.ScrapeInterval = mcfg.Global.Prometheus.ScrapeInterval
	}
	if o.Metrics.Autoscrape.ScrapeTimeout == 0 {
		o.Metrics.Autoscrape.ScrapeTimeout = mcfg.Global.Prometheus.ScrapeTimeout
	}

	return nil
}

// MarshalYAML implements yaml.Marshaler for SubsystemOptions. Integrations
// will be marshaled inline.
func (o SubsystemOptions) MarshalYAML() (any, error) {
	return MarshalYAML(o)
}

// UnmarshalYAML implements yaml.Unmarshaler for SubsystemOptions. Inline
// integrations will be unmarshaled into o.Configs.
func (o *SubsystemOptions) UnmarshalYAML(unmarshal func(any) error) error {
	*o = DefaultSubsystemOptions
	return UnmarshalYAML(o, unmarshal)
}

// Package autoscrape implements a scraper for integrations.
package autoscrape

import (
	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"
)

// DefaultGlobal holds default values for Global.
var DefaultGlobal = Global{
	Enable:          true,
	MetricsInstance: "default",
}

// Global holds default settings for metrics integrations that support
// autoscraping. Integrations may override their settings.
type Global struct {
	Enable          bool           `yaml:"enable,omitempty"`           // Whether self-scraping should be enabled.
	MetricsInstance string         `yaml:"metrics_instance,omitempty"` // Metrics instance name to send metrics to.
	ScrapeInterval  model.Duration `yaml:"scrape_interval,omitempty"`  // Self-scraping frequency.
	ScrapeTimeout   model.Duration `yaml:"scrape_timeout,omitempty"`   // Self-scraping timeout.
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (g *Global) UnmarshalYAML(f func(any) error) error {
	*g = DefaultGlobal
	type global Global
	return f((*global)(g))
}

// Config configure autoscrape for an individual integration. Override defaults.
type Config struct {
	Enable          *bool          `yaml:"enable,omitempty"`           // Whether self-scraping should be enabled.
	MetricsInstance string         `yaml:"metrics_instance,omitempty"` // Metrics instance name to send metrics to.
	ScrapeInterval  model.Duration `yaml:"scrape_interval,omitempty"`  // Self-scraping frequency.
	ScrapeTimeout   model.Duration `yaml:"scrape_timeout,omitempty"`   // Self-scraping timeout.

	RelabelConfigs       []*relabel.Config `yaml:"relabel_configs,omitempty"`        // Relabel the autoscrape job
	MetricRelabelConfigs []*relabel.Config `yaml:"metric_relabel_configs,omitempty"` // Relabel individual autoscrape metrics
}

// ScrapeConfig bind a Prometheus scrape config with an instance to send
// scraped metrics to.
type ScrapeConfig struct {
	Instance string
	Config   prom_config.ScrapeConfig
}

// Package config provides common configuration structs shared among
// implementations of integrations.Integration.
package config

import (
	"net/url"
	"time"

	"github.com/prometheus/prometheus/model/relabel"
)

// Common is a set of common options shared by all integrations. It should be
// utilised by an integration's config by inlining the common options:
//
//	type IntegrationConfig struct {
//	  Common config.Common `yaml:",inline"`
//	}
type Common struct {
	Enabled              bool              `yaml:"enabled,omitempty"`
	InstanceKey          *string           `yaml:"instance,omitempty"`
	ScrapeIntegration    *bool             `yaml:"scrape_integration,omitempty"`
	ScrapeInterval       time.Duration     `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout        time.Duration     `yaml:"scrape_timeout,omitempty"`
	RelabelConfigs       []*relabel.Config `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []*relabel.Config `yaml:"metric_relabel_configs,omitempty"`
	WALTruncateFrequency time.Duration     `yaml:"wal_truncate_frequency,omitempty"`
}

// ScrapeConfig is a subset of options used by integrations to inform how samples
// should be scraped. It is utilized by the integrations.Manager to define a full
// Prometheus-compatible ScrapeConfig.
type ScrapeConfig struct {
	// JobName should be a unique name indicating the collection of samples to be
	// scraped. It will be prepended by "integrations/" when used by the integrations
	// manager.
	JobName string

	// MetricsPath is the path relative to the integration where metrics are exposed.
	// It should match a route added to the router provided in Integration.RegisterRoutes.
	// The path will be prepended by "/integrations/<integration name>" when read by
	// the integrations manager.
	MetricsPath string

	// QueryParams is a set of query parameters, that if set, will be appended to
	// MetricsPath and used for scraping the integration's target.
	QueryParams url.Values
}

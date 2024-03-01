package common

import (
	"github.com/grafana/agent/internal/static/integrations/v2/autoscrape"
	"github.com/prometheus/prometheus/model/labels"
)

// MetricsConfig is a set of common options shared by metrics integrations. It
// should be utilised by an integration's config by inlining the common
// options:
//
//	type IntegrationConfig struct {
//	  Common common.MetricsConfig `yaml:",inline"`
//	}
type MetricsConfig struct {
	Autoscrape  autoscrape.Config `yaml:"autoscrape,omitempty"`
	InstanceKey *string           `yaml:"instance,omitempty"`
	ExtraLabels labels.Labels     `yaml:"extra_labels,omitempty"`
}

// ApplyDefaults applies defaults to mc.
func (mc *MetricsConfig) ApplyDefaults(g autoscrape.Global) {
	if mc.Autoscrape.Enable == nil {
		val := g.Enable
		mc.Autoscrape.Enable = &val
	}
	if mc.Autoscrape.MetricsInstance == "" {
		mc.Autoscrape.MetricsInstance = g.MetricsInstance
	}
	if mc.Autoscrape.ScrapeInterval == 0 {
		mc.Autoscrape.ScrapeInterval = g.ScrapeInterval
	}
	if mc.Autoscrape.ScrapeTimeout == 0 {
		mc.Autoscrape.ScrapeTimeout = g.ScrapeTimeout
	}
}

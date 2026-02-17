// Package apache_http embeds https://github.com/Lusitaniae/apache_exporter
package apache_http

import (
	"net/http"
	"net/url"

	ae "github.com/Lusitaniae/apache_exporter/collector"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// DefaultConfig holds the default settings for the apache_http integration
var DefaultConfig = Config{
	ApacheAddr:         "http://localhost/server-status?auto",
	ApacheHostOverride: "",
	ApacheInsecure:     false,
}

// Config controls the apache_http integration.
type Config struct {
	ApacheAddr         string               `yaml:"scrape_uri,omitempty"`
	ApacheHostOverride string               `yaml:"host_override,omitempty"`
	ApacheInsecure     bool                 `yaml:"insecure,omitempty"`
	Common             common.MetricsConfig `yaml:",inline"`
}

// ApplyDefaults applies the integration's default configuration.
func (c *Config) ApplyDefaults(globals integrations_v2.Globals) error {
	c.Common.ApplyDefaults(globals.SubsystemOpts.Metrics.Autoscrape)
	return nil
}

// Identifier returns a string that identifies the integration.
func (c *Config) Identifier(globals integrations_v2.Globals) (string, error) {
	if c.Common.InstanceKey != nil {
		return *c.Common.InstanceKey, nil
	}
	u, err := url.Parse(c.ApacheAddr)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration this config is for.
func (c *Config) Name() string {
	return "apache_http"
}

type apacheHandler struct {
	cfg *Config
	log log.Logger
}

// NewIntegration instantiates a new integrations.MetricsIntegration
// which will handle requests to the apache http integration.
func (c *Config) NewIntegration(logger log.Logger, globals integrations_v2.Globals) (integrations_v2.Integration, error) {
	ah := &apacheHandler{cfg: c, log: logger}
	h, err := ah.createHandler()
	if err != nil {
		return nil, err
	}

	return metricsutils.NewMetricsHandlerIntegration(logger, c, c.Common, globals, h)
}

func (ah *apacheHandler) createHandler() (http.HandlerFunc, error) {
	_, err := url.ParseRequestURI(ah.cfg.ApacheAddr)
	if err != nil {
		level.Error(ah.log).Log("msg", "scrape_uri is invalid", "err", err)
		return nil, err
	}

	aeExporter := ae.NewExporter(ah.log, &ae.Config{
		ScrapeURI:    ah.cfg.ApacheAddr,
		HostOverride: ah.cfg.ApacheHostOverride,
		Insecure:     ah.cfg.ApacheInsecure,
	})

	return func(w http.ResponseWriter, r *http.Request) {
		registry := prometheus.NewRegistry()
		registry.MustRegister(aeExporter)
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}, nil
}

func init() {
	integrations_v2.Register(&Config{}, integrations_v2.TypeMultiplex)
}

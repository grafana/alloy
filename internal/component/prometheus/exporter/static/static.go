package static

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.static",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "static"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

type Arguments struct {
	Text string `alloy:"text,attr"`
}

var DefaultArguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	var p expfmt.TextParser

	// Fixme perform additional validation that parser do not handle
	_, err := p.TextToMetricFamilies(strings.NewReader(a.Text))
	return err
}

func (a *Arguments) Convert() *Config {
	return &Config{a.Text}
}

var _ integrations.Config = (*Config)(nil)

type Config struct {
	text string
}

// InstanceKey implements integrations.Config.
func (c *Config) InstanceKey(agentKey string) (string, error) {
	return "static", nil
}

// Name implements integrations.Config.
func (c *Config) Name() string {
	return "static"
}

// NewIntegration implements integrations.Config.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return &Integration{cfg: *c, reg: prometheus.NewRegistry()}, nil
}

type Integration struct {
	cfg Config
	reg *prometheus.Registry
}

// MetricsHandler implements integrations.Integration.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	var p expfmt.TextParser
	mf, err := p.TextToMetricFamilies(strings.NewReader(i.cfg.text))
	// This should not happen because we have already validated that it is possible to parse it.
	if err != nil {
		return nil, err
	}

	return promhttp.HandlerFor(newStaticGatherer(mf), promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}), nil
}

// Run implements integrations.Integration.
func (i *Integration) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// ScrapeConfigs implements integrations.Integration.
func (i *Integration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     i.cfg.Name(),
		MetricsPath: "/metrics",
	}}
}

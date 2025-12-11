package static

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.static",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "static"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), opts.ID)
}

type Arguments struct {
	Text                       string `alloy:"text,attr"`
	MetricNameValidationScheme string `alloy:"metric_name_validation_scheme,attr,optional"`
}

var DefaultArguments = Arguments{
	MetricNameValidationScheme: model.LegacyValidation.String(),
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	validationScheme := model.LegacyValidation
	switch a.MetricNameValidationScheme {
	case model.LegacyValidation.String():
		validationScheme = model.LegacyValidation
	case model.UTF8Validation.String():
		validationScheme = model.UTF8Validation
	default:
		return fmt.Errorf("invalid metric_name_validation_scheme %q: must be either %q or %q", 
			a.MetricNameValidationScheme, model.LegacyValidation.String(), model.UTF8Validation.String())
	}

	p := expfmt.NewTextParser(validationScheme)
	_, err := p.TextToMetricFamilies(strings.NewReader(a.Text))
	if err != nil {
		return fmt.Errorf("failed to parse prom text: %w", err)
	}
	return nil
}

func (a *Arguments) Convert() *Config {
	return &Config{
		text:             a.Text,
		validationScheme: a.MetricNameValidationScheme,
	}
}

var _ integrations.Config = (*Config)(nil)

type Config struct {
	text             string
	validationScheme string
}

func (c *Config) InstanceKey(key string) (string, error) {
	return key, nil
}

func (c *Config) Name() string {
	return "static"
}

func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return &Integration{cfg: *c, reg: prometheus.NewRegistry()}, nil
}

type Integration struct {
	cfg Config
	reg *prometheus.Registry
}

func (i *Integration) MetricsHandler() (http.Handler, error) {
	validationScheme := model.LegacyValidation
	if i.cfg.validationScheme == model.UTF8Validation.String() {
		validationScheme = model.UTF8Validation
	}

	p := expfmt.NewTextParser(validationScheme)
	mf, err := p.TextToMetricFamilies(strings.NewReader(i.cfg.text))
	// This should not happen because we have already validated that it is possible to parse it.
	if err != nil {
		return nil, fmt.Errorf("failed to parse prom text: %w", err)
	}

	return promhttp.HandlerFor(newStaticGatherer(mf), promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}), nil
}

func (i *Integration) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (i *Integration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     i.cfg.Name(),
		MetricsPath: "/metrics",
	}}
}

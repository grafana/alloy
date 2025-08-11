package static

import (
	"context"
	"net/http"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/prometheus/client_golang/prometheus/promhttp"
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
	Metrics []MetricEnum `alloy:"metric,enum"`
}

type MetricEnum struct {
	Gauge   *Gauge   `alloy:"gauge,block,optional"`
	Counter *Counter `alloy:"counter,block,optional"`
}

type Counter struct {
	Name   string            `alloy:"name,attr"`
	Help   string            `alloy:"help,attr"`
	Value  float64           `alloy:"value,attr"`
	Labels map[string]string `alloy:"labels,attr"`
}

type Gauge struct {
	Name   string            `alloy:"name,attr"`
	Help   string            `alloy:"help,attr"`
	Value  float64           `alloy:"value,attr"`
	Labels map[string]string `alloy:"labels,attr"`
}

type Histogram struct {
	Name    string            `alloy:"name,attr"`
	Help    string            `alloy:"help,attr"`
	Buckets []float64         `alloy:"buckets,attr"`
	Values  []float64         `alloy:"values,attr"`
	Labels  map[string]string `alloy:"labels,attr"`
}

var DefaultArguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	// FIXME(kalleep): Validate name uniqueness acrrouss all static metrics
	return nil
}

func (a *Arguments) Convert() *Config {
	var (
		gauges   []Gauge
		counters []Counter
	)

	for _, m := range a.Metrics {
		if m.Gauge != nil {
			gauges = append(gauges, *m.Gauge)
		}
		if m.Counter != nil {
			counters = append(counters, *m.Counter)
		}
	}

	return &Config{
		Counters: counters,
	}
}

var _ integrations.Config = (*Config)(nil)

type Config struct {
	Gauges     []Gauge
	Counters   []Counter
	Histograms []Histogram
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
	register(i.reg, i.cfg.Counters, func(m Counter) prometheus.Collector {
		var counter = prometheus.NewCounter(prometheus.CounterOpts{
			Name:        m.Name,
			Help:        m.Help,
			ConstLabels: m.Labels,
		})

		counter.Add(m.Value)
		return counter
	})

	register(i.reg, i.cfg.Gauges, func(m Gauge) prometheus.Collector {
		var gauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        m.Name,
			Help:        m.Help,
			ConstLabels: m.Labels,
		})

		gauge.Set(m.Value)
		return gauge
	})

	register(i.reg, i.cfg.Histograms, func(m Histogram) prometheus.Collector {
		var histogram = prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:        m.Name,
			Help:        m.Help,
			Buckets:     m.Buckets,
			ConstLabels: m.Labels,
		})

		for _, v := range m.Values {
			histogram.Observe(v)
		}
		return histogram
	})

	return promhttp.HandlerFor(i.reg, promhttp.HandlerOpts{}), nil
}

func register[T any](reg prometheus.Registerer, metrics []T, creator func(m T) prometheus.Collector) {
	for _, m := range metrics {
		util.MustRegisterOrGet(reg, creator(m))
	}

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

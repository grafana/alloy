package cloudwatch_exporter

import (
	"context"
	"log/slog"
	"net/http"

	yace "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	yaceClients "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	yaceModel "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/alloy/internal/static/integrations/config"
)

type cachingFactory interface {
	yaceClients.Factory
	Refresh()
	Clear()
}

var _ cachingFactory = &yaceClients.CachingFactory{}

// exporter wraps YACE entrypoint around an Integration implementation
type exporter struct {
	name                 string
	logger               *slog.Logger
	cachingClientFactory cachingFactory
	scrapeConf           yaceModel.JobsConfig
	labelsSnakeCase      bool
}

// NewCloudwatchExporter creates a new YACE wrapper, that implements Integration
// FIXME(kalleep): we no longer need debug flag..
func NewCloudwatchExporter(name string, logger *slog.Logger, conf yaceModel.JobsConfig, fipsEnabled, labelsSnakeCase, _ bool) (*exporter, error) {
	var factory cachingFactory
	var err error

	factory, err = yaceClients.NewFactory(logger, conf, fipsEnabled)
	if err != nil {
		return nil, err
	}

	return &exporter{
		name:                 name,
		logger:               logger,
		cachingClientFactory: factory,
		scrapeConf:           conf,
		labelsSnakeCase:      labelsSnakeCase,
	}, nil
}

func (e *exporter) MetricsHandler() (http.Handler, error) {
	// Wrapping in a handler so in every execution, a new registry is created and yace's entrypoint called
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		e.logger.Debug("Running collect in cloudwatch_exporter")

		// since we have called refresh, we have loaded all the credentials
		// into the clients and it is now safe to call concurrently. Defer the
		// clearing, so we always clear credentials before the next scrape
		e.cachingClientFactory.Refresh()
		defer e.cachingClientFactory.Clear()

		reg := prometheus.NewRegistry()
		for _, metric := range yace.Metrics {
			if err := reg.Register(metric); err != nil {
				e.logger.Debug("Could not register cloudwatch api metric")
			}
		}
		err := yace.UpdateMetrics(
			context.Background(),
			e.logger,
			e.scrapeConf,
			reg,
			e.cachingClientFactory,
			yace.MetricsPerQuery(metricsPerQuery),
			yace.LabelsSnakeCase(e.labelsSnakeCase),
			yace.CloudWatchAPIConcurrency(cloudWatchConcurrency),
			yace.TaggingAPIConcurrency(tagConcurrency),
		)
		if err != nil {
			e.logger.Error("Error collecting cloudwatch metrics", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, req)
	})
	return h, nil
}

func (e *exporter) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     e.name,
		MetricsPath: "/metrics",
	}}
}

func (e *exporter) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

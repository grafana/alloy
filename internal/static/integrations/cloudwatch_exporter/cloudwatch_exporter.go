package cloudwatch_exporter

import (
	"context"
	"log/slog"
	"net/http"

	yaceModel "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"

	"github.com/go-kit/log"
	yace "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	yaceClients "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	yaceClientsV1 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/v1"
	yaceClientsV2 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations/config"
)

type cachingFactory interface {
	yaceClients.Factory
	Refresh()
	Clear()
}

var _ cachingFactory = &yaceClientsV2.CachingFactory{}

// exporter wraps YACE entrypoint around an Integration implementation
type exporter struct {
	name                 string
	logger               *slog.Logger
	cachingClientFactory cachingFactory
	scrapeConf           yaceModel.JobsConfig
	labelsSnakeCase      bool
}

// NewCloudwatchExporter creates a new YACE wrapper, that implements Integration
func NewCloudwatchExporter(name string, logger log.Logger, conf yaceModel.JobsConfig, fipsEnabled, labelsSnakeCase, debug, useAWSSDKVersionV2 bool) (*exporter, error) {
	var factory cachingFactory
	var err error

	l := slog.New(newSlogHandler(logging.NewSlogGoKitHandler(logger), debug))

	if useAWSSDKVersionV2 {
		factory, err = yaceClientsV2.NewFactory(l, conf, fipsEnabled)
	} else {
		factory = yaceClientsV1.NewFactory(l, conf, fipsEnabled)
	}

	if err != nil {
		return nil, err
	}

	return &exporter{
		name:                 name,
		logger:               l,
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

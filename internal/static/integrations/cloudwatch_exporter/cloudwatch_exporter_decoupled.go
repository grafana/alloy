package cloudwatch_exporter

import (
	"context"
	"net/http"
	"time"

	"github.com/go-kit/log"
	yace "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg"
	yaceClientsV1 "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/clients/v1"
	yaceClientsV2 "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/clients/v2"
	yaceModel "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/static/integrations/config"
)

// asyncExporter wraps YACE entrypoint around an Integration implementation
type asyncExporter struct {
	name                 string
	logger               yaceLoggerWrapper
	cachingClientFactory cachingFactory
	scrapeConf           yaceModel.JobsConfig
	registry             atomic.Pointer[prometheus.Registry]
	// scrapeInterval is the frequency in which a background go-routine collects new AWS metrics via YACE.
	scrapeInterval time.Duration
}

// NewDecoupledCloudwatchExporter creates a new YACE wrapper, that implements Integration. The decouple feature spawns a
// background go-routine to perform YACE metric collection allowing for a decoupled collection of AWS metrics from the
// ServerHandler.
func NewDecoupledCloudwatchExporter(name string, logger log.Logger, conf yaceModel.JobsConfig, scrapeInterval time.Duration, fipsEnabled, debug bool, useAWSSDKVersionV2 bool) (*asyncExporter, error) {
	loggerWrapper := yaceLoggerWrapper{
		debug: debug,
		log:   logger,
	}

	var factory cachingFactory
	var err error

	if useAWSSDKVersionV2 {
		factory, err = yaceClientsV2.NewFactory(loggerWrapper, conf, fipsEnabled)
	} else {
		factory = yaceClientsV1.NewFactory(loggerWrapper, conf, fipsEnabled)
	}

	if err != nil {
		return nil, err
	}

	return &asyncExporter{
		name:                 name,
		logger:               loggerWrapper,
		cachingClientFactory: factory,
		scrapeConf:           conf,
		registry:             atomic.Pointer[prometheus.Registry]{},
		scrapeInterval:       scrapeInterval,
	}, nil
}

func (e *asyncExporter) MetricsHandler() (http.Handler, error) {
	// Wrapping handler to have logging around handler
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		reg := e.registry.Load()
		if reg == nil {
			e.logger.Warn("cloudwatch_exporter prometheus metric registry is empty")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, req)
	})
	return h, nil
}

func (e *asyncExporter) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     e.name,
		MetricsPath: "/metrics",
	}}
}

func (e *asyncExporter) Run(ctx context.Context) error {
	ticker := time.NewTicker(e.scrapeInterval)
	defer ticker.Stop()
	for {
		e.scrape(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (e *asyncExporter) scrape(ctx context.Context) {
	e.logger.Debug("Running collect in cloudwatch_exporter")
	// since we have called refresh, we have loaded all the credentials
	// into the clients and it is now safe to call concurrently. Defer the
	// clearing, so we always clear credentials before the next scrape
	e.cachingClientFactory.Refresh()
	defer e.cachingClientFactory.Clear()

	reg := prometheus.NewRegistry()
	err := yace.UpdateMetrics(
		ctx,
		e.logger,
		e.scrapeConf,
		reg,
		e.cachingClientFactory,
		yace.MetricsPerQuery(metricsPerQuery),
		yace.LabelsSnakeCase(labelsSnakeCase),
		yace.CloudWatchAPIConcurrency(cloudWatchConcurrency),
		yace.TaggingAPIConcurrency(tagConcurrency),
	)
	if err != nil {
		e.logger.Error(err, "Error collecting cloudwatch metrics")
	}
	// always update the registry even on error, to ensure we don't expose stale metrics from the previous
	// registry
	e.registry.Store(reg)
}

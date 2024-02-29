package cloudwatch_exporter

import (
	"context"
	"net/http"

	"github.com/go-kit/log"
	yace "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg"
	yaceClients "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/clients"
	yaceClientsV1 "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/clients/v1"
	yaceConf "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/config"
	yaceLog "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/grafana/agent/internal/static/integrations/config"
)

type cachingFactory interface {
	yaceClients.Factory
	Refresh()
	Clear()
}

var _ cachingFactory = &yaceClientsV1.CachingFactory{}

// exporter wraps YACE entrypoint around an Integration implementation
type exporter struct {
	name                 string
	logger               yaceLoggerWrapper
	cachingClientFactory cachingFactory
	scrapeConf           yaceConf.ScrapeConf
}

// NewCloudwatchExporter creates a new YACE wrapper, that implements Integration
func NewCloudwatchExporter(name string, logger log.Logger, conf yaceConf.ScrapeConf, fipsEnabled, debug bool) *exporter {
	loggerWrapper := yaceLoggerWrapper{
		debug: debug,
		log:   logger,
	}
	return &exporter{
		name:                 name,
		logger:               loggerWrapper,
		cachingClientFactory: yaceClientsV1.NewFactory(conf, fipsEnabled, loggerWrapper),
		scrapeConf:           conf,
	}
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
		err := yace.UpdateMetrics(
			context.Background(),
			e.logger,
			e.scrapeConf,
			reg,
			e.cachingClientFactory,
			yace.MetricsPerQuery(metricsPerQuery),
			yace.LabelsSnakeCase(labelsSnakeCase),
			yace.CloudWatchAPIConcurrency(cloudWatchConcurrency),
			yace.TaggingAPIConcurrency(tagConcurrency),
			// Enable max-dimension-associator feature flag
			// https://github.com/nerdswords/yet-another-cloudwatch-exporter/blob/master/docs/feature_flags.md#new-associator-algorithm
			yace.EnableFeatureFlag(yaceConf.MaxDimensionsAssociator),
		)
		if err != nil {
			e.logger.Error(err, "Error collecting cloudwatch metrics")
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

// yaceLoggerWrapper is wrapper implementation of yaceLog.Logger, based out of a log.Logger.
type yaceLoggerWrapper struct {
	log log.Logger

	// debug is just used for development purposes
	debug bool
}

func (l yaceLoggerWrapper) Info(message string, keyvals ...interface{}) {
	l.log.Log(append([]interface{}{"level", "info", "msg", message}, keyvals...)...)
}

func (l yaceLoggerWrapper) Debug(message string, keyvals ...interface{}) {
	if l.debug {
		l.log.Log(append([]interface{}{"level", "debug", "msg", message}, keyvals...)...)
	}
}

func (l yaceLoggerWrapper) Error(err error, message string, keyvals ...interface{}) {
	l.log.Log(append([]interface{}{"level", "error", "msg", message, "err", err}, keyvals...)...)
}

func (l yaceLoggerWrapper) Warn(message string, keyvals ...interface{}) {
	l.log.Log(append([]interface{}{"level", "warn", "msg", message}, keyvals...)...)
}

func (l yaceLoggerWrapper) With(keyvals ...interface{}) yaceLog.Logger {
	withLog := log.With(l.log, keyvals)
	return yaceLoggerWrapper{
		log: withLog,
	}
}

func (l yaceLoggerWrapper) IsDebugEnabled() bool {
	return l.debug
}

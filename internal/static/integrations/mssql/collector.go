package mssql

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/burningalchemist/sql_exporter"
	"github.com/burningalchemist/sql_exporter/config"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"gopkg.in/yaml.v3"
)

// Embedded config.CollectorConfig as yaml.
// We do this so that we can unmarshal instead of creating a static instance in code;
// This is because there is special unmarshal logic for the CollectorConfig
// that sets some unexported fields.
//
//go:embed collector_config.yaml
var collectorConfigBytes []byte

// collectorConfig is a config that can be used to construct a
// sql_exporter.Target that scrapes metrics from an instance of mssql
var collectorConfig config.CollectorConfig

// initialize static collector config
func init() {
	err := yaml.Unmarshal(collectorConfigBytes, &collectorConfig)
	if err != nil {
		panic(fmt.Errorf("failed to unmarshal mssql integration collector config: %w", err))
	}
}

// targetCollectorAdapter adapts sql_exporter.Target to prometheus.Collector
type targetCollectorAdapter struct {
	target sql_exporter.Target
	logger log.Logger
}

// newTargetCollectorAdapter creates a new TargetCollectorAdapter
func newTargetCollectorAdapter(t sql_exporter.Target, l log.Logger) targetCollectorAdapter {
	return targetCollectorAdapter{
		target: t,
		logger: l,
	}
}

// Collect calls the collect function of the underlying sql_exporter.Target, converting each
// returned sql_exporter.Metric to a prometheus.Metric.
func (t targetCollectorAdapter) Collect(m chan<- prometheus.Metric) {
	sqlMetrics := make(chan sql_exporter.Metric)

	go func() {
		t.target.Collect(context.Background(), sqlMetrics)
		close(sqlMetrics)
	}()

	for metric := range sqlMetrics {
		m <- sqlPrometheusMetricAdapter{
			Metric: metric,
			logger: t.logger,
		}
	}
}

// Describe is an empty method, which marks the prometheus.Collector as "unchecked"
func (t targetCollectorAdapter) Describe(chan<- *prometheus.Desc) {}

// sqlPrometheusMetricAdapter adapts sql_exporter.Metric to prometheus.Metric
type sqlPrometheusMetricAdapter struct {
	sql_exporter.Metric
	logger log.Logger
}

// Write writes the sql_exporter.Metric to the prometheus metric pb
func (s sqlPrometheusMetricAdapter) Write(m *dto.Metric) error {
	return s.Metric.Write(m)
}

// Desc converts the underlying sql_exporter.Metric description to
// a prometheus.Desc.
func (s sqlPrometheusMetricAdapter) Desc() *prometheus.Desc {
	sqlDesc := s.Metric.Desc()

	if sqlDesc == nil {
		// nil desc indicates that this metric has an invalid descriptor.
		// we can get the error using the Write function.
		err := s.Metric.Write(nil)
		level.Error(s.logger).Log("msg", "Invalid metric description.", "err", err)
		return prometheus.NewInvalidDesc(err)
	}

	constLabelsDtos := sqlDesc.ConstLabels()
	constLabels := make(map[string]string, len(constLabelsDtos))

	for _, l := range constLabelsDtos {
		constLabels[l.GetName()] = l.GetValue()
	}

	return prometheus.NewDesc(
		sqlDesc.Name(),
		sqlDesc.Help(),
		sqlDesc.Labels(),
		constLabels,
	)
}

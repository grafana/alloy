package http

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type translator struct {
	internal prometheus.Gatherer
}

var (
	activeSeries       = "prometheus_remote_write_wal_storage_active_series"
	deletedSeries      = "prometheus_remote_write_wal_storage_deleted_series"
	totalCreatedSeries = "prometheus_remote_write_wal_storage_created_series_total"
	appendedSamples    = "prometheus_remote_write_wal_samples_appended_total"
)

func (t translator) Gather() ([]*dto.MetricFamily, error) {
	metrics, err := t.internal.Gather()
	if err != nil {
		return nil, err
	}
	// To preserve backwards compatibility for metrics we need to ensure the name for metrics used on official dashboards
	// stays the same.
	for _, m := range metrics {
		switch *m.Name {
		case "prometheus_agent_active_series":
			m.Name = &activeSeries
		case "prometheus_agent_deleted_series":
			m.Name = &deletedSeries
		case "prometheus_agent_out_of_order_samples_total":
			m.Name = &totalCreatedSeries
		case "prometheus_agent_samples_appended_total":
			m.Name = &appendedSamples
		}
	}
	return metrics, nil
}

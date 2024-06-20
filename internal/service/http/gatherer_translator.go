package http

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type translator struct {
	internal prometheus.Gatherer
}

var (
	activeSeries  = "prometheus_remote_write_wal_storage_active_series"
	deletedSeries = "prometheus_remote_write_wal_storage_deleted_series"
)

func (t translator) Gather() ([]*dto.MetricFamily, error) {
	metrics, err := t.internal.Gather()
	if err != nil {
		return nil, err
	}
	for _, m := range metrics {
		switch *m.Name {
		case "prometheus_agent_active_series":
			m.Name = &activeSeries
		case "prometheus_agent_deleted_series":
			m.Name = &deletedSeries
		}
	}
	return metrics, nil
}

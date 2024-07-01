package http

import (
	"fmt"
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
			if *m.Type != dto.MetricType_GAUGE {
				return nil, fmt.Errorf("metric %s should be a gauge but is %s", *m.Name, dto.MetricType_name[int32(*m.Type)])
			}
			m.Name = &activeSeries
		case "prometheus_agent_deleted_series":
			if *m.Type != dto.MetricType_GAUGE {
				return nil, fmt.Errorf("metric %s should be a gauge but is %s", *m.Name, dto.MetricType_name[int32(*m.Type)])
			}
			m.Name = &deletedSeries
		case "prometheus_agent_out_of_order_samples_total":
			if *m.Type != dto.MetricType_COUNTER {
				return nil, fmt.Errorf("metric %s should be a counter but is %s", *m.Name, dto.MetricType_name[int32(*m.Type)])
			}
			m.Name = &totalCreatedSeries
		case "prometheus_agent_samples_appended_total":
			if *m.Type != dto.MetricType_COUNTER {
				return nil, fmt.Errorf("metric %s should be a counter but is %s", *m.Name, dto.MetricType_name[int32(*m.Type)])
			}
			m.Name = &appendedSamples
		}
	}
	return metrics, nil
}

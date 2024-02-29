package mssql

import (
	"context"
	"os"
	"testing"

	"github.com/burningalchemist/sql_exporter"
	"github.com/burningalchemist/sql_exporter/errors"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetCollectorAdapter_Collect(t *testing.T) {
	metricDesc := mockMetricDesc()
	target := mockTarget{
		metrics: []sql_exporter.Metric{
			sql_exporter.NewMetric(metricDesc, 1, "labelval1", "labelval2"),
		},
	}

	tca := newTargetCollectorAdapter(target, log.NewJSONLogger(os.Stdout))
	metricChan := make(chan prometheus.Metric, 1)
	tca.Collect(metricChan)

	metric := <-metricChan

	assert.Equal(t, prometheus.NewDesc(
		"metric_name",
		"help string",
		[]string{"label1", "label2"},
		map[string]string{
			"key": "val",
		},
	), metric.Desc())

	var dto io_prometheus_client.Metric
	require.NoError(t, metric.Write(&dto))
	assert.Equal(t, dto.Gauge.GetValue(), float64(1))
	assert.Equal(t, dto.Label, []*io_prometheus_client.LabelPair{
		{
			Name:  strp("key"),
			Value: strp("val"),
		},
		{
			Name:  strp("label1"),
			Value: strp("labelval1"),
		},
		{
			Name:  strp("label2"),
			Value: strp("labelval2"),
		},
	})
}

func TestSqlPrometheusMetricAdapter_Write(t *testing.T) {
	metricDesc := mockMetricDesc()
	metric := sqlPrometheusMetricAdapter{
		Metric: sql_exporter.NewMetric(metricDesc, 1, "labelval1", "labelval2"),
		logger: log.NewJSONLogger(os.Stdout),
	}

	var dto io_prometheus_client.Metric
	require.NoError(t, metric.Write(&dto))

	assert.Equal(t, dto.Gauge.GetValue(), float64(1))
	assert.Equal(t, dto.Label, []*io_prometheus_client.LabelPair{
		{
			Name:  strp("key"),
			Value: strp("val"),
		},
		{
			Name:  strp("label1"),
			Value: strp("labelval1"),
		},
		{
			Name:  strp("label2"),
			Value: strp("labelval2"),
		},
	})
}

func TestSqlPrometheusMetricAdapter_Desc(t *testing.T) {
	t.Run("AutomaticMetricDesc", func(t *testing.T) {
		metricDesc := mockMetricDesc()
		metric := sqlPrometheusMetricAdapter{
			Metric: sql_exporter.NewMetric(metricDesc, 1, "labelval1", "labelval2"),
			logger: log.NewJSONLogger(os.Stdout),
		}

		desc := metric.Desc()
		require.NotNil(t, desc)
		require.Equal(t, prometheus.NewDesc(
			"metric_name",
			"help string",
			[]string{"label1", "label2"},
			map[string]string{
				"key": "val",
			},
		), desc)
	})

	t.Run("InvalidMetricDesc", func(t *testing.T) {
		metricErr := errors.New("", "some error")
		metric := sqlPrometheusMetricAdapter{
			Metric: sql_exporter.NewInvalidMetric(metricErr),
			logger: log.NewJSONLogger(os.Stdout),
		}

		desc := metric.Desc()
		require.NotNil(t, desc)
		require.Equal(t, prometheus.NewInvalidDesc(
			metricErr,
		), desc)
	})
}

// helper function to create pointers to string literals
func strp(s string) *string {
	return &s
}

func mockMetricDesc() sql_exporter.MetricDesc {
	return sql_exporter.NewAutomaticMetricDesc(
		"",
		"metric_name",
		"help string",
		prometheus.GaugeValue,
		[]*io_prometheus_client.LabelPair{
			{
				Name:  strp("key"),
				Value: strp("val"),
			},
		},
		"label1", "label2",
	)
}

type mockTarget struct {
	metrics []sql_exporter.Metric
}

func (mt mockTarget) Collect(_ context.Context, ch chan<- sql_exporter.Metric) {
	for _, m := range mt.metrics {
		ch <- m
	}
}

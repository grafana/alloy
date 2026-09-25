package expose

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
)

func TestNew_ExportsReceiver(t *testing.T) {
	var exported Exports
	reg := prometheus.NewRegistry()

	comp, err := New(component.Options{
		ID:     "prometheus.expose.test",
		Logger: log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {
			exported = e.(Exports)
		},
		Registerer: reg,
	}, Arguments{})
	require.NoError(t, err)
	require.NotNil(t, comp)
	require.NotNil(t, exported.Receiver, "receiver export must be set")
}

func TestAppendMetric_AppearsInCollect(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := NewMetricsCollector("", "", nil)
	require.NoError(t, reg.Register(collector))

	lbls := labels.FromStrings(model.MetricNameLabel, "my_metric", "job", "test")
	collector.AppendMetric(lbls, time.Now().UnixMilli(), 42.0)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, mfs, 1)
	require.Equal(t, "my_metric", mfs[0].GetName())
	require.InDelta(t, 42.0, mfs[0].Metric[0].Gauge.GetValue(), 0.001)
}

func TestAppendMetric_GlobalLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := NewMetricsCollector("", "", map[string]string{"env": "prod"})
	require.NoError(t, reg.Register(collector))

	lbls := labels.FromStrings(model.MetricNameLabel, "my_metric", "job", "test")
	collector.AppendMetric(lbls, time.Now().UnixMilli(), 1.0)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, mfs, 1)

	metric := mfs[0].Metric[0]
	labelMap := make(map[string]string)
	for _, lp := range metric.Label {
		labelMap[lp.GetName()] = lp.GetValue()
	}
	require.Equal(t, "prod", labelMap["env"])
	require.Equal(t, "test", labelMap["job"])
}

func TestAppendMetric_NamespacedMetricName(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := NewMetricsCollector("ns", "sub", nil)
	require.NoError(t, reg.Register(collector))

	lbls := labels.FromStrings(model.MetricNameLabel, "my_metric")
	collector.AppendMetric(lbls, time.Now().UnixMilli(), 7.0)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, mfs, 1)
	require.Equal(t, "ns_sub_my_metric", mfs[0].GetName())
}

func TestAppendMetric_UpdatesExistingSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := NewMetricsCollector("", "", nil)
	require.NoError(t, reg.Register(collector))

	lbls := labels.FromStrings(model.MetricNameLabel, "my_metric")
	collector.AppendMetric(lbls, time.Now().UnixMilli(), 1.0)
	collector.AppendMetric(lbls, time.Now().UnixMilli(), 99.0)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, mfs[0].Metric, 1, "same series must not be duplicated")
	require.InDelta(t, 99.0, mfs[0].Metric[0].Gauge.GetValue(), 0.001)
}

func TestUpdate_RecreatesCollectorOnChange(t *testing.T) {
	reg := prometheus.NewRegistry()
	var exported Exports

	comp, err := New(component.Options{
		ID:     "prometheus.expose.test",
		Logger: log.NewNopLogger(),
		OnStateChange: func(e component.Exports) {
			exported = e.(Exports)
		},
		Registerer: reg,
	}, Arguments{Namespace: "old"})
	require.NoError(t, err)
	require.NotNil(t, exported.Receiver)

	// Override to isolated registry
	comp.registerer = reg

	err = comp.Update(Arguments{Namespace: "new"})
	require.NoError(t, err)
	require.Equal(t, "new", comp.args.Namespace)
}

func TestRun_UnregistersOnStop(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := NewMetricsCollector("", "", nil)
	require.NoError(t, reg.Register(collector))

	comp := &Component{
		collector:  collector,
		registerer: reg,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- comp.Run(ctx)
	}()

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	// After Run returns the collector should be unregistered; re-registering should succeed
	require.NoError(t, reg.Register(collector))
}

package mapping

import (
	"fmt"
	dto "github.com/prometheus/client_model/go"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestValidator(t *testing.T) {
	args := Arguments{}
	err := args.Validate()
	require.NoError(t, err)
}

func TestMapping(t *testing.T) {
	mapper := generateMapping(t)
	lbls := labels.FromStrings("source", "value1")
	newLbls := mapper.mapping(lbls)
	require.True(t, newLbls.Has("target"))
}

func TestMappingEmptySourceLabelValue(t *testing.T) {
	mapper := generateMapping(t)
	lbls := labels.FromStrings("source", "")
	newLbls := mapper.mapping(lbls)
	require.True(t, newLbls.Has("target"))
	require.Equal(t, newLbls.Get("target"), "empty")
}

func TestMappingEmptyTargetLabelValue(t *testing.T) {
	mapper := generateMapping(t)
	lbls := labels.FromStrings("source", "value2")
	newLbls := mapper.mapping(lbls)
	require.False(t, newLbls.Has("target"))
}

func TestMetrics(t *testing.T) {
	mapper := generateMapping(t)
	lbls := labels.FromStrings("__address__", "localhost")

	mapper.mapping(lbls)
	m := &dto.Metric{}
	err := mapper.metricsProcessed.Write(m)
	require.NoError(t, err)
	require.True(t, *(m.Counter.Value) == 1)
}

func generateMapping(t *testing.T) *Component {
	ls := labelstore.New(nil, prom.DefaultRegisterer)
	fanout := prometheus.NewInterceptor(nil, ls, prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
		require.True(t, l.Has("new_label"))
		return ref, nil
	}))
	mapper, err := New(component.Options{
		ID:             "1",
		Logger:         util.TestAlloyLogger(t),
		OnStateChange:  func(e component.Exports) {},
		Registerer:     prom.NewRegistry(),
		GetServiceData: getServiceData,
	}, Arguments{
		ForwardTo:   []storage.Appendable{fanout},
		SourceLabel: "source",
		LabelValuesMapping: map[string]map[string]string{
			"":       {"target": "empty"},
			"value1": {"target": "eulav"},
			"value2": {},
		},
	})
	require.NotNil(t, mapper)
	require.NoError(t, err)
	return mapper
}

func getServiceData(name string) (interface{}, error) {
	switch name {
	case labelstore.ServiceName:
		return labelstore.New(nil, prom.DefaultRegisterer), nil
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}

package relabel

import (
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestUpdateReset(t *testing.T) {
	relabeller := generateRelabel(t)
	lbls := labels.FromStrings("__address__", "localhost")
	relabeller.relabel(storage.SeriesRef(1), 0, lbls)
	require.True(t, relabeller.cache.Len() == 1)
	_ = relabeller.Update(Arguments{
		CacheSize:            100000,
		MetricRelabelConfigs: []*alloy_relabel.Config{},
	})
	require.True(t, relabeller.cache.Len() == 0)
}

func TestValidator(t *testing.T) {
	args := Arguments{CacheSize: 0}
	err := args.Validate()
	require.Error(t, err)

	args.CacheSize = 1
	err = args.Validate()
	require.NoError(t, err)
}

func TestNil(t *testing.T) {
	fanout := prometheus.NewInterceptor(nil, prometheus.WithAppendHook(func(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
		require.True(t, false)
		return ref, nil
	}))
	relabeller, err := New(component.Options{
		ID:             "1",
		Logger:         util.TestAlloyLogger(t),
		OnStateChange:  func(e component.Exports) {},
		Registerer:     prom.NewRegistry(),
		GetServiceData: getServiceData,
	}, Arguments{
		ForwardTo: []storage.Appendable{fanout},
		MetricRelabelConfigs: []*alloy_relabel.Config{
			{
				SourceLabels: []string{"__address__"},
				Regex:        alloy_relabel.Regexp(relabel.MustNewRegexp("(.+)")),
				Action:       "drop",
			},
		},
		CacheSize: 100000,
	})
	require.NotNil(t, relabeller)
	require.NoError(t, err)

	lbls := labels.FromStrings("__address__", "localhost")
	relabeller.relabel(storage.SeriesRef(1), 0, lbls)
}

func TestLRU(t *testing.T) {
	relabeller := generateRelabel(t)

	for i := 0; i < 600_000; i++ {
		lbls := labels.FromStrings("__address__", "localhost", "inc", strconv.Itoa(i))
		relabeller.relabel(storage.SeriesRef(i), 0, lbls)
	}
	require.Equal(t, 100_000, relabeller.cache.Len())
}

func TestLRUNaN(t *testing.T) {
	relabeller := generateRelabel(t)
	lbls := labels.FromStrings("__address__", "localhost")
	ref := storage.SeriesRef(1)
	relabeller.relabel(ref, 0, lbls)

	_, found := relabeller.getFromCache(ref)
	require.True(t, found)

	relabeller.relabel(ref, math.Float64frombits(value.StaleNaN), lbls)

	_, found = relabeller.getFromCache(ref)
	require.False(t, found)
}

func TestMetrics(t *testing.T) {
	relabeller := generateRelabel(t)
	lbls := labels.FromStrings("__address__", "localhost")

	relabeller.relabel(storage.SeriesRef(1), 0, lbls)
	m := &dto.Metric{}
	err := relabeller.metricsProcessed.Write(m)
	require.NoError(t, err)
	require.True(t, *(m.Counter.Value) == 1)
}

func BenchmarkCache(b *testing.B) {
	fanout := prometheus.NewInterceptor(nil, prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
		require.True(b, l.Has("new_label"))
		return ref, nil
	}))
	var entry storage.Appendable
	_, _ = New(component.Options{
		ID:     "1",
		Logger: util.TestAlloyLogger(b),
		OnStateChange: func(e component.Exports) {
			newE := e.(Exports)
			entry = newE.Receiver
		},
		Registerer: prom.NewRegistry(),
	}, Arguments{
		ForwardTo: []storage.Appendable{fanout},
		MetricRelabelConfigs: []*alloy_relabel.Config{
			{
				SourceLabels: []string{"__address__"},
				Regex:        alloy_relabel.Regexp(relabel.MustNewRegexp("(.+)")),
				TargetLabel:  "new_label",
				Replacement:  "new_value",
				Action:       "replace",
			},
		},
	})

	lbls := labels.FromStrings("__address__", "localhost")
	app := entry.Appender(b.Context())
	for i := 0; i < b.N; i++ {
		app.Append(0, lbls, time.Now().UnixMilli(), 0)
	}
	app.Commit()
}

func generateRelabel(t *testing.T) *Component {
	fanout := prometheus.NewInterceptor(nil)
	relabeller, err := New(component.Options{
		ID:             "1",
		Logger:         util.TestAlloyLogger(t),
		OnStateChange:  func(e component.Exports) {},
		Registerer:     prom.NewRegistry(),
		GetServiceData: getServiceData,
	}, Arguments{
		ForwardTo: []storage.Appendable{fanout},
		MetricRelabelConfigs: []*alloy_relabel.Config{
			{
				SourceLabels: []string{"__address__"},
				Regex:        alloy_relabel.Regexp(relabel.MustNewRegexp("(.+)")),
				TargetLabel:  "new_label",
				Replacement:  "new_value",
				Action:       "replace",
			},
		},
		CacheSize: 100_000,
	})
	require.NotNil(t, relabeller)
	require.NoError(t, err)
	return relabeller
}

func TestRuleGetter(t *testing.T) {
	// Set up the component Arguments.
	originalCfg := `rule {
         action       = "keep"
		 source_labels = ["__name__"]
         regex        = "up"
       }
		forward_to = []`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(originalCfg), &args))

	// Set up and start the component.
	tc, err := componenttest.NewControllerFromID(nil, "prometheus.relabel")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args)
		require.NoError(t, err)
	}()
	require.NoError(t, tc.WaitExports(time.Second))

	// Use the getter to retrieve the original relabeling rules.
	exports := tc.Exports().(Exports)
	gotOriginal := exports.Rules

	// Update the component with new relabeling rules and retrieve them.
	updatedCfg := `rule {
         action       = "drop"
		 source_labels = ["__name__"]
         regex        = "up"
       }
		forward_to = []`
	require.NoError(t, syntax.Unmarshal([]byte(updatedCfg), &args))

	require.NoError(t, tc.Update(args))
	exports = tc.Exports().(Exports)
	gotUpdated := exports.Rules

	require.NotEqual(t, gotOriginal, gotUpdated)
	require.Len(t, gotOriginal, 1)
	require.Len(t, gotUpdated, 1)

	require.Equal(t, gotOriginal[0].Action, alloy_relabel.Keep)
	require.Equal(t, gotUpdated[0].Action, alloy_relabel.Drop)
	require.Equal(t, gotUpdated[0].SourceLabels, gotOriginal[0].SourceLabels)
	require.Equal(t, gotUpdated[0].Regex, gotOriginal[0].Regex)
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

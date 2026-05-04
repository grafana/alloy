package relabel

import (
	"fmt"
	"math"
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
	relabeller.relabel(0, lbls)
	require.True(t, relabeller.cache.Len() == 1)
	require.NoError(t, relabeller.Update(Arguments{
		CacheSize:            100000,
		MetricRelabelConfigs: []*alloy_relabel.Config{},
	}))
	require.True(t, relabeller.cache.Len() == 0)
}

func TestValidator(t *testing.T) {
	cases := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{name: "default LRU", args: Arguments{CacheSize: 1}},
		{name: "TTL only", args: Arguments{CacheSize: 0, CacheTTL: 5 * time.Minute}},
		{name: "negative size", args: Arguments{CacheSize: -1}, wantErr: "max_cache_size must be >= 0"},
		{name: "negative ttl", args: Arguments{CacheSize: 1, CacheTTL: -time.Second}, wantErr: "cache_ttl must be >= 0"},
		{name: "both set", args: Arguments{CacheSize: 100, CacheTTL: time.Minute}, wantErr: "mutually exclusive"},
		{name: "neither set", args: Arguments{}, wantErr: "one of max_cache_size or cache_ttl must be set"},
		{name: "ttl too short", args: Arguments{CacheSize: 0, CacheTTL: 30 * time.Second}, wantErr: "at least"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.args.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
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
	relabeller.relabel(0, lbls)
}

// TestUpdateSwitchesCacheMode confirms that toggling cache_ttl on or
// off via Update() swaps the cache implementation.
func TestUpdateSwitchesCacheMode(t *testing.T) {
	relabeller := generateRelabelWithArgs(t, Arguments{CacheSize: 1_000})
	require.IsType(t, &lruRelabelCache{}, relabeller.cache)

	require.NoError(t, relabeller.Update(Arguments{CacheSize: 0, CacheTTL: time.Minute}))
	require.IsType(t, &ttlRelabelCache{}, relabeller.cache)

	require.NoError(t, relabeller.Update(Arguments{CacheSize: 1_000}))
	require.IsType(t, &lruRelabelCache{}, relabeller.cache)
}

// TestStaleNaNRemovesFromCache asserts that the relabel hook drops a
// cached entry when it sees a stale-NaN sample, regardless of cache
// mode (the cache itself is exercised via the component's interface).
func TestStaleNaNRemovesFromCache(t *testing.T) {
	relabeller := generateRelabel(t)
	lbls := labels.FromStrings("__address__", "localhost")
	relabeled := relabeller.relabel(0, lbls)

	require.NotEqual(t, lbls, relabeled)

	_, found := relabeller.getFromCache(lbls)
	require.True(t, found)

	relabeller.relabel(math.Float64frombits(value.StaleNaN), lbls)

	_, found = relabeller.getFromCache(lbls)
	require.False(t, found)
}

// TestCacheSizeMetric verifies the cache_size gauge reports the live
// cache length when scraped, in each mode.
func TestCacheSizeMetric(t *testing.T) {
	cases := []struct {
		name string
		args Arguments
		want float64
	}{
		{name: "lru", args: Arguments{CacheSize: 100}, want: 1},
		{name: "ttl", args: Arguments{CacheSize: 0, CacheTTL: time.Hour}, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := prom.NewRegistry()
			fanout := prometheus.NewInterceptor(nil)
			args := tc.args
			args.ForwardTo = []storage.Appendable{fanout}
			args.MetricRelabelConfigs = []*alloy_relabel.Config{
				{
					SourceLabels: []string{"__address__"},
					Regex:        alloy_relabel.Regexp(relabel.MustNewRegexp("(.+)")),
					TargetLabel:  "new_label",
					Replacement:  "new_value",
					Action:       "replace",
				},
			}
			relabeller, err := New(component.Options{
				ID:             "1",
				Logger:         util.TestAlloyLogger(t),
				OnStateChange:  func(e component.Exports) {},
				Registerer:     reg,
				GetServiceData: getServiceData,
			}, args)
			require.NoError(t, err)
			t.Cleanup(func() {
				relabeller.mut.RLock()
				relabeller.cache.Close()
				relabeller.mut.RUnlock()
			})

			relabeller.relabel(0, labels.FromStrings("__address__", "localhost"))

			mfs, err := reg.Gather()
			require.NoError(t, err)
			var got float64
			for _, mf := range mfs {
				if mf.GetName() == "alloy_prometheus_relabel_cache_size" {
					got = mf.Metric[0].Gauge.GetValue()
					break
				}
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMetrics(t *testing.T) {
	relabeller := generateRelabel(t)
	lbls := labels.FromStrings("__address__", "localhost")

	relabeller.relabel(0, lbls)
	m := &dto.Metric{}
	err := relabeller.metricsProcessed.Write(m)
	require.NoError(t, err)
	require.True(t, *(m.Counter.Value) == 1)
}

func BenchmarkCacheParallel(b *testing.B) {
	fanout := prometheus.NewInterceptor(nil, prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
		return ref, nil
	}))
	var entry storage.Appendable
	_, err := New(component.Options{
		ID:     "1",
		Logger: util.TestAlloyLogger(b),
		OnStateChange: func(e component.Exports) {
			newE := e.(Exports)
			entry = newE.Receiver
		},
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
	require.NoError(b, err)

	lbls := labels.FromStrings("__address__", "localhost")
	b.RunParallel(func(pb *testing.PB) {
		app := entry.Appender(b.Context())
		for pb.Next() {
			app.Append(0, lbls, time.Now().UnixMilli(), 0)
		}
		app.Commit()
	})
}

func BenchmarkCache(b *testing.B) {
	fanout := prometheus.NewInterceptor(nil, prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
		require.True(b, l.Has("new_label"))
		return ref, nil
	}))
	var entry storage.Appendable
	_, err := New(component.Options{
		ID:     "1",
		Logger: util.TestAlloyLogger(b),
		OnStateChange: func(e component.Exports) {
			newE := e.(Exports)
			entry = newE.Receiver
		},
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
	require.NoError(b, err)

	lbls := labels.FromStrings("__address__", "localhost")
	app := entry.Appender(b.Context())
	for i := 0; i < b.N; i++ {
		app.Append(0, lbls, time.Now().UnixMilli(), 0)
	}
	app.Commit()
}

// BenchmarkCacheModes exercises the relabel hot path under each cache
// mode at steady state (single label set; every call hits the cache
// after the first). Use to compare per-call overhead between modes.
func BenchmarkCacheModes(b *testing.B) {
	cases := []struct {
		name string
		args Arguments
	}{
		{name: "lru", args: Arguments{CacheSize: 100_000}},
		{name: "ttl", args: Arguments{CacheSize: 0, CacheTTL: 10 * time.Minute}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			fanout := prometheus.NewInterceptor(nil, prometheus.WithAppendHook(func(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64, _ storage.Appender) (storage.SeriesRef, error) {
				return ref, nil
			}))
			args := tc.args
			args.ForwardTo = []storage.Appendable{fanout}
			args.MetricRelabelConfigs = []*alloy_relabel.Config{
				{
					SourceLabels: []string{"__address__"},
					Regex:        alloy_relabel.Regexp(relabel.MustNewRegexp("(.+)")),
					TargetLabel:  "new_label",
					Replacement:  "new_value",
					Action:       "replace",
				},
			}
			var entry storage.Appendable
			_, err := New(component.Options{
				ID:     "1",
				Logger: util.TestAlloyLogger(b),
				OnStateChange: func(e component.Exports) {
					entry = e.(Exports).Receiver
				},
				Registerer:     prom.NewRegistry(),
				GetServiceData: getServiceData,
			}, args)
			require.NoError(b, err)

			lbls := labels.FromStrings("__address__", "localhost")
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				app := entry.Appender(b.Context())
				for pb.Next() {
					app.Append(0, lbls, time.Now().UnixMilli(), 0)
				}
				app.Commit()
			})
		})
	}
}

func generateRelabel(t *testing.T) *Component {
	return generateRelabelWithArgs(t, Arguments{CacheSize: 100_000})
}

func generateRelabelWithArgs(t *testing.T, args Arguments) *Component {
	fanout := prometheus.NewInterceptor(nil)
	args.ForwardTo = []storage.Appendable{fanout}
	args.MetricRelabelConfigs = []*alloy_relabel.Config{
		{
			SourceLabels: []string{"__address__"},
			Regex:        alloy_relabel.Regexp(relabel.MustNewRegexp("(.+)")),
			TargetLabel:  "new_label",
			Replacement:  "new_value",
			Action:       "replace",
		},
	}
	relabeller, err := New(component.Options{
		ID:             "1",
		Logger:         util.TestAlloyLogger(t),
		OnStateChange:  func(e component.Exports) {},
		Registerer:     prom.NewRegistry(),
		GetServiceData: getServiceData,
	}, args)
	require.NotNil(t, relabeller)
	require.NoError(t, err)
	t.Cleanup(func() {
		relabeller.mut.RLock()
		relabeller.cache.Close()
		relabeller.mut.RUnlock()
	})
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

func getServiceData(name string) (any, error) {
	switch name {
	case labelstore.ServiceName:
		return labelstore.New(nil, prom.DefaultRegisterer), nil
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}

// TestHashCollision demonstrates that the cache can return incorrect results when
// two different labelsets have the same hash. This is a known limitation of using
// hash as the cache key without collision detection.
func TestHashCollision(t *testing.T) {
	relabeller := generateRelabel(t)

	// These two series have the same XXHash; thanks to https://github.com/pstibrany/labels_hash_collisions
	ls1 := labels.FromStrings("__name__", "metric", "lbl", "HFnEaGl")
	ls2 := labels.FromStrings("__name__", "metric", "lbl", "RqcXatm")

	if ls1.Hash() != ls2.Hash() {
		// These ones are the same when using -tags slicelabels
		ls1 = labels.FromStrings("__name__", "metric", "lbl1", "value", "lbl2", "l6CQ5y")
		ls2 = labels.FromStrings("__name__", "metric", "lbl1", "value", "lbl2", "v7uDlF")
	}

	if ls1.Hash() != ls2.Hash() {
		t.Skip("Unable to find colliding label hashes for this labels implementation")
	}

	// Relabel the first labelset - this will cache the result
	relabeled1 := relabeller.relabel(0, ls1)
	require.NotEmpty(t, relabeled1)

	// Relabel the second labelset - due to hash collision, this will return
	// the cached result from ls1 instead of relabeling ls2
	relabeled2 := relabeller.relabel(0, ls2)
	require.NotEmpty(t, relabeled2)

	// This documents an inherited deficiency
	t.Log("Expected failure: hash collision causes cache to return wrong labels")
	require.True(t, labels.Equal(relabeled1, relabeled2),
		"Hash collision: different input labels produced same cached output. "+
			"ls1=%s, ls2=%s, relabeled1=%s, relabeled2=%s",
		ls1.String(), ls2.String(), relabeled1.String(), relabeled2.String())
}

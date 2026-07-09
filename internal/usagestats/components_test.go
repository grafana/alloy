package usagestats

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractOtelComponents(t *testing.T) {
	tests := map[string]struct {
		conf map[string]any
		want map[string][]string
	}{
		"nil conf": {
			conf: nil,
			want: map[string][]string{},
		},
		"empty conf": {
			conf: map[string]any{},
			want: map[string][]string{},
		},
		"groups by kind, collapses ids to types, keeps same type across kinds separate": {
			conf: map[string]any{
				"receivers": map[string]any{
					"otlp":              map[string]any{},
					"otlp/2":            map[string]any{}, // collapses to otlp; deduped within kind
					"prometheus/scrape": map[string]any{},
				},
				"processors": map[string]any{
					"batch": map[string]any{},
				},
				"exporters": map[string]any{
					"otlphttp":   map[string]any{},
					"debug":      map[string]any{},
					"otlp":       map[string]any{}, // otlp exporter stays distinct from otlp receiver
					"prometheus": map[string]any{},
				},
				"connectors": map[string]any{
					"forward": map[string]any{},
				},
				"extensions": map[string]any{
					"alloyengine":  map[string]any{},
					"health_check": map[string]any{},
				},
				// non-component sections are ignored
				"service": map[string]any{
					"pipelines": map[string]any{},
				},
			},
			want: map[string][]string{
				"receivers":  {"otlp", "prometheus"},
				"processors": {"batch"},
				"exporters":  {"debug", "otlp", "otlphttp", "prometheus"},
				"connectors": {"forward"},
				"extensions": {"alloyengine", "health_check"},
			},
		},
		"omits non-map and empty sections": {
			conf: map[string]any{
				"receivers":  "not-a-map",
				"processors": nil,
				"exporters": map[string]any{
					"otlp": map[string]any{},
				},
			},
			want: map[string][]string{
				"exporters": {"otlp"},
			},
		},
		"skips empty component ids": {
			conf: map[string]any{
				"receivers": map[string]any{
					"":     map[string]any{},
					"/foo": map[string]any{},
					"otlp": map[string]any{},
				},
			},
			want: map[string][]string{
				"receivers": {"otlp"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, ExtractOtelComponents(tc.conf))
		})
	}
}

func TestTrackerMetrics(t *testing.T) {
	t.Run("empty tracker reports nothing", func(t *testing.T) {
		tr := &Tracker{}
		require.Empty(t, tr.Metrics())
	})

	t.Run("default engine reports enabled-components only", func(t *testing.T) {
		tr := &Tracker{}
		tr.SetEnabledComponentsFunc(func() []string {
			return []string{"prometheus.scrape", "prometheus.remote_write"}
		})

		metrics := tr.Metrics()
		require.Equal(t, []string{"prometheus.scrape", "prometheus.remote_write"}, metrics[enabledComponentsMetric])
		require.NotContains(t, metrics, otelComponentsMetric)
		require.NotContains(t, metrics, alloyEngineComponentsMetric)
	})

	t.Run("otel engine reports otel components only when no alloyengine getter", func(t *testing.T) {
		tr := &Tracker{}
		byKind := map[string][]string{"receivers": {"otlp"}, "processors": {"batch"}}
		tr.SetOTelComponentsFunc(func() map[string][]string { return byKind })

		metrics := tr.Metrics()
		require.Equal(t, byKind, metrics[otelComponentsMetric])
		require.NotContains(t, metrics, enabledComponentsMetric)
		require.NotContains(t, metrics, alloyEngineComponentsMetric)
	})

	t.Run("otel engine includes alloyengine components when getter registered", func(t *testing.T) {
		tr := &Tracker{}
		byKind := map[string][]string{"receivers": {"otlp"}, "extensions": {"alloyengine"}}
		tr.SetOTelComponentsFunc(func() map[string][]string { return byKind })
		tr.SetAlloyEngineComponentsFunc(func() []string {
			return []string{"prometheus.scrape", "loki.write"}
		})

		metrics := tr.Metrics()
		require.Equal(t, byKind, metrics[otelComponentsMetric])
		require.Equal(t, []string{"prometheus.scrape", "loki.write"}, metrics[alloyEngineComponentsMetric])
		require.NotContains(t, metrics, enabledComponentsMetric)
	})
}

// TestTrackerRace exercises concurrent set/read to catch data races under -race.
func TestTrackerRace(t *testing.T) {
	tr := &Tracker{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			tr.SetEnabledComponentsFunc(func() []string { return []string{"prometheus.scrape"} })
		}()
		go func() {
			defer wg.Done()
			tr.SetOTelComponentsFunc(func() map[string][]string { return map[string][]string{"receivers": {"otlp"}} })
		}()
		go func() {
			defer wg.Done()
			tr.SetAlloyEngineComponentsFunc(func() []string { return []string{"loki.write"} })
		}()
		go func() { defer wg.Done(); _ = tr.Metrics() }()
	}
	wg.Wait()
}

package livedebugging

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateCollectorReturnsCountsFromRollingWindow(t *testing.T) {
	collector := newRateCollector()

	collector.record("prometheus.scrape.default", PrometheusMetric, 10, []string{"prometheus.remote_write.default"})
	collector.rotate(time.Unix(100, 0))
	collector.record("prometheus.scrape.default", PrometheusMetric, 5, []string{"prometheus.remote_write.default"})
	collector.rotate(time.Unix(101, 0))

	got, err := collector.snapshot(time.Unix(101, 500_000_000), 5)

	require.NoError(t, err)
	require.Equal(t, []Data{{
		ComponentID:        "prometheus.scrape.default",
		TargetComponentIDs: []string{"prometheus.remote_write.default"},
		Type:               PrometheusMetric,
		Count:              15,
	}}, got)
}

func TestRateCollectorExcludesBucketsOutsideRollingWindow(t *testing.T) {
	collector := newRateCollector()

	collector.record("prometheus.scrape.default", PrometheusMetric, 10, nil)
	collector.rotate(time.Unix(90, 0))
	collector.record("prometheus.scrape.default", PrometheusMetric, 5, nil)
	collector.rotate(time.Unix(100, 0))

	got, err := collector.snapshot(time.Unix(100, 500_000_000), 5)

	require.NoError(t, err)
	require.Equal(t, uint64(5), got[0].Count)
}

func TestRateCollectorKeepsDestinationCountsSeparate(t *testing.T) {
	collector := newRateCollector()

	collector.record("otelcol.receiver.otlp.default", OtelMetric, 7, []string{
		"otelcol.processor.batch.one",
		"otelcol.processor.batch.two",
	})
	collector.rotate(time.Unix(100, 0))

	got, err := collector.snapshot(time.Unix(100, 500_000_000), 5)

	require.NoError(t, err)
	require.ElementsMatch(t, []Data{
		{
			ComponentID:        "otelcol.receiver.otlp.default",
			TargetComponentIDs: []string{"otelcol.processor.batch.one"},
			Type:               OtelMetric,
			Count:              7,
		},
		{
			ComponentID:        "otelcol.receiver.otlp.default",
			TargetComponentIDs: []string{"otelcol.processor.batch.two"},
			Type:               OtelMetric,
			Count:              7,
		},
	}, got)
}

func TestRateCollectorRejectsUnsupportedWindow(t *testing.T) {
	collector := newRateCollector()

	_, err := collector.snapshot(time.Now(), 0)
	require.Error(t, err)

	_, err = collector.snapshot(time.Now(), 61)
	require.Error(t, err)
}

func TestLiveDebuggingExposesRecordedRateSnapshots(t *testing.T) {
	liveDebugging := NewLiveDebugging()
	now := time.Now()

	liveDebugging.RecordRate("prometheus.scrape.default", PrometheusMetric, 12, nil)
	liveDebugging.rates.rotate(now)

	got, err := liveDebugging.RateSnapshot(5)

	require.NoError(t, err)
	require.Equal(t, []Data{{
		ComponentID: "prometheus.scrape.default",
		Type:        PrometheusMetric,
		Count:       12,
	}}, got)
}

func TestPublishRecordsRateWithoutCallbacks(t *testing.T) {
	liveDebugging := NewLiveDebugging()
	now := time.Now()

	Publish(liveDebugging, NewData(
		"otelcol.receiver.otlp.default",
		OtelTrace,
		9,
		nil,
		WithTargetComponentIDs([]string{"otelcol.processor.batch.default"}),
	))
	liveDebugging.rates.rotate(now)

	got, err := liveDebugging.RateSnapshot(5)

	require.NoError(t, err)
	require.Equal(t, uint64(9), got[0].Count)
	require.Equal(t, []string{"otelcol.processor.batch.default"}, got[0].TargetComponentIDs)
}

func TestServiceContinuouslyRotatesRateSnapshots(t *testing.T) {
	service := New()
	service.rateSnapshotInterval = 10 * time.Millisecond
	liveDebugging := service.Data().(*liveDebugging)
	liveDebugging.RecordRate("prometheus.scrape.default", PrometheusMetric, 12, nil)

	ctx, cancel := context.WithCancel(t.Context())
	runDone := make(chan error, 1)
	go func() {
		runDone <- service.Run(ctx, nil)
	}()

	require.Eventually(t, func() bool {
		snapshot, err := liveDebugging.RateSnapshot(5)
		return err == nil && len(snapshot) == 1 && snapshot[0].Count == 12
	}, time.Second, 10*time.Millisecond)

	cancel()
	require.NoError(t, <-runDone)
}

func BenchmarkRateCollectorRecordExistingEdge(b *testing.B) {
	collector := newRateCollector()
	collector.record("prometheus.scrape.default", PrometheusMetric, 1, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		collector.record("prometheus.scrape.default", PrometheusMetric, 1, nil)
	}
}

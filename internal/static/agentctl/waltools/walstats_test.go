package waltools

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
	"github.com/stretchr/testify/require"
)

func TestWALStats(t *testing.T) {
	walDir := setupTestWAL(t)
	stats, err := CalculateStats(walDir)
	require.NoError(t, err)

	// Test From, To separately since comparing time.Time objects can be flaky
	require.Equal(t, int64(1), timestamp.FromTime(stats.From))
	require.Equal(t, int64(20), timestamp.FromTime(stats.To))

	require.Equal(t, WALStats{
		From:             stats.From,
		To:               stats.To,
		CheckpointNumber: 1,
		FirstSegment:     0,
		LastSegment:      3,
		HashCollisions:   1,
		InvalidRefs:      1,
		Targets: []WALTargetStats{{
			Instance: "test-instance",
			Job:      "test-job",
			Samples:  20,
			Series:   21,
		}},
	}, stats)
}

// setupTestWAL creates a test WAL with consistent sample data.
// The WAL will be deleted when the test finishes.
//
// The directory the WAL is in is returned.
func setupTestWAL(t *testing.T) string {
	l := log.NewNopLogger()

	walDir := t.TempDir()

	reg := prometheus.NewRegistry()
	w, err := wlog.NewSize(log.NewNopLogger(), reg, filepath.Join(walDir, "wal"), wlog.DefaultSegmentSize, wlog.CompressionSnappy)
	require.NoError(t, err)
	defer w.Close()

	// First, create a few series of 10 metrics. Each metric will have a
	// cardinality of 2, for a total of 20 series.
	var series []record.RefSeries
	addSeries := func(name string) {
		baseLabels := []string{"__name__", name, "job", "test-job", "instance", "test-instance"}
		labelsInitial := append(baseLabels, "initial", "yes")
		labelsNotInitial := append(baseLabels, "initial", "no")

		series = append(
			series,
			record.RefSeries{Ref: chunks.HeadSeriesRef(len(series)) + 1, Labels: labels.FromStrings(labelsInitial...)},
			record.RefSeries{Ref: chunks.HeadSeriesRef(len(series)) + 2, Labels: labels.FromStrings(labelsNotInitial...)},
		)
	}
	for i := 0; i < 10; i++ {
		addSeries(fmt.Sprintf("metric_%d", i))
	}
	// Force in a duplicate hash
	series = append(series, record.RefSeries{
		Ref:    99,
		Labels: labels.FromStrings("__name__", "metric_1", "job", "test-job", "instance", "test-instance", "initial", "yes"),
	})

	nextSegment := func(w *wlog.WL) error {
		_, err := w.NextSegment()
		return err
	}

	// Encode the samples to the WAL and create a new segment.
	var encoder record.Encoder
	buf := encoder.Series(series, nil)
	err = w.Log(buf)
	require.NoError(t, err)
	require.NoError(t, nextSegment(w))

	// Checkpoint the previous segment.
	_, err = wlog.Checkpoint(l, w, 0, 1, func(_ chunks.HeadSeriesRef) bool { return true }, 0)
	require.NoError(t, err)
	require.NoError(t, nextSegment(w))

	// Create some samples and then make a new segment.
	var samples []record.RefSample
	for i := 0; i < 20; i++ {
		samples = append(samples, record.RefSample{
			Ref: chunks.HeadSeriesRef(i + 1),
			T:   int64(i + 1),
			V:   1,
		})
	}
	// Force in an invalid ref
	samples = append(samples, record.RefSample{Ref: 404, T: 1, V: 1})

	buf = encoder.Samples(samples, nil)
	err = w.Log(buf)
	require.NoError(t, err)
	require.NoError(t, nextSegment(w))

	return w.Dir()
}

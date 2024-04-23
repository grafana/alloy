package queue

import (
	"testing"
	"time"

	log2 "github.com/go-kit/log"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestParquetSample(t *testing.T) {

	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)
	require.NoError(t, err)
	bb, err := l.serialize()
	require.NoError(t, err)

	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.Len(t, metrics[0].seriesLabels, 1)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
}

func TestParquetSampleMultiple(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)
	require.NoError(t, err)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"lbl":      "label_1",
	})

	err = l.AddMetric(lbls2, nil, ts, 11, nil, nil, tSample)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))
}

func TestParquetSampleMultipleDifferent(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
		"badlabel": "arrr",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)
	require.NoError(t, err)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test1",
		"lbl":      "label_1",
		"bob":      "foo",
	})

	err = l.AddMetric(lbls2, nil, ts, 11, nil, nil, tSample)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))
}

func TestParquetSampleTTL(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	time.Sleep(2 * time.Second)
	metrics, err := DeserializeParquet(bb, 1)
	require.NoError(t, err)
	require.Len(t, metrics, 0)
}

func TestParquetExemplar(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, exemplarLabels, ts, 10, nil, nil, tExemplar)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].sType == tExemplar)
	require.Len(t, metrics[0].seriesLabels, 1)
	require.Len(t, metrics[0].exemplarLabels, 1)

	require.True(t, metrics[0].seriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].seriesLabels[0].Value == "test")

	require.True(t, metrics[0].exemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].exemplarLabels[0].Value == "one")
}

func TestParquetMultipleExemplar(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})

	ts := time.Now().Unix()
	err := l.AddMetric(lbls, exemplarLabels, ts, 10, nil, nil, tExemplar)
	require.NoError(t, err)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"bob":      "arr",
	})
	exemplarLabels2 := labels.FromMap(map[string]string{
		"ex":  "one",
		"ex2": "two",
	})
	l.AddMetric(lbls2, exemplarLabels2, ts, 11, nil, nil, tExemplar)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)
	require.True(t, metrics[0].sType == tExemplar)
	require.Len(t, metrics[0].seriesLabels, 1)
	require.Len(t, metrics[0].exemplarLabels, 1)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabelsExemplar(exemplarLabels, metrics, ts, 10))

	require.True(t, hasLabel(lbls2, metrics, ts, 11))
	require.True(t, hasLabelsExemplar(exemplarLabels2, metrics, ts, 11))
}

func TestParquetExemplarNoTS(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	err := l.AddMetric(lbls, exemplarLabels, 0, 10, nil, nil, tExemplar)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].timestamp == 0)
	require.True(t, metrics[0].sType == tExemplar)
	require.Len(t, metrics[0].seriesLabels, 1)
	require.Len(t, metrics[0].exemplarLabels, 1)

	require.True(t, metrics[0].seriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].seriesLabels[0].Value == "test")

	require.True(t, metrics[0].exemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].exemplarLabels[0].Value == "one")
}

func TestParquetExemplarAndMetric(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	err := l.AddMetric(lbls, exemplarLabels, 0, 10, nil, nil, tExemplar)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)

	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].timestamp == 0)
	require.True(t, metrics[0].sType == tExemplar)
	require.Len(t, metrics[0].seriesLabels, 1)
	require.Len(t, metrics[0].exemplarLabels, 1)

	require.True(t, metrics[0].seriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].seriesLabels[0].Value == "test")

	require.True(t, metrics[0].exemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].exemplarLabels[0].Value == "one")
}

func TestParquetHistogram(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})

	// Note this histogram may not make logical sense, but the important thing is we get the same data in that we passed in.
	h := &histogram.Histogram{
		CounterResetHint: histogram.NotCounterReset,
		Schema:           2,
		ZeroThreshold:    1,
		ZeroCount:        1,
		Count:            10,
		Sum:              20,
		PositiveSpans: []histogram.Span{
			{
				Offset: 1,
				Length: 2,
			},
		},
		NegativeSpans: []histogram.Span{
			{
				Offset: 3,
				Length: 4,
			},
			{
				Offset: 5,
				Length: 6,
			},
		},
		PositiveBuckets: []int64{
			1,
			2,
			3,
		},
		NegativeBuckets: []int64{
			4,
			5,
			6,
		},
	}
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 0, h, nil, tHistogram)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)

	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	m := metrics[0]
	require.True(t, m.sType == tHistogram)
	require.Len(t, m.seriesLabels, 1)
	require.True(t, h.Equals(m.histogram))
}

func TestParquetFloatHistogram(t *testing.T) {
	l := newParquetWrite(fakeQueue{}, 16*1024*1024, 30*time.Second, log2.NewNopLogger())
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})

	// Note this histogram may not make logical sense, but the important thing is we get the same data in that we passed in.
	h := &histogram.FloatHistogram{
		CounterResetHint: histogram.NotCounterReset,
		Schema:           2,
		ZeroThreshold:    1.1,
		ZeroCount:        1.2,
		Count:            10.6,
		Sum:              20.5,
		PositiveSpans: []histogram.Span{
			{
				Offset: 1,
				Length: 2,
			},
		},
		NegativeSpans: []histogram.Span{
			{
				Offset: 3,
				Length: 4,
			},
			{
				Offset: 5,
				Length: 6,
			},
		},
		PositiveBuckets: []float64{
			1.1,
			2.2,
			3.3,
		},
		NegativeBuckets: []float64{
			4.4,
			5.5,
			6.6,
		},
	}
	ts := time.Now().Unix()
	err := l.AddMetric(lbls, nil, ts, 0, nil, h, tFloatHistogram)
	require.NoError(t, err)

	bb, err := l.serialize()
	require.NoError(t, err)
	metrics, err := DeserializeParquet(bb, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	m := metrics[0]
	require.True(t, m.sType == tFloatHistogram)
	require.Len(t, m.seriesLabels, 1)
	require.True(t, h.Equals(m.floatHistogram))
}

func hasLabel(lbls labels.Labels, metrics []TimeSeries, ts int64, val float64) bool {
	for _, m := range metrics {
		if labels.Compare(m.seriesLabels, lbls) == 0 {
			return ts == m.timestamp && val == m.value
		}
	}
	return false
}

func hasLabelsExemplar(lbls labels.Labels, metrics []TimeSeries, ts int64, val float64) bool {
	for _, m := range metrics {
		if labels.Compare(m.exemplarLabels, lbls) == 0 {
			return ts == m.timestamp && val == m.value
		}
	}
	return false
}

type fakeQueue struct{}

func (f fakeQueue) Add(data []byte) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (f fakeQueue) Next(enc []byte) ([]byte, string, bool, bool) {
	//TODO implement me
	panic("implement me")
}

func (f fakeQueue) Name() string {
	return "test"
}

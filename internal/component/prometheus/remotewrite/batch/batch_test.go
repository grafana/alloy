package batch

/*
import (
	"bytes"
	"github.com/prometheus/prometheus/model/histogram"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestSample(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)

	bb := newBuffer(nil)
	l.serialize(bb)
	out := newBuffer(bb.Bytes())
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.Len(t, metrics[0].SeriesLabels, 1)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
}

func TestSampleMultiple(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"lbl":      "label_1",
	})

	l.AddMetric(lbls2, nil, ts, 11, nil, nil, tSample)

	bb := newBuffer(nil)
	l.serialize(bb)
	out := newBuffer(bb.Bytes())
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))
}

func TestSampleMultipleDifferent(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
		"badlabel": "arrr",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test1",
		"lbl":      "label_1",
		"bob":      "foo",
	})

	l.AddMetric(lbls2, nil, ts, 11, nil, nil, tSample)

	bb := &buffer{
		Buffer:       &bytes.Buffer{},
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0, 1024),
		debug:        true,
	}
	l.serialize(bb)
	out := &buffer{
		Buffer:       bytes.NewBuffer(bb.Bytes()),
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0, 1024),
		debug:        true,
	}
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))
}

func TestSampleTTL(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, nil, nil, tSample)

	bb := newBuffer(nil)
	l.serialize(bb)
	out := newBuffer(bb.Bytes())
	time.Sleep(2 * time.Second)
	metrics, err := Deserialize(out, 1)
	ttl := &TTLError{}
	require.ErrorAs(t, err, ttl)
	require.Len(t, metrics, 0)
}

func TestExemplar(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, exemplarLabels, ts, 10, nil, nil, tExemplar)

	bb := newBuffer(nil)
	l.serialize(bb)
	out := newBuffer(bb.Bytes())
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].SeriesType == tExemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")

	require.True(t, metrics[0].ExemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].ExemplarLabels[0].Value == "one")
}

func TestMultipleExemplar(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})

	ts := time.Now().Unix()
	l.AddMetric(lbls, exemplarLabels, ts, 10, nil, nil, tExemplar)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"bob":      "arr",
	})
	exemplarLabels2 := labels.FromMap(map[string]string{
		"ex":  "one",
		"ex2": "two",
	})
	l.AddMetric(lbls2, exemplarLabels2, ts, 11, nil, nil, tExemplar)

	bb := newBuffer(nil)
	l.serialize(bb)
	out := newBuffer(bb.Bytes())
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)
	require.True(t, metrics[0].SeriesType == tExemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabelsExemplar(exemplarLabels, metrics, ts, 10))

	require.True(t, hasLabel(lbls2, metrics, ts, 11))
	require.True(t, hasLabelsExemplar(exemplarLabels2, metrics, ts, 11))
}

func TestExemplarNoTS(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	l.AddMetric(lbls, exemplarLabels, 0, 10, nil, nil, tExemplar)

	bb := &buffer{
		Buffer:       &bytes.Buffer{},
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0, 1024),
		debug:        true,
	}
	l.serialize(bb)
	out := &buffer{
		Buffer:       bytes.NewBuffer(bb.Bytes()),
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0, 1024),
		debug:        true,
	}
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].Timestamp == 0)
	require.True(t, metrics[0].SeriesType == tExemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")

	require.True(t, metrics[0].ExemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].ExemplarLabels[0].Value == "one")
}

func TestExemplarAndMetric(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	exemplarLabels := labels.FromMap(map[string]string{
		"ex": "one",
	})
	l.AddMetric(lbls, exemplarLabels, 0, 10, nil, nil, tExemplar)

	bb := &buffer{
		Buffer:       &bytes.Buffer{},
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0, 1024),
		debug:        true,
	}
	l.serialize(bb)
	out := &buffer{
		Buffer:       bytes.NewBuffer(bb.Bytes()),
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0, 1024),
		debug:        true,
	}
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].Timestamp == 0)
	require.True(t, metrics[0].SeriesType == tExemplar)
	require.Len(t, metrics[0].SeriesLabels, 1)
	require.Len(t, metrics[0].ExemplarLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")

	require.True(t, metrics[0].ExemplarLabels[0].Name == "ex")
	require.True(t, metrics[0].ExemplarLabels[0].Value == "one")
}

func TestHistogram(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
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
	l.AddMetric(lbls, nil, ts, 0, h, nil, tHistogram)

	bb := newBuffer(nil)
	l.serialize(bb)
	out := newBuffer(bb.Bytes())
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	m := metrics[0]
	require.True(t, m.SeriesType == tHistogram)
	require.Len(t, m.SeriesLabels, 1)
	require.True(t, h.Equals(m.Histogram))
}

func TestFloatHistogram(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
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
	l.AddMetric(lbls, nil, ts, 0, nil, h, tFloatHistogram)

	bb := newBuffer(nil)
	l.serialize(bb)
	out := newBuffer(bb.Bytes())
	metrics, err := Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	m := metrics[0]
	require.True(t, m.SeriesType == tFloatHistogram)
	require.Len(t, m.SeriesLabels, 1)
	require.True(t, h.Equals(m.FloatHistogram))
}

func hasLabel(lbls labels.Labels, metrics []*TimeSeries, ts int64, val float64) bool {
	for _, m := range metrics {
		if labels.Compare(m.SeriesLabels, lbls) == 0 {
			return ts == m.Timestamp && val == m.Value
		}
	}
	return false
}

func hasLabelsExemplar(lbls labels.Labels, metrics []*TimeSeries, ts int64, val float64) bool {
	for _, m := range metrics {
		if labels.Compare(m.ExemplarLabels, lbls) == 0 {
			return ts == m.Timestamp && val == m.Value
		}
	}
	return false
}

func newBuffer(bb []byte) *buffer {
	return &buffer{
		Buffer:       bytes.NewBuffer(bb),
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0, 1024),
		debug:        true,
	}
}
*/

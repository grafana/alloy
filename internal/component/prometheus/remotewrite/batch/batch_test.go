package batch

import (
	"bytes"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestLinear(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, tSample)

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
	require.Len(t, metrics[0].SeriesLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")
}

func TestLinearMultiple(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, tSample)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"lbl":      "label_1",
	})

	l.AddMetric(lbls2, nil, ts, 11, tSample)

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

func TestLinearMultipleDifferent(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
		"badlabel": "arrr",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, tSample)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test1",
		"lbl":      "label_1",
		"bob":      "foo",
	})

	l.AddMetric(lbls2, nil, ts, 11, tSample)

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

func TestLinearTTL(t *testing.T) {
	l := newBatch(nil, 16*1024*1024)

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, nil, ts, 10, tSample)

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
	time.Sleep(2 * time.Second)
	metrics, err := Deserialize(out, 1)
	ttl := &TTLError{}
	require.ErrorAs(t, err, ttl)
	require.Len(t, metrics, 0)
}

func hasLabel(lbls labels.Labels, metrics []*TimeSeries, ts int64, val float64) bool {
	for _, m := range metrics {
		if labels.Compare(m.SeriesLabels, lbls) == 0 {
			return ts == m.Timestamp && val == m.Value
		}
	}
	return false
}

package batch

import (
	"bytes"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestLinear(t *testing.T) {
	l := newBatch()
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, ts, 10)

	bb := &bytes.Buffer{}
	l.Serialize(bb)
	out := bytes.NewBuffer(bb.Bytes())
	metrics, err := l.Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.Len(t, metrics[0].SeriesLabels, 1)

	require.True(t, metrics[0].SeriesLabels[0].Name == "__name__")
	require.True(t, metrics[0].SeriesLabels[0].Value == "test")
}

func TestLinearMultiple(t *testing.T) {
	l := newBatch()
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, ts, 10)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"lbl":      "label_1",
	})

	l.AddMetric(lbls2, ts, 11)

	bb := &bytes.Buffer{}
	l.Serialize(bb)
	out := bytes.NewBuffer(bb.Bytes())
	metrics, err := l.Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))
}

func TestLinearReuse(t *testing.T) {
	l := LinearPool.Get().(*batch)
	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, ts, 10)

	lbls2 := labels.FromMap(map[string]string{
		"__name__": "test",
		"lbl":      "label_1",
	})
	l.AddMetric(lbls2, ts, 11)

	bb := &bytes.Buffer{}
	l.Serialize(bb)
	out := bytes.NewBuffer(bb.Bytes())
	metrics, err := l.Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 2)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
	require.True(t, hasLabel(lbls2, metrics, ts, 11))

	l.Reset()
	LinearPool.Put(l)

	l = LinearPool.Get().(*batch)
	l.AddMetric(lbls, ts, 10)
	bb = &bytes.Buffer{}
	l.Serialize(bb)
	out = bytes.NewBuffer(bb.Bytes())
	metrics, err = l.Deserialize(out, 100)
	require.NoError(t, err)
	require.Len(t, metrics, 1)

	require.True(t, hasLabel(lbls, metrics, ts, 10))
}

func TestLinearTTL(t *testing.T) {
	l := newBatch()

	lbls := labels.FromMap(map[string]string{
		"__name__": "test",
	})
	ts := time.Now().Unix()
	l.AddMetric(lbls, ts, 10)

	bb := &bytes.Buffer{}
	l.Serialize(bb)
	out := bytes.NewBuffer(bb.Bytes())
	time.Sleep(2 * time.Second)
	metrics, err := l.Deserialize(out, 1)
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

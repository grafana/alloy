package pyroscope

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func Test_FanOut(t *testing.T) {
	totalAppend := atomic.NewInt32(0)
	lbls := labels.New(
		labels.Label{Name: "foo", Value: "bar"},
	)
	f := NewFanout([]Appendable{
		AppendableFunc(func(_ context.Context, labels labels.Labels, _ []*RawSample) error {
			require.Equal(t, lbls, labels)
			totalAppend.Inc()
			return nil
		}),
		AppendableFunc(func(_ context.Context, labels labels.Labels, _ []*RawSample) error {
			require.Equal(t, lbls, labels)
			totalAppend.Inc()
			return nil
		}),
		AppendableFunc(func(_ context.Context, labels labels.Labels, _ []*RawSample) error {
			require.Equal(t, lbls, labels)
			totalAppend.Inc()
			return nil
		}),
	}, "foo", prometheus.NewRegistry())
	require.NoError(t, f.Appender().Append(t.Context(), lbls, []*RawSample{}))
	require.Equal(t, int32(3), totalAppend.Load())
	f.UpdateChildren([]Appendable{
		AppendableFunc(func(_ context.Context, labels labels.Labels, _ []*RawSample) error {
			require.Equal(t, lbls, labels)
			totalAppend.Inc()
			return errors.New("foo")
		}),
		AppendableFunc(func(_ context.Context, labels labels.Labels, _ []*RawSample) error {
			require.Equal(t, lbls, labels)
			totalAppend.Inc()
			return nil
		}),
	})
	totalAppend.Store(0)
	require.Error(t, f.Appender().Append(t.Context(), lbls, []*RawSample{}))
	require.Equal(t, int32(2), totalAppend.Load())
}

func Test_FanOut_AppendIngest(t *testing.T) {
	totalAppend := atomic.NewInt32(0)
	profile := &IncomingProfile{
		RawBody: []byte("test"),
		Labels:  labels.New(labels.Label{Name: "foo", Value: "bar"}),
	}

	f := NewFanout([]Appendable{
		AppendableIngestFunc(func(_ context.Context, p *IncomingProfile) error {
			require.Equal(t, profile.RawBody, p.RawBody)
			require.Equal(t, profile.Labels, p.Labels)
			totalAppend.Inc()
			return nil
		}),
		AppendableIngestFunc(func(_ context.Context, p *IncomingProfile) error {
			require.Equal(t, profile.RawBody, p.RawBody)
			require.Equal(t, profile.Labels, p.Labels)
			totalAppend.Inc()
			return nil
		}),
		AppendableIngestFunc(func(_ context.Context, p *IncomingProfile) error {
			require.Equal(t, profile.RawBody, p.RawBody)
			require.Equal(t, profile.Labels, p.Labels)
			totalAppend.Inc()
			return errors.New("foo")
		}),
	}, "foo", prometheus.NewRegistry())
	totalAppend.Store(0)
	require.Error(t, f.Appender().AppendIngest(t.Context(), profile))
	require.Equal(t, int32(3), totalAppend.Load())
	f.UpdateChildren([]Appendable{
		AppendableIngestFunc(func(_ context.Context, p *IncomingProfile) error {
			require.Equal(t, profile.RawBody, p.RawBody)
			require.Equal(t, profile.Labels, p.Labels)
			totalAppend.Inc()
			return nil
		}),
		AppendableIngestFunc(func(_ context.Context, p *IncomingProfile) error {
			require.Equal(t, profile.RawBody, p.RawBody)
			require.Equal(t, profile.Labels, p.Labels)
			totalAppend.Inc()
			return errors.New("bar")
		}),
	})
	totalAppend.Store(0)
	require.Error(t, f.Appender().AppendIngest(t.Context(), profile))
	require.Equal(t, int32(2), totalAppend.Load())
}

func TestFanoutBatch(t *testing.T) {
	series := []RawProfileSeries{
		{Labels: labels.FromStrings("service_name", "one"), Samples: []*RawSample{{RawProfile: []byte("one")}}},
		{Labels: labels.FromStrings("service_name", "two"), Samples: []*RawSample{{RawProfile: []byte("two")}}},
	}
	batchCalls, individualCalls := 0, 0
	batch := AppenderMock{AppendBatchFunc: func(_ context.Context, got []RawProfileSeries) error {
		batchCalls++
		require.Equal(t, series, got)
		return nil
	}}
	legacy := AppendableFunc(func(_ context.Context, lbs labels.Labels, samples []*RawSample) error {
		require.Equal(t, series[individualCalls].Labels, lbs)
		require.Same(t, series[individualCalls].Samples[0], samples[0])
		individualCalls++
		return nil
	})
	f := NewFanout([]Appendable{batch, legacy}, "test", prometheus.NewRegistry())
	require.NoError(t, AppendBatch(t.Context(), f.Appender(), series))
	require.Equal(t, 1, batchCalls)
	require.Equal(t, 2, individualCalls)
	require.NoError(t, AppendBatch(t.Context(), f.Appender(), nil))
	require.Equal(t, 1, batchCalls)
}

func TestFanoutBatchError(t *testing.T) {
	want := errors.New("failed batch")
	calls := 0
	f := NewFanout([]Appendable{
		AppenderMock{AppendBatchFunc: func(context.Context, []RawProfileSeries) error { return want }},
		AppenderMock{AppendBatchFunc: func(context.Context, []RawProfileSeries) error { calls++; return nil }},
	}, "test", prometheus.NewRegistry())
	err := AppendBatch(t.Context(), f.Appender(), []RawProfileSeries{{Labels: labels.FromStrings("service_name", "test")}})
	require.ErrorIs(t, err, want)
	require.Equal(t, 1, calls)
}

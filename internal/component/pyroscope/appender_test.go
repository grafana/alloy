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
	lbls := labels.Labels{
		{Name: "foo", Value: "bar"},
	}
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
	require.NoError(t, f.Appender().Append(context.Background(), lbls, []*RawSample{}))
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
	require.Error(t, f.Appender().Append(context.Background(), lbls, []*RawSample{}))
	require.Equal(t, int32(2), totalAppend.Load())
}

package prometheus_test

import (
	"testing"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus"
)

func TestInterceptorAppenderV2_AppendExemplars(t *testing.T) {
	lbls := labels.FromStrings("job", "test")
	exemplars := []exemplar.Exemplar{{Value: 1, Ts: 10, HasTs: true}}

	t.Run("passes exemplars to the next appender", func(t *testing.T) {
		child := &exemplarAppenderV2{}
		interceptor := prometheus.NewInterceptor(exemplarStore{child})

		ref, err := interceptor.AppenderV2(t.Context()).(storage.ExemplarAppenderV2).AppendExemplars(5, lbls, exemplars)
		require.NoError(t, err)
		require.Equal(t, storage.SeriesRef(5), ref)
		require.Equal(t, []storage.SeriesRef{5}, child.refs)
		require.Equal(t, [][]exemplar.Exemplar{exemplars}, child.exemplars)
	})

	t.Run("without a next appendable, exemplars are dropped", func(t *testing.T) {
		interceptor := prometheus.NewInterceptor(nil)

		_, err := interceptor.AppenderV2(t.Context()).(storage.ExemplarAppenderV2).AppendExemplars(5, lbls, exemplars)
		require.NoError(t, err)
	})
}

package convert

import (
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConverterGC(t *testing.T) {
	tt := []struct {
		name      string
		staleTime time.Duration
		expectGC  bool
	}{
		{name: "evicts stale entries", staleTime: 0, expectGC: true},
		{name: "retains fresh entries", staleTime: time.Hour, expectGC: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			conv := New(util.TestAlloyLogger(t).Slog(), testappender.ConstantAppendable{Inner: &testappender.Appender{}}, Options{HonorMetadata: true})

			metrics := pmetric.NewMetrics()
			gauge := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			gauge.SetName("test_metric")
			gauge.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1)
			require.NoError(t, conv.ConsumeMetrics(t.Context(), metrics))

			require.NotZero(t, syncMapLen(&conv.seriesCache))
			require.NotZero(t, syncMapLen(&conv.metadataCache))

			conv.GC(tc.staleTime)

			if tc.expectGC {
				require.Zero(t, syncMapLen(&conv.seriesCache))
				require.Zero(t, syncMapLen(&conv.metadataCache))
			} else {
				require.NotZero(t, syncMapLen(&conv.seriesCache))
				require.NotZero(t, syncMapLen(&conv.metadataCache))
			}
		})
	}
}

func syncMapLen(m *sync.Map) int {
	var n int
	m.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

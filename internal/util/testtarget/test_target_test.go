package testtarget

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util/assertmetrics"
)

func TestTargetHelper(t *testing.T) {
	tt := NewTestTarget()
	defer tt.Close()

	c := tt.AddCounter(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "A test counter",
	})
	c.Add(10)

	g := tt.AddGauge(prometheus.GaugeOpts{
		Name: "test_gauge",
		Help: "A test gauge",
	})
	g.Set(123)

	h := tt.AddHistogram(prometheus.HistogramOpts{
		Name: "test_histogram",
		Help: "A test histogram",
	})
	h.Observe(3)

	expected := `# HELP test_counter A test counter
# TYPE test_counter counter
test_counter 10
# HELP test_gauge A test gauge
# TYPE test_gauge gauge
test_gauge 123
# HELP test_histogram A test histogram
# TYPE test_histogram histogram
test_histogram_bucket{le="0.005"} 0
test_histogram_bucket{le="0.01"} 0
test_histogram_bucket{le="0.025"} 0
test_histogram_bucket{le="0.05"} 0
test_histogram_bucket{le="0.1"} 0
test_histogram_bucket{le="0.25"} 0
test_histogram_bucket{le="0.5"} 0
test_histogram_bucket{le="1"} 0
test_histogram_bucket{le="2.5"} 0
test_histogram_bucket{le="5"} 1
test_histogram_bucket{le="10"} 1
test_histogram_bucket{le="+Inf"} 1
test_histogram_sum 3
test_histogram_count 1
`

	actual := assertmetrics.ReadMetrics(t, tt.Registry())
	require.Equal(t, expected, actual)
}

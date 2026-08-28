package util

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func Test_UnregisterTwice_NormalCollector(t *testing.T) {
	u := WrapWithUnregisterer(prometheus.NewRegistry())
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_metric",
		Help: "Test metric.",
	})
	u.Register(c)
	require.True(t, u.Unregister(c))
	require.False(t, u.Unregister(c))
}

type uncheckedCollector struct{}

func (uncheckedCollector) Describe(chan<- *prometheus.Desc) {}

func (uncheckedCollector) Collect(chan<- prometheus.Metric) {}

var _ prometheus.Collector = uncheckedCollector{}

func Test_UnregisterTwice_UncheckedCollector(t *testing.T) {
	u := WrapWithUnregisterer(prometheus.NewRegistry())
	c := uncheckedCollector{}
	u.Register(c)
	require.True(t, u.Unregister(c))
	require.True(t, u.Unregister(c))
}

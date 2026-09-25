package client

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestDisableTenantLabel(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg, true)
	m.sentEntries.WithLabelValues("loki.example").Inc()
	m.droppedEntries.WithLabelValues("loki.example", reasonGeneric).Inc()
	m.requestDuration.WithLabelValues("204", "loki.example").Observe(0.1)

	families, err := reg.Gather()
	require.NoError(t, err)
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				require.NotEqual(t, labelTenant, label.GetName(), "metric %s unexpectedly has tenant label", family.GetName())
			}
		}
	}
	require.Len(t, families, 3)
}

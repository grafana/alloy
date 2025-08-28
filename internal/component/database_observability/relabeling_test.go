package database_observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelBuilder_Get(t *testing.T) {
	a, b := "a", "b"
	av, bv := "1", "2"
	labels := []*dto.LabelPair{
		{Name: &a, Value: &av},
		{Name: &b, Value: &bv},
	}
	lb := newLabelBuilder(labels)

	assert.Equal(t, "1", lb.Get("a"))
	assert.Equal(t, "2", lb.Get("b"))
}

func TestLabelBuilder_Range(t *testing.T) {
	a, b := "a", "b"
	av, bv := "1", "2"
	labels := []*dto.LabelPair{
		{Name: &a, Value: &av},
		{Name: &b, Value: &bv},
	}
	lb := newLabelBuilder(labels)

	gotMap := map[string]string{}
	lb.Range(func(label, value string) {
		gotMap[label] = value
	})
	assert.Len(t, gotMap, 2)
	assert.Equal(t, "1", gotMap["a"])
	assert.Equal(t, "2", gotMap["b"])
}

func TestLabelBuilder_Set(t *testing.T) {
	a, b := "a", "b"
	av, bv := "1", "2"
	labels := []*dto.LabelPair{
		{Name: &a, Value: &av},
		{Name: &b, Value: &bv},
	}
	lb := newLabelBuilder(labels)

	lb.Set("a", "3")
	assert.Equal(t, "3", lb.Get("a"))
}

func TestLabelBuilder_Delete(t *testing.T) {
	a, b := "a", "b"
	av, bv := "1", "2"
	labels := []*dto.LabelPair{
		{Name: &a, Value: &av},
		{Name: &b, Value: &bv},
	}
	lb := newLabelBuilder(labels)

	assert.Equal(t, "1", lb.Get("a"))
	lb.Del("a")
	assert.Equal(t, "", lb.Get("a"))
}

func TestRelabelingGatherer_AddsAndReplacesServerID(t *testing.T) {
	reg := prometheus.NewRegistry()

	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_metric_total",
		Help: "test metric",
	}, []string{"instance"})

	require.NoError(t, reg.Register(cv))

	cv.WithLabelValues("inst-1").Inc()

	g := &RelabelingGatherer{
		gatherer: reg,
		rules:    GetRelabelingRules("some-server-id"),
	}

	metrics, err := g.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range metrics {
		if mf.GetName() == "test_metric_total" {
			for _, m := range mf.GetMetric() {
				assert.True(t, hasLabel(m.GetLabel(), "server_id", "some-server-id"))
			}
			found = true
		}
	}
	require.True(t, found)
}

func hasLabel(labels []*dto.LabelPair, name, value string) bool {
	for _, l := range labels {
		if l.GetName() == name && l.GetValue() == value {
			return true
		}
	}
	return false
}

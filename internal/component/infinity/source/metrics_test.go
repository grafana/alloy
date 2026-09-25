package source

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestSanitizeName(t *testing.T) {
	tests := map[string]string{
		"latency":    "latency",
		"latency-ms": "latency_ms",
		"9lives":     "_9lives",
		"a.b c":      "a_b_c",
		"":           "_",
	}
	for in, want := range tests {
		require.Equal(t, want, sanitizeName(in), in)
	}
}

func TestFrameToSamples(t *testing.T) {
	f := data.NewFrame("q",
		data.NewField("service", nil, []*string{ptr("api"), ptr("db")}),
		data.NewField("latency-ms", nil, []*float64{ptr(12.0), nil}),
		data.NewField("healthy", nil, []bool{true, false}),
	)

	res, err := frameToSamples(f, "infinity.source.x", "status", metricsSpec{prefix: "svc_"})
	require.NoError(t, err)
	require.Zero(t, res.duplicates)
	require.ElementsMatch(t, []sample{
		{labels: labels.FromStrings("__name__", "svc_latency_ms", "service", "api", "job", "infinity.source.x", "instance", "status"), value: 12},
		{labels: labels.FromStrings("__name__", "svc_healthy", "service", "api", "job", "infinity.source.x", "instance", "status"), value: 1},
		{labels: labels.FromStrings("__name__", "svc_healthy", "service", "db", "job", "infinity.source.x", "instance", "status"), value: 0},
	}, res.samples)
}

func TestFrameToSamplesDuplicatesKeepFirst(t *testing.T) {
	f := data.NewFrame("q",
		data.NewField("service", nil, []string{"api", "api"}),
		data.NewField("v", nil, []float64{1, 2}),
	)

	res, err := frameToSamples(f, "j", "i", metricsSpec{})
	require.NoError(t, err)
	require.Equal(t, 1, res.duplicates)
	require.Len(t, res.samples, 1)
	require.Equal(t, 1.0, res.samples[0].value)
}

func TestFrameToSamplesJobInstanceOverride(t *testing.T) {
	f := data.NewFrame("q",
		data.NewField("job", nil, []string{"from-api"}),
		data.NewField("v", nil, []float64{1}),
	)

	res, err := frameToSamples(f, "j", "i", metricsSpec{})
	require.NoError(t, err)
	require.True(t, res.overridden)
	require.Equal(t, "j", res.samples[0].labels.Get("job"))
}

func TestFrameToSamplesSeriesLimit(t *testing.T) {
	f := data.NewFrame("q",
		data.NewField("k", nil, []string{"a", "b", "c"}),
		data.NewField("v", nil, []float64{1, 2, 3}),
	)

	_, err := frameToSamples(f, "j", "i", metricsSpec{seriesLimit: 2})
	require.Equal(t, reasonSeriesLimit, reasonOf(err))
}

func TestTracker(t *testing.T) {
	a := sample{labels: labels.FromStrings("__name__", "a")}
	b := sample{labels: labels.FromStrings("__name__", "b")}
	up := upSample("j", "i", 1)

	tr := newTracker()
	require.Empty(t, tr.stale([]sample{a, b, up}))
	tr.replace([]sample{a, b, up})

	require.Equal(t, []labels.Labels{b.labels}, tr.stale([]sample{a, up}))
	tr.replace([]sample{a, up})
	require.Equal(t, 2, tr.len(), "the tracker holds only the last poll")

	require.ElementsMatch(t, []labels.Labels{a.labels}, tr.allExcept(up.labels))
	require.ElementsMatch(t, []labels.Labels{a.labels, up.labels}, tr.all())

	tr.reset()
	require.Zero(t, tr.len())
}

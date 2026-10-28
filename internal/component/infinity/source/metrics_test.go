package source

import (
	"fmt"
	"runtime"
	"strings"
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

func TestFrameToSamplesReservedUp(t *testing.T) {
	f := data.NewFrame("q",
		data.NewField("up", nil, []bool{false}),
		data.NewField("v", nil, []float64{1}),
	)

	res, err := frameToSamples(f, "j", "i", metricsSpec{})
	require.NoError(t, err)
	require.True(t, res.reservedUp)
	require.Len(t, res.samples, 1)
	require.Equal(t, "v", res.samples[0].labels.Get("__name__"))
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

func TestFrameToSamplesRejectsLongLabelValue(t *testing.T) {
	// A value of 16 MiB or more makes the labels package panic.
	f := data.NewFrame("q",
		data.NewField("k", nil, []string{strings.Repeat("a", 17<<20)}),
		data.NewField("v", nil, []float64{1}),
	)

	_, err := frameToSamples(f, "j", "i", metricsSpec{})
	require.Equal(t, reasonTooLarge, reasonOf(err))
}

// TestFrameToSamplesLabelBudget checks that one row with a long label and
// many numeric columns fails before each sample copies the label.
func TestFrameToSamplesLabelBudget(t *testing.T) {
	fields := []*data.Field{data.NewField("k", nil, []string{strings.Repeat("a", maxLabelValueLen)})}
	for i := range 4_000 {
		fields = append(fields, data.NewField(fmt.Sprintf("v%d", i), nil, []float64{1}))
	}
	f := data.NewFrame("q", fields...)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := frameToSamples(f, "j", "i", metricsSpec{})
	runtime.ReadMemStats(&after)

	require.Equal(t, reasonTooLarge, reasonOf(err))
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(2*maxLabelBytes), "the budget must stop the copies")
}

func TestFrameToSamplesRecoversPanic(t *testing.T) {
	// The second field is shorter than the frame, so reading it panics.
	f := data.NewFrame("q",
		data.NewField("v", nil, []float64{1, 2}),
		data.NewField("k", nil, []string{"a"}),
	)

	_, err := frameToSamples(f, "j", "i", metricsSpec{})
	require.Equal(t, reasonParse, reasonOf(err))
}

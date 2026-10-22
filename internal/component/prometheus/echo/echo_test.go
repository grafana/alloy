package echo

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/appenders/adapter"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestArgumentsDefaults(t *testing.T) {
	args := Arguments{}
	args.SetToDefault()
	require.Equal(t, DefaultArguments, args)
}

func TestComponent_Creation(t *testing.T) {
	ctx := componenttest.TestContext(t)

	comp, err := New(component.Options{
		ID:     "test",
		Logger: logging.NewSlogNop(),
	}, Arguments{})
	require.NoError(t, err)
	require.NotNil(t, comp)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	time.Sleep(100 * time.Millisecond)
}

func TestComponent_ExportsReceiver(t *testing.T) {
	var exports component.Exports

	comp, err := New(component.Options{
		ID:     "test",
		Logger: logging.NewSlogNop(),
		OnStateChange: func(e component.Exports) {
			exports = e
		},
	}, Arguments{})
	require.NoError(t, err)
	require.NotNil(t, comp)

	echoExports, ok := exports.(Exports)
	require.True(t, ok)
	require.NotNil(t, echoExports.Receiver)

	require.Equal(t, comp, echoExports.Receiver)
}

func TestComponent_Update(t *testing.T) {
	comp, err := New(component.Options{
		ID:     "test",
		Logger: logging.NewSlogNop(),
	}, Arguments{})
	require.NoError(t, err)

	err = comp.Update(Arguments{})
	require.NoError(t, err)
}

// appendCall is the arguments of one AppenderV2.Append call.
type appendCall struct {
	lbls      labels.Labels
	st, t     int64
	v         float64
	h         *histogram.Histogram
	fh        *histogram.FloatHistogram
	exemplars []exemplar.Exemplar
	meta      metadata.Metadata
}

func TestAppender(t *testing.T) {
	requests := labels.FromStrings("__name__", "requests_total", "job", "test")
	temperature := labels.FromStrings("__name__", "temperature", "job", "test")
	latency := labels.FromStrings("__name__", "latency", "job", "test")

	tests := []struct {
		name     string
		format   string
		appends  []appendCall
		rollback bool
		expected string
		// expectedExemplars is compared with the exemplars the appender
		// recorded, because exemplars aren't printed for untyped metrics.
		expectedExemplars map[string]seriesExemplar
		// TODO: Remove v2Only when the V1 appender is deleted.
		v2Only bool
	}{
		{
			name:    "single sample",
			appends: []appendCall{{lbls: requests, t: 1000, v: 7}},
			expected: `# TYPE requests_total untyped
requests_total{job="test"} 7 1000
`,
		},
		{
			name: "multiple metrics are sorted",
			appends: []appendCall{
				{lbls: temperature, t: 1000, v: 21.5},
				{lbls: labels.FromStrings("__name__", "temperature", "job", "a"), t: 1000, v: 20},
				{lbls: requests, t: 1000, v: 7},
			},
			expected: `# TYPE requests_total untyped
requests_total{job="test"} 7 1000
# TYPE temperature untyped
temperature{job="a"} 20 1000
temperature{job="test"} 21.5 1000
`,
		},
		{
			name:    "sample without timestamp",
			appends: []appendCall{{lbls: requests, v: 7}},
			expected: `# TYPE requests_total untyped
requests_total{job="test"} 7
`,
		},
		{
			name:    "openmetrics format",
			format:  "openmetrics",
			appends: []appendCall{{lbls: requests, t: 1000, v: 7}},
			expected: `# TYPE requests_total unknown
requests_total{job="test"} 7.0 1.0
`,
		},
		{
			name:    "unknown format falls back to text",
			format:  "bogus",
			appends: []appendCall{{lbls: requests, t: 1000, v: 7}},
			expected: `# TYPE requests_total untyped
requests_total{job="test"} 7 1000
`,
		},
		{
			name: "metadata",
			appends: []appendCall{{
				lbls: requests,
				t:    1000,
				v:    7,
				meta: metadata.Metadata{Type: "counter", Help: "Total requests."},
			}},
			expected: `# HELP requests_total Total requests.
# TYPE requests_total untyped
requests_total{job="test"} 7 1000
`,
		},
		{
			name:    "histogram",
			appends: []appendCall{{lbls: latency, t: 1000, h: &histogram.Histogram{Count: 10, Sum: 4.5}}},
			expected: `# TYPE latency histogram
latency_bucket{job="test",le="+Inf"} 10
latency_sum{job="test"} 4.5
latency_count{job="test"} 10
`,
		},
		{
			name:    "float histogram",
			appends: []appendCall{{lbls: latency, t: 1000, fh: &histogram.FloatHistogram{Count: 2.5, Sum: 10.25}}},
			expected: `# TYPE latency histogram
latency_bucket{job="test",le="+Inf"} 2.5
latency_sum{job="test"} 10.25
latency_count{job="test"} 2.5
`,
			// The V1 appender ignores float histograms.
			v2Only: true,
		},
		{
			name: "start timestamp is ignored",
			appends: []appendCall{
				{lbls: requests, st: 500, t: 1000, v: 7},
				{lbls: latency, st: 500, t: 1000, h: &histogram.Histogram{Count: 10, Sum: 4.5}},
			},
			expected: `# TYPE latency histogram
latency_bucket{job="test",le="+Inf"} 10
latency_sum{job="test"} 4.5
latency_count{job="test"} 10
# TYPE requests_total untyped
requests_total{job="test"} 7 1000
`,
		},
		{
			name: "last sample of a series wins",
			appends: []appendCall{
				{lbls: requests, t: 1000, v: 7},
				{lbls: requests, t: 2000, v: 8},
			},
			expected: `# TYPE requests_total untyped
requests_total{job="test"} 8 2000
`,
		},
		{
			name: "last exemplar of a series wins",
			appends: []appendCall{{
				lbls: requests,
				t:    1000,
				v:    7,
				exemplars: []exemplar.Exemplar{
					{Labels: labels.FromStrings("trace_id", "abc"), Value: 1, Ts: 900, HasTs: true},
					{Labels: labels.FromStrings("trace_id", "def"), Value: 2, Ts: 950, HasTs: true},
				},
			}},
			expected: `# TYPE requests_total untyped
requests_total{job="test"} 7 1000
`,
			expectedExemplars: map[string]seriesExemplar{
				requests.String(): {
					Labels:   requests,
					Exemplar: exemplar.Exemplar{Labels: labels.FromStrings("trace_id", "def"), Value: 2, Ts: 950, HasTs: true},
				},
			},
		},
		{
			name:     "rollback discards samples",
			appends:  []appendCall{{lbls: requests, t: 1000, v: 7}},
			rollback: true,
			expected: "",
		},
	}

	// TODO: Delete the v1 entry when the V1 appender is deleted.
	appenders := []struct {
		name string
		new  func(c *Component, ctx context.Context) (storage.AppenderV2, *echoAppender)
	}{
		{
			name: "v2",
			new: func(c *Component, ctx context.Context) (storage.AppenderV2, *echoAppender) {
				app := c.AppenderV2(ctx).(*echoAppenderV2)
				return app, app.buf
			},
		},
		{
			// The adapter makes the same V1 calls a scrape loop would.
			name: "v1",
			new: func(c *Component, ctx context.Context) (storage.AppenderV2, *echoAppender) {
				app := c.Appender(ctx).(*echoAppender)
				return adapter.AppenderV1AsV2(app), app
			},
		},
	}

	for _, tc := range tests {
		for _, a := range appenders {
			t.Run(tc.name+"/"+a.name, func(t *testing.T) {
				if tc.v2Only && a.name != "v2" {
					t.Skip("not supported by the V1 appender")
				}

				comp, out := newTestComponent(t, Arguments{Format: tc.format})
				app, buf := a.new(comp, t.Context())

				for _, call := range tc.appends {
					ref, err := app.Append(0, call.lbls, call.st, call.t, call.v, call.h, call.fh, storage.AppendV2Options{
						Exemplars: call.exemplars,
						Metadata:  call.meta,
					})
					require.NoError(t, err)
					require.Equal(t, storage.SeriesRef(0), ref)
				}

				if tc.expectedExemplars != nil {
					require.Equal(t, tc.expectedExemplars, buf.exemplars)
				}

				if tc.rollback {
					require.NoError(t, app.Rollback())
				}
				require.NoError(t, app.Commit())

				require.Equal(t, tc.expected, printedMetrics(t, out))
			})
		}
	}
}

func newTestComponent(t *testing.T, args Arguments) (*Component, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	logger, err := logging.New(&buf, logging.Options{
		Level:  logging.LevelInfo,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)

	comp, err := New(component.Options{
		ID:     "test",
		Logger: logger.Slog(),
	}, args)
	require.NoError(t, err)
	return comp, &buf
}

// printedMetrics returns the unquoted metrics field of the log line that
// echo writes on Commit.
func printedMetrics(t *testing.T, buf *bytes.Buffer) string {
	t.Helper()
	out := strings.TrimSpace(buf.String())
	_, quoted, found := strings.Cut(out, " metrics=")
	require.True(t, found, "no metrics field found in %q", out)
	metrics, err := strconv.Unquote(quoted)
	require.NoError(t, err)
	return metrics
}

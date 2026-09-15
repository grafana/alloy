package usagestats

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/util"
)

// TestReporter checks that the reporter POSTs the components tracked for each
// engine to grafana.com. Components are supplied directly; turning an OTel
// config into component types is covered separately by TestExtractOtelComponents.
func TestReporter(t *testing.T) {
	tests := map[string]struct {
		setup func(tr *Tracker)
		want  map[string]any
	}{
		"empty tracker reports no metrics": {
			setup: func(*Tracker) {},
			want:  map[string]any{},
		},
		"default engine reports enabled components": {
			setup: func(tr *Tracker) {
				tr.SetEnabledComponentsFunc(func() []string {
					return []string{"prometheus.scrape", "loki.write"}
				})
			},
			want: map[string]any{
				"enabled-components": []string{"prometheus.scrape", "loki.write"},
			},
		},
		"otel engine reports otel components": {
			setup: func(tr *Tracker) {
				tr.SetOTelComponentsFunc(func() map[string][]string {
					return map[string][]string{"receivers": {"otlp"}, "processors": {"batch"}}
				})
			},
			want: map[string]any{
				"otel-components": map[string][]string{"receivers": {"otlp"}, "processors": {"batch"}},
			},
		},
		"otel engine also reports embedded alloyengine components": {
			setup: func(tr *Tracker) {
				tr.SetOTelComponentsFunc(func() map[string][]string {
					return map[string][]string{"receivers": {"otlp"}}
				})
				tr.SetAlloyEngineComponentsFunc(func() []string {
					return []string{"prometheus.scrape", "loki.write"}
				})
			},
			want: map[string]any{
				"otel-components":        map[string][]string{"receivers": {"otlp"}},
				"alloyengine-components": []string{"prometheus.scrape", "loki.write"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tr := &Tracker{}
			tc.setup(tr)

			// Capture the body the reporter POSTs.
			gotBody := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				gotBody <- body
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			origURL := usageStatsURL
			t.Cleanup(func() { usageStatsURL = origURL })
			usageStatsURL = server.URL

			rep := &reporter{
				logger: util.TestAlloyLogger(t).Slog(),
				seed:   &alloyseed.Seed{UID: "test-uid"},
			}
			require.NoError(t, rep.reportUsage(context.Background(), time.Now(), tr.Metrics()))

			var got Report
			require.NoError(t, json.Unmarshal(<-gotBody, &got))
			require.Equal(t, "test-uid", got.UsageStatsID)

			// Compare via JSON: the wire representation is what the backend reads,
			// and it normalizes away Go's []string vs []any after a round-trip.
			wantJSON, err := json.Marshal(tc.want)
			require.NoError(t, err)
			gotJSON, err := json.Marshal(got.Metrics)
			require.NoError(t, err)
			require.JSONEq(t, string(wantJSON), string(gotJSON))
		})
	}
}

// TestExtractOtelComponents covers turning an OTel Collector config (as parsed
// from YAML) into component types grouped by kind: ids collapse to their type
// (otlp/2 -> otlp) and dedupe within a kind, non-component sections are ignored,
// and malformed sections and empty ids are skipped.
func TestExtractOtelComponents(t *testing.T) {
	const groupedConfig = `
receivers:
  otlp:
  otlp/2:
  prometheus/scrape:
processors:
  batch:
exporters:
  debug:
  otlp:
service:
  pipelines:
    metrics:
`
	const malformedConfig = `
receivers: not-a-map
processors:
exporters:
  "":
  "/foo":
  otlp:
`
	tests := map[string]struct {
		config string
		want   map[string][]string
	}{
		"empty config": {
			config: "",
			want:   map[string][]string{},
		},
		"groups by kind and collapses ids to types": {
			config: groupedConfig,
			want: map[string][]string{
				"receivers":  {"otlp", "prometheus"},
				"processors": {"batch"},
				"exporters":  {"debug", "otlp"},
			},
		},
		"skips malformed sections and empty ids": {
			config: malformedConfig,
			want: map[string][]string{
				"exporters": {"otlp"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var conf map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(tc.config), &conf))
			require.Equal(t, tc.want, ExtractOtelComponents(conf))
		})
	}
}

func Test_ReportLoop_BoundedRetriesOnFailure(t *testing.T) {
	// A failed report advances the schedule by a full reportInterval, so a
	// persistently unreachable endpoint is contacted at most once per interval
	// rather than on every (much shorter) check tick. With the interval set far
	// longer than the test, a correct reporter contacts the endpoint exactly
	// once regardless of how slowly the test host runs.
	reportCheckInterval = time.Millisecond
	reportInterval = time.Hour
	reportBackoffConfig = backoff.Config{
		MinBackoff: time.Millisecond,
		MaxBackoff: 2 * time.Millisecond,
		MaxRetries: 2,
	}

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	usageStatsURL = server.URL

	r := &reporter{logger: util.TestAlloyLogger(t).Slog()}
	// Make the first report eligible immediately; otherwise the loop waits a
	// full (now hour-long) interval before its first attempt.
	r.lastReport = time.Now().Add(-2 * reportInterval)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	var cycles atomic.Int64
	metricsFunc := func() map[string]any {
		cycles.Add(1)
		return map[string]any{}
	}
	require.Equal(t, context.DeadlineExceeded, r.run(ctx, metricsFunc))

	require.Equal(t, int64(1), cycles.Load())
}

func Test_NextReport(t *testing.T) {
	fixtures := map[string]struct {
		interval  time.Duration
		createdAt time.Time
		now       time.Time

		next time.Time
	}{
		"createdAt aligned with interval and now": {
			interval:  1 * time.Hour,
			createdAt: time.Unix(0, time.Hour.Nanoseconds()),
			now:       time.Unix(0, 2*time.Hour.Nanoseconds()),
			next:      time.Unix(0, 2*time.Hour.Nanoseconds()),
		},
		"createdAt aligned with interval": {
			interval:  1 * time.Hour,
			createdAt: time.Unix(0, time.Hour.Nanoseconds()),
			now:       time.Unix(0, 2*time.Hour.Nanoseconds()+1),
			next:      time.Unix(0, 3*time.Hour.Nanoseconds()),
		},
		"createdAt not aligned": {
			interval:  1 * time.Hour,
			createdAt: time.Unix(0, time.Hour.Nanoseconds()+18*time.Minute.Nanoseconds()+20*time.Millisecond.Nanoseconds()),
			now:       time.Unix(0, 2*time.Hour.Nanoseconds()+1),
			next:      time.Unix(0, 2*time.Hour.Nanoseconds()+18*time.Minute.Nanoseconds()+20*time.Millisecond.Nanoseconds()),
		},
	}
	for name, f := range fixtures {
		t.Run(name, func(t *testing.T) {
			next := nextReport(f.interval, f.createdAt, f.now)
			require.Equal(t, f.next, next)
		})
	}
}

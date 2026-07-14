package static

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{
			name: "valid",
			args: Arguments{ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: "build_info", Value: 1, Type: "gauge"}}},
		},
		{
			name:    "unsupported type",
			args:    Arguments{ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: "build_info", Type: "histogram"}}},
			wantErr: `unsupported type "histogram"`,
		},
		{
			name:    "no metrics",
			args:    Arguments{ScrapeInterval: time.Minute},
			wantErr: "at least one metric block",
		},
		{
			name:    "empty name",
			args:    Arguments{ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: ""}}},
			wantErr: "name must not be empty",
		},
		{
			name:    "invalid metric name",
			args:    Arguments{ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: "\xff\xfe"}}},
			wantErr: "is not a valid metric name",
		},
		{
			name:    "duplicate names",
			args:    Arguments{ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: "a", Type: "gauge"}, {Name: "a", Type: "gauge"}}},
			wantErr: "duplicate metric name",
		},
		{
			name:    "duplicate names only after prefix",
			args:    Arguments{Prefix: "p", ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: "a", Type: "gauge"}, {Name: "a", Type: "gauge"}}},
			wantErr: `duplicate metric name "p_a"`,
		},
		{
			name:    "zero interval",
			args:    Arguments{ScrapeInterval: 0, Metrics: []MetricConfig{{Name: "a"}}},
			wantErr: "scrape_interval must be greater than 0",
		},
		{
			name:    "invalid component label name",
			args:    Arguments{ScrapeInterval: time.Minute, Labels: map[string]string{"": "x"}, Metrics: []MetricConfig{{Name: "a", Type: "gauge"}}},
			wantErr: "is not a valid label name",
		},
		{
			name:    "invalid metric label name",
			args:    Arguments{ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: "a", Type: "gauge", Labels: map[string]string{"\xff": "x"}}}},
			wantErr: `metric[0]: "\xff" is not a valid label name`,
		},
		{
			name:    "reserved component label name",
			args:    Arguments{ScrapeInterval: time.Minute, Labels: map[string]string{"__name__": "x"}, Metrics: []MetricConfig{{Name: "a", Type: "gauge"}}},
			wantErr: `label "__name__" is reserved`,
		},
		{
			name:    "reserved metric label name",
			args:    Arguments{ScrapeInterval: time.Minute, Metrics: []MetricConfig{{Name: "a", Type: "gauge", Labels: map[string]string{"__name__": "x"}}}},
			wantErr: `metric[0]: label "__name__" is reserved`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.args.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestBuildSeries(t *testing.T) {
	args := Arguments{
		Prefix: "self_report",
		Labels: map[string]string{"region": "us-east-1", "color": "blue"},
		Metrics: []MetricConfig{
			{
				Name:   "build_info",
				Value:  1,
				Type:   "info",
				Help:   "build metadata",
				Labels: map[string]string{"color": "green"}, // overrides component label
			},
			{
				Name:  "uptime",
				Value: 42,
				Type:  "gauge",
			},
		},
	}

	series := buildSeries(args)
	require.Len(t, series, 2)

	require.Equal(t, 1.0, series[0].value)
	require.Equal(t, labels.FromStrings(
		"__name__", "self_report_build_info",
		"color", "green",
		"region", "us-east-1",
	), series[0].labels)
	require.Equal(t, model.MetricTypeInfo, series[0].metadata.Type)
	require.Equal(t, "build metadata", series[0].metadata.Help)

	require.Equal(t, 42.0, series[1].value)
	require.Equal(t, labels.FromStrings(
		"__name__", "self_report_uptime",
		"color", "blue",
		"region", "us-east-1",
	), series[1].labels)
	require.Equal(t, model.MetricTypeGauge, series[1].metadata.Type)
}

func TestMetricName(t *testing.T) {
	require.Equal(t, "build_info", metricName("", "build_info"))
	require.Equal(t, "self_report_build_info", metricName("self_report", "build_info"))
}

func TestEmit(t *testing.T) {
	sink := newCaptureSink()

	c := newTestComponent(t, Arguments{
		Prefix:         "self_report",
		ScrapeInterval: time.Minute,
		Labels:         map[string]string{"region": "us-east-1"},
		Metrics: []MetricConfig{
			{Name: "build_info", Value: 1, Type: "info", Help: "build metadata", Labels: map[string]string{"version": "1.0"}},
			{Name: "uptime", Value: 7, Type: "gauge"},
		},
		ForwardTo: []storage.Appendable{sink.appendable},
	})

	c.emit(context.Background())

	byName := sink.byName()
	require.Len(t, byName, 2)

	build := byName["self_report_build_info"]
	require.Equal(t, 1.0, build.value)
	require.Equal(t, "1.0", build.labels.Get("version"))
	require.Equal(t, "us-east-1", build.labels.Get("region"))
	require.Positive(t, build.ts)
	require.Equal(t, model.MetricTypeInfo, build.metadata.Type)
	require.Equal(t, "build metadata", build.metadata.Help)

	uptime := byName["self_report_uptime"]
	require.Equal(t, 7.0, uptime.value)
	require.Equal(t, model.MetricTypeGauge, uptime.metadata.Type)

	require.Equal(t, 2.0, counterValue(t, c.metricsEmitted))
}

func TestRunEmitsOnStartAndUpdate(t *testing.T) {
	sink := newCaptureSink()

	c := newTestComponent(t, Arguments{
		ScrapeInterval: time.Minute,
		Metrics:        []MetricConfig{{Name: "up", Value: 1}},
		ForwardTo:      []storage.Appendable{sink.appendable},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	// The initial emit on startup should reach the sink quickly.
	require.Eventually(t, func() bool {
		_, ok := sink.byName()["up"]
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	// An Update should trigger a re-emit with the new configuration.
	require.NoError(t, c.Update(Arguments{
		ScrapeInterval: time.Minute,
		Metrics:        []MetricConfig{{Name: "up", Value: 1}, {Name: "down", Value: 0}},
		ForwardTo:      []storage.Appendable{sink.appendable},
	}))

	require.Eventually(t, func() bool {
		names := sink.byName()
		_, hasUp := names["up"]
		_, hasDown := names["down"]
		return hasUp && hasDown
	}, 2*time.Second, 10*time.Millisecond)
}

// TestUpdateCoalescesInterval verifies that when multiple Updates land before
// Run consumes the reload signal, the newest interval is still applied: the
// signal is coalesced (buffer holds at most one) but the latest interval lives
// in shared state, which Run reads on wake-up.
func TestUpdateCoalescesInterval(t *testing.T) {
	sink := newCaptureSink()

	// New calls Update once, leaving one signal buffered. Run is intentionally
	// not started, so no signal is consumed.
	c := newTestComponent(t, Arguments{
		ScrapeInterval: time.Minute,
		Metrics:        []MetricConfig{{Name: "up", Value: 1}},
		ForwardTo:      []storage.Appendable{sink.appendable},
	})

	for _, d := range []time.Duration{30 * time.Second, 10 * time.Second} {
		require.NoError(t, c.Update(Arguments{
			ScrapeInterval: d,
			Metrics:        []MetricConfig{{Name: "up", Value: 1}},
			ForwardTo:      []storage.Appendable{sink.appendable},
		}))
	}

	// The reload signal is coalesced to a single pending item...
	require.Len(t, c.reload, 1)

	// ...but the latest interval is what Run will read.
	c.mut.RLock()
	got := c.interval
	c.mut.RUnlock()
	require.Equal(t, 10*time.Second, got)
}

func TestSyntaxDecode(t *testing.T) {
	cfg := `
		prefix = "self_report"
		metric {
			name = "build_info"
			labels {
				color = "green"
			}
		}
		metric {
			name  = "uptime"
			value = 12
			type  = "counter"
			help  = "seconds since start"
		}
		labels {
			region = "us-east-1"
		}
		forward_to = []
	`
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	require.Equal(t, "self_report", args.Prefix)
	require.Equal(t, time.Minute, args.ScrapeInterval) // default applied
	require.Equal(t, map[string]string{"region": "us-east-1"}, args.Labels)
	require.Len(t, args.Metrics, 2)
	require.Equal(t, "build_info", args.Metrics[0].Name)
	require.Equal(t, 1.0, args.Metrics[0].Value)      // default value
	require.Equal(t, "unknown", args.Metrics[0].Type) // default type
	require.Equal(t, map[string]string{"color": "green"}, args.Metrics[0].Labels)
	require.Equal(t, "uptime", args.Metrics[1].Name)
	require.Equal(t, 12.0, args.Metrics[1].Value)
	require.Equal(t, "counter", args.Metrics[1].Type)
	require.Equal(t, "seconds since start", args.Metrics[1].Help)
}

// newTestComponent builds a Component wired to in-memory services for testing.
func newTestComponent(t *testing.T, args Arguments) *Component {
	t.Helper()
	c, err := New(component.Options{
		ID:             "prometheus.static.test",
		Logger:         util.TestAlloyLogger(t).Slog(),
		Registerer:     prom.NewRegistry(),
		GetServiceData: getServiceData,
	}, args)
	require.NoError(t, err)
	return c
}

func getServiceData(name string) (any, error) {
	switch name {
	case labelstore.ServiceName:
		return labelstore.New(nil, prom.NewRegistry()), nil
	case livedebugging.ServiceName:
		return livedebugging.NewLiveDebugging(), nil
	default:
		return nil, fmt.Errorf("service not found %s", name)
	}
}

func counterValue(t *testing.T, c prom.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}

// capturedSample records a single appended sample and its metadata.
type capturedSample struct {
	labels   labels.Labels
	ts       int64
	value    float64
	metadata metadata.Metadata
}

// captureSink records samples appended to it via an Interceptor appendable.
type captureSink struct {
	mut        sync.Mutex
	samples    map[string]capturedSample
	appendable *prometheus.Interceptor
}

func newCaptureSink() *captureSink {
	s := &captureSink{samples: make(map[string]capturedSample)}
	s.appendable = prometheus.NewInterceptor(nil,
		prometheus.WithAppendHook(
			func(_ storage.SeriesRef, l labels.Labels, t int64, v float64, _ storage.Appender) (storage.SeriesRef, error) {
				s.mut.Lock()
				defer s.mut.Unlock()
				name := l.Get("__name__")
				cur := s.samples[name]
				cur.labels = l.Copy()
				cur.ts = t
				cur.value = v
				s.samples[name] = cur
				return 0, nil
			},
		),
		prometheus.WithMetadataHook(
			func(_ storage.SeriesRef, l labels.Labels, m metadata.Metadata, _ storage.Appender) (storage.SeriesRef, error) {
				s.mut.Lock()
				defer s.mut.Unlock()
				name := l.Get("__name__")
				cur := s.samples[name]
				cur.metadata = m
				s.samples[name] = cur
				return 0, nil
			},
		),
	)
	return s
}

func (s *captureSink) byName() map[string]capturedSample {
	s.mut.Lock()
	defer s.mut.Unlock()
	out := make(map[string]capturedSample, len(s.samples))
	for k, v := range s.samples {
		out[k] = v
	}
	return out
}

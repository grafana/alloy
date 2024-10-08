package queue

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func BenchmarkE2E(b *testing.B) {
	// Around 120k ops if you look at profile roughly 20k are actual implementation with the rest being benchmark
	// setup.
	type e2eTest struct {
		name   string
		maker  func(index int, app storage.Appender)
		tester func(samples []prompb.TimeSeries)
	}
	tests := []e2eTest{
		{
			// This should be ~1200 allocs an op
			name: "normal",
			maker: func(index int, app storage.Appender) {
				ts, v, lbls := makeSeries(index)
				_, _ = app.Append(0, lbls, ts, v)
			},
			tester: func(samples []prompb.TimeSeries) {
				b.Helper()
				for _, s := range samples {
					require.True(b, len(s.Samples) == 1)
				}
			},
		},
	}
	for _, test := range tests {
		b.Run(test.name, func(t *testing.B) {
			runBenchmark(t, test.maker, test.tester)
		})
	}
}

func runBenchmark(t *testing.B, add func(index int, appendable storage.Appender), _ func(samples []prompb.TimeSeries)) {
	t.ReportAllocs()
	l := log.NewNopLogger()
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	expCh := make(chan Exports, 1)
	c, err := newComponentBenchmark(t, l, srv.URL, expCh)
	require.NoError(t, err)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		runErr := c.Run(ctx)
		require.NoError(t, runErr)
	}()
	// Wait for export to spin up.
	exp := <-expCh

	index := 0
	app := exp.Receiver.Appender(ctx)

	for i := 0; i < t.N; i++ {
		index++
		add(index, app)
	}
	require.NoError(t, app.Commit())

	tm := time.NewTimer(10 * time.Second)
	select {
	case <-done:
	case <-tm.C:
	}
	cancel()
}

func newComponentBenchmark(t *testing.B, l log.Logger, url string, exp chan Exports) (*Queue, error) {
	return NewComponent(component.Options{
		ID:       "test",
		Logger:   l,
		DataPath: t.TempDir(),
		OnStateChange: func(e component.Exports) {
			exp <- e.(Exports)
		},
		Registerer: fakeRegistry{},
		Tracer:     nil,
	}, Arguments{
		TTL: 2 * time.Hour,
		Serialization: Serialization{
			MaxSignalsToBatch: 100_000,
			BatchFrequency:    1 * time.Second,
		},
		Endpoints: []EndpointConfig{{
			Name:                    "test",
			URL:                     url,
			Timeout:                 10 * time.Second,
			RetryBackoff:            1 * time.Second,
			MaxRetryBackoffAttempts: 0,
			BatchCount:              50,
			FlushFrequency:          1 * time.Second,
			QueueCount:              1,
		}},
	})
}

var _ prometheus.Registerer = (*fakeRegistry)(nil)

type fakeRegistry struct{}

func (f fakeRegistry) Register(collector prometheus.Collector) error {
	return nil
}

func (f fakeRegistry) MustRegister(collector ...prometheus.Collector) {
}

func (f fakeRegistry) Unregister(collector prometheus.Collector) bool {
	return true
}

package prometheus_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/log"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/service/labelstore"
)

func TestRollback(t *testing.T) {
	ls := labelstore.New(nil, promclient.DefaultRegisterer)
	fanout := prometheus.NewFanout([]storage.Appendable{prometheus.NewFanout(nil, "1", promclient.DefaultRegisterer, ls)}, "", promclient.DefaultRegisterer, ls)
	app := fanout.Appender(t.Context())
	err := app.Rollback()
	require.NoError(t, err)
}

func TestCommit(t *testing.T) {
	ls := labelstore.New(nil, promclient.DefaultRegisterer)
	fanout := prometheus.NewFanout([]storage.Appendable{prometheus.NewFanout(nil, "1", promclient.DefaultRegisterer, ls)}, "", promclient.DefaultRegisterer, ls)
	app := fanout.Appender(t.Context())
	err := app.Commit()
	require.NoError(t, err)
}

func BenchmarkAppenderFlows(b *testing.B) {
	numberOfMetrics := []int{2000}
	for _, n := range numberOfMetrics {
		metrics := setupMetrics(n)
		now := time.Now().UnixMilli()

		testName := fmt.Sprintf("labelstore/new-metrics=%d", n)
		ls := labelstore.New(log.NewNopLogger(), promclient.DefaultRegisterer)
		rw1 := remotewrite.NewInterceptor("1", &atomic.Bool{}, noopDebugDataPublisher{}, ls, noopStore{})
		rw2 := remotewrite.NewInterceptor("2", &atomic.Bool{}, noopDebugDataPublisher{}, ls, noopStore{})
		children := []storage.Appendable{rw1, rw2}
		fanout := prometheus.NewFanout(children, "fanout", promclient.DefaultRegisterer, ls)

		b.Run(testName, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				app := fanout.Appender(b.Context())
				for _, metric := range metrics {
					app.Append(0, metric, now, 1.0)
				}
				app.Commit()

				b.StopTimer()
				fanout.Clear()
				b.StartTimer()
			}
		})

		testName = fmt.Sprintf("seriesref/new-metrics=%d", n)
		ls = labelstore.New(log.NewNopLogger(), promclient.DefaultRegisterer, false)
		rw1 = remotewrite.NewInterceptor("1", &atomic.Bool{}, noopDebugDataPublisher{}, ls, noopStore{})
		rw2 = remotewrite.NewInterceptor("2", &atomic.Bool{}, noopDebugDataPublisher{}, ls, noopStore{})
		children = []storage.Appendable{rw1, rw2}
		fanout = prometheus.NewFanout(children, "fanout", promclient.DefaultRegisterer, ls)

		b.Run(testName, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				app := fanout.Appender(b.Context())
				for _, metric := range metrics {
					app.Append(0, metric, now, 1.0)
				}
				app.Commit()

				b.StopTimer()
				fanout.Clear()
				b.StartTimer()
			}
		})
	}
}

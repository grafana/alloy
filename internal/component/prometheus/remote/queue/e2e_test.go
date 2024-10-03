package queue

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestE2E(t *testing.T) {
	type e2eTest struct {
		name     string
		maker    func(index int, app storage.Appender) (float64, labels.Labels)
		tester   func(samples []prompb.TimeSeries)
		testMeta func(samples []prompb.MetricMetadata)
	}
	tests := []e2eTest{
		{
			name: "normal",
			maker: func(index int, app storage.Appender) (float64, labels.Labels) {
				ts, v, lbls := makeSeries(index)
				_, errApp := app.Append(0, lbls, ts, v)
				require.NoError(t, errApp)
				return v, lbls
			},
			tester: func(samples []prompb.TimeSeries) {
				t.Helper()
				for _, s := range samples {
					require.True(t, len(s.Samples) == 1)
					require.True(t, s.Samples[0].Timestamp > 0)
					require.True(t, s.Samples[0].Value > 0)
					require.True(t, len(s.Labels) == 1)
					require.Truef(t, s.Labels[0].Name == fmt.Sprintf("name_%d", int(s.Samples[0].Value)), "%d name %s", int(s.Samples[0].Value), s.Labels[0].Name)
					require.True(t, s.Labels[0].Value == fmt.Sprintf("value_%d", int(s.Samples[0].Value)))
				}
			},
		},
		{
			name: "metadata",
			maker: func(index int, app storage.Appender) (float64, labels.Labels) {
				meta, lbls := makeMetadata(index)
				_, errApp := app.UpdateMetadata(0, lbls, meta)
				require.NoError(t, errApp)
				return 0, lbls
			},
			testMeta: func(samples []prompb.MetricMetadata) {
				for _, s := range samples {
					require.True(t, s.GetUnit() == "seconds")
					require.True(t, s.Help == "metadata help")
					require.True(t, s.Unit == "seconds")
					require.True(t, s.Type == prompb.MetricMetadata_COUNTER)
					require.True(t, strings.HasPrefix(s.MetricFamilyName, "name_"))
				}
			},
		},

		{
			name: "histogram",
			maker: func(index int, app storage.Appender) (float64, labels.Labels) {
				ts, lbls, h := makeHistogram(index)
				_, errApp := app.AppendHistogram(0, lbls, ts, h, nil)
				require.NoError(t, errApp)
				return h.Sum, lbls
			},
			tester: func(samples []prompb.TimeSeries) {
				t.Helper()
				for _, s := range samples {
					require.True(t, len(s.Samples) == 1)
					require.True(t, s.Samples[0].Timestamp > 0)
					require.True(t, s.Samples[0].Value == 0)
					require.True(t, len(s.Labels) == 1)
					histSame(t, hist(int(s.Histograms[0].Sum)), s.Histograms[0])
				}
			},
		},
		{
			name: "float histogram",
			maker: func(index int, app storage.Appender) (float64, labels.Labels) {
				ts, lbls, h := makeFloatHistogram(index)
				_, errApp := app.AppendHistogram(0, lbls, ts, nil, h)
				require.NoError(t, errApp)
				return h.Sum, lbls
			},
			tester: func(samples []prompb.TimeSeries) {
				t.Helper()
				for _, s := range samples {
					require.True(t, len(s.Samples) == 1)
					require.True(t, s.Samples[0].Timestamp > 0)
					require.True(t, s.Samples[0].Value == 0)
					require.True(t, len(s.Labels) == 1)
					histFloatSame(t, histFloat(int(s.Histograms[0].Sum)), s.Histograms[0])
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runTest(t, test.maker, test.tester, test.testMeta)
		})
	}
}

const (
	iterations = 100
	items      = 10_000
)

func runTest(t *testing.T, add func(index int, appendable storage.Appender) (float64, labels.Labels), test func(samples []prompb.TimeSeries), metaTest func(meta []prompb.MetricMetadata)) {
	l := util.TestAlloyLogger(t)
	done := make(chan struct{})
	var series atomic.Int32
	var meta atomic.Int32
	samples := make([]prompb.TimeSeries, 0)
	metaSamples := make([]prompb.MetricMetadata, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newSamples, newMetadata := handlePost(t, w, r)
		series.Add(int32(len(newSamples)))
		meta.Add(int32(len(newMetadata)))
		samples = append(samples, newSamples...)
		metaSamples = append(metaSamples, newMetadata...)
		if series.Load() == iterations*items {
			done <- struct{}{}
		}
		if meta.Load() == iterations*items {
			done <- struct{}{}
		}
	}))
	expCh := make(chan Exports, 1)
	c, err := newComponent(t, l, srv.URL, expCh, prometheus.NewRegistry())
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
	results := make(map[float64]labels.Labels)
	mut := sync.Mutex{}

	for i := 0; i < iterations; i++ {
		go func() {
			app := exp.Receiver.Appender(ctx)
			for j := 0; j < items; j++ {
				index++
				v, lbl := add(index, app)
				mut.Lock()
				results[v] = lbl
				mut.Unlock()
			}
			require.NoError(t, app.Commit())
		}()
	}
	// This is a weird use case to handle eventually.
	tm := time.NewTimer(15 * time.Second)
	select {
	case <-done:
	case <-tm.C:
	}
	cancel()
	for _, s := range samples {
		if len(s.Histograms) == 1 {
			lbls, ok := results[s.Histograms[0].Sum]
			require.True(t, ok)
			for i, sLbl := range s.Labels {
				require.True(t, lbls[i].Name == sLbl.Name)
				require.True(t, lbls[i].Value == sLbl.Value)
			}
		} else {
			lbls, ok := results[s.Samples[0].Value]
			require.True(t, ok)
			for i, sLbl := range s.Labels {
				require.True(t, lbls[i].Name == sLbl.Name)
				require.True(t, lbls[i].Value == sLbl.Value)
			}
		}
	}
	if test != nil {
		test(samples)
	} else {
		metaTest(metaSamples)
	}
	require.Truef(t, types.OutStandingTimeSeriesBinary.Load() == 0, "there are %d time series not collected", types.OutStandingTimeSeriesBinary.Load())
}

func handlePost(t *testing.T, _ http.ResponseWriter, r *http.Request) ([]prompb.TimeSeries, []prompb.MetricMetadata) {
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	data, err = snappy.Decode(nil, data)
	require.NoError(t, err)

	var req prompb.WriteRequest
	err = req.Unmarshal(data)
	require.NoError(t, err)
	return req.GetTimeseries(), req.Metadata
}

func makeSeries(index int) (int64, float64, labels.Labels) {
	return time.Now().UTC().Unix(), float64(index), labels.FromStrings(fmt.Sprintf("name_%d", index), fmt.Sprintf("value_%d", index))
}

func makeMetadata(index int) (metadata.Metadata, labels.Labels) {
	return metadata.Metadata{
		Type: "counter",
		Unit: "seconds",
		Help: "metadata help",
	}, labels.FromStrings("__name__", fmt.Sprintf("name_%d", index))
}

func makeHistogram(index int) (int64, labels.Labels, *histogram.Histogram) {
	return time.Now().UTC().Unix(), labels.FromStrings(fmt.Sprintf("name_%d", index), fmt.Sprintf("value_%d", index)), hist(index)
}

func makeExemplar(index int) exemplar.Exemplar {
	return exemplar.Exemplar{
		Labels: labels.FromStrings(fmt.Sprintf("name_%d", index), fmt.Sprintf("value_%d", index)),
		Ts:     time.Now().Unix(),
		HasTs:  true,
		Value:  float64(index),
	}
}

func hist(i int) *histogram.Histogram {
	return &histogram.Histogram{
		CounterResetHint: 1,
		Schema:           2,
		ZeroThreshold:    3,
		ZeroCount:        4,
		Count:            5,
		Sum:              float64(i),
		PositiveSpans: []histogram.Span{
			{
				Offset: 1,
				Length: 2,
			},
		},
		NegativeSpans: []histogram.Span{
			{
				Offset: 3,
				Length: 4,
			},
		},
		PositiveBuckets: []int64{1, 2, 3},
		NegativeBuckets: []int64{1, 2, 3},
	}
}

func histSame(t *testing.T, h *histogram.Histogram, pb prompb.Histogram) {
	require.True(t, h.Sum == pb.Sum)
	require.True(t, h.ZeroCount == pb.ZeroCount.(*prompb.Histogram_ZeroCountInt).ZeroCountInt)
	require.True(t, h.Schema == pb.Schema)
	require.True(t, h.Count == pb.Count.(*prompb.Histogram_CountInt).CountInt)
	require.True(t, h.ZeroThreshold == pb.ZeroThreshold)
	require.True(t, int32(h.CounterResetHint) == int32(pb.ResetHint))
	require.True(t, reflect.DeepEqual(h.PositiveBuckets, pb.PositiveDeltas))
	require.True(t, reflect.DeepEqual(h.NegativeBuckets, pb.NegativeDeltas))
	histSpanSame(t, h.PositiveSpans, pb.PositiveSpans)
	histSpanSame(t, h.NegativeSpans, pb.NegativeSpans)
}

func histSpanSame(t *testing.T, h []histogram.Span, pb []prompb.BucketSpan) {
	require.True(t, len(h) == len(pb))
	for i := range h {
		require.True(t, h[i].Length == pb[i].Length)
		require.True(t, h[i].Offset == pb[i].Offset)
	}
}

func makeFloatHistogram(index int) (int64, labels.Labels, *histogram.FloatHistogram) {
	return time.Now().UTC().Unix(), labels.FromStrings(fmt.Sprintf("name_%d", index), fmt.Sprintf("value_%d", index)), histFloat(index)
}

func histFloat(i int) *histogram.FloatHistogram {
	return &histogram.FloatHistogram{
		CounterResetHint: 1,
		Schema:           2,
		ZeroThreshold:    3,
		ZeroCount:        4,
		Count:            5,
		Sum:              float64(i),
		PositiveSpans: []histogram.Span{
			{
				Offset: 1,
				Length: 2,
			},
		},
		NegativeSpans: []histogram.Span{
			{
				Offset: 3,
				Length: 4,
			},
		},
		PositiveBuckets: []float64{1.1, 2.2, 3.3},
		NegativeBuckets: []float64{1.2, 2.3, 3.4},
	}
}

func histFloatSame(t *testing.T, h *histogram.FloatHistogram, pb prompb.Histogram) {
	require.True(t, h.Sum == pb.Sum)
	require.True(t, h.ZeroCount == pb.ZeroCount.(*prompb.Histogram_ZeroCountFloat).ZeroCountFloat)
	require.True(t, h.Schema == pb.Schema)
	require.True(t, h.Count == pb.Count.(*prompb.Histogram_CountFloat).CountFloat)
	require.True(t, h.ZeroThreshold == pb.ZeroThreshold)
	require.True(t, int32(h.CounterResetHint) == int32(pb.ResetHint))
	require.True(t, reflect.DeepEqual(h.PositiveBuckets, pb.PositiveCounts))
	require.True(t, reflect.DeepEqual(h.NegativeBuckets, pb.NegativeCounts))
	histSpanSame(t, h.PositiveSpans, pb.PositiveSpans)
	histSpanSame(t, h.NegativeSpans, pb.NegativeSpans)
}

func newComponent(t *testing.T, l *logging.Logger, url string, exp chan Exports, reg prometheus.Registerer) (*Queue, error) {
	return NewComponent(component.Options{
		ID:       "test",
		Logger:   l,
		DataPath: t.TempDir(),
		OnStateChange: func(e component.Exports) {
			exp <- e.(Exports)
		},
		Registerer: reg,
		Tracer:     nil,
	}, Arguments{
		TTL:               2 * time.Hour,
		MaxSignalsToBatch: 10_000,
		BatchFrequency:    1 * time.Second,
		Connections: []ConnectionConfig{{
			Name:                    "test",
			URL:                     url,
			Timeout:                 20 * time.Second,
			RetryBackoff:            5 * time.Second,
			MaxRetryBackoffAttempts: 1,
			BatchCount:              50,
			FlushFrequency:          1 * time.Second,
			QueueCount:              1,
		}},
		ExternalLabels: nil,
	})
}

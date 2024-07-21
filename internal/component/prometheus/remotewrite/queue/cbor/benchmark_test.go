package cbor

import (
	"context"
	"fmt"
	"github.com/dustin/go-humanize"
	log2 "github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"math/rand"
	"runtime"
	"strconv"
	"testing"
	"time"
)

const metricCount = 100_000

func BenchmarkComplexCbor(b *testing.B) {
	series := generateSeries(metricCount, 10)
	totalBytes := 0
	totalMemory := 0
	for i := 0; i < b.N; i++ {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		q := &fq{}
		wr := newCBORWrite(q, 32*1024*1024, 100*time.Millisecond, log2.NewNopLogger(), prometheus.DefaultRegisterer)
		for _, j := range series {
			err := wr.AddMetric(j.l, nil, j.t, j.v, nil, nil, types.Sample)
			require.NoError(b, err)
		}
		err := wr.write()
		require.NoError(b, err)
		totalBytes += q.totalBytes
		runtime.ReadMemStats(&m2)
		totalMemory = int(m2.HeapInuse - m1.HeapInuse)
	}
	b.Log("bytes written", humanize.Bytes(uint64(totalBytes)))
	b.Log("bytes written per run", humanize.Bytes(uint64(totalBytes/b.N)))
	b.Log("memory used per run", humanize.Bytes(uint64(totalMemory/b.N)))

}

func BenchmarkSimpleCbor(b *testing.B) {
	series := generatePromPB(metricCount, 10)
	totalBytes := 0
	totalMemory := 0
	for i := 0; i < b.N; i++ {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		q := &fq{}
		wr, err := NewSerializer(32*1024*1024, 100*time.Millisecond, q)
		require.NoError(b, err)
		raw := make([]*Raw, 0)
		for _, j := range series {
			buf, err := j.Marshal()
			require.NoError(b, err)
			r := &Raw{
				Hash:  rand.Uint64(),
				Bytes: buf,
				Ts:    j.Samples[0].Timestamp,
			}
			require.NoError(b, err)
			raw = append(raw, r)
		}
		err = wr.Append(raw)
		require.NoError(b, err)
		totalBytes += q.totalBytes
		runtime.ReadMemStats(&m2)
		totalMemory = int(m2.HeapInuse - m1.HeapInuse)
	}
	b.Log("bytes written", humanize.Bytes(uint64(totalBytes)))
	b.Log("bytes written per run", humanize.Bytes(uint64(totalBytes/b.N)))
	b.Log("memory used per run", humanize.Bytes(uint64(totalMemory/b.N)))

}

func generateSeries(metricCount, labelCount int) []ts {
	series := make([]ts, metricCount)
	for i := 0; i < metricCount; i++ {
		timeseries := ts{
			v: rand.Float64(),
			t: time.Now().UTC().Unix(),
			l: labels.EmptyLabels(),
		}
		timeseries.l = append(timeseries.l, labels.Label{
			Name:  "__name__",
			Value: strconv.Itoa(i),
		})
		for j := 0; j < labelCount; j++ {
			timeseries.l = append(timeseries.l, labels.Label{
				Name:  fmt.Sprintf("name_%d", j),
				Value: fmt.Sprintf("value_%d", rand.Intn(20)),
			})
		}
		series[i] = timeseries
	}
	return series
}

func generatePromPB(metricCount, labelCount int) []prompb.TimeSeries {
	series := make([]prompb.TimeSeries, metricCount)
	for i := 0; i < metricCount; i++ {
		timeseries := prompb.TimeSeries{
			Samples: make([]prompb.Sample, 0),
			Labels:  make([]prompb.Label, 0),
		}

		timeseries.Samples = append(timeseries.Samples, prompb.Sample{
			Value:     rand.Float64(),
			Timestamp: time.Now().UTC().Unix(),
		})
		timeseries.Labels = append(timeseries.Labels, prompb.Label{
			Name:  "__name__",
			Value: strconv.Itoa(i),
		})
		for j := 0; j < labelCount; j++ {
			timeseries.Labels = append(timeseries.Labels, prompb.Label{
				Name:  fmt.Sprintf("name_%d", j),
				Value: fmt.Sprintf("value_%d", j),
			})
		}
		series[i] = timeseries
	}
	return series
}

type ts struct {
	v float64
	t int64
	l labels.Labels
}

type fq struct {
	totalBytes int
}

func (f *fq) Add(data []byte) (string, error) {
	f.totalBytes = f.totalBytes + len(data)
	return "ok", nil
}

func (f fq) Next(ctx context.Context, enc []byte) ([]byte, string, error) {
	return nil, "", nil
}

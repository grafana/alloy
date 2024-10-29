package network

import (
	"context"
	"github.com/grafana/alloy/internal/util"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/golang/snappy"
	"github.com/grafana/alloy/internal/component/prometheus/write/queue/types"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestSending(t *testing.T) {
	recordsFound := atomic.Uint32{}
	svr := httptest.NewServer(handler(t, http.StatusOK, func(wr *prompb.WriteRequest) {
		recordsFound.Add(uint32(len(wr.Timeseries)))
	}))

	defer svr.Close()
	ctx := context.Background()
	ctx, cncl := context.WithCancel(ctx)
	defer cncl()

	cc := types.ConnectionConfig{
		URL:           svr.URL,
		Timeout:       1 * time.Second,
		BatchCount:    10,
		FlushInterval: 1 * time.Second,
		Connections:   4,
	}

	logger := log.NewNopLogger()
	wr, err := New(cc, logger, func(s types.NetworkStats) {}, func(s types.NetworkStats) {})
	wr.Start()
	defer wr.Stop()

	require.NoError(t, err)
	for i := 0; i < 1_000; i++ {
		send(t, wr, ctx)
	}
	require.Eventually(t, func() bool {
		return recordsFound.Load() == 1_000
	}, 10*time.Second, 100*time.Millisecond)
}

func TestUpdatingConfig(t *testing.T) {
	recordsFound := atomic.Uint32{}
	lastBatchSize := atomic.Uint32{}
	svr := httptest.NewServer(handler(t, http.StatusOK, func(wr *prompb.WriteRequest) {
		lastBatchSize.Store(uint32(len(wr.Timeseries)))
		recordsFound.Add(uint32(len(wr.Timeseries)))
	}))

	defer svr.Close()

	cc := types.ConnectionConfig{
		URL:           svr.URL,
		Timeout:       1 * time.Second,
		BatchCount:    10,
		FlushInterval: 5 * time.Second,
		Connections:   1,
	}

	logger := util.TestAlloyLogger(t)

	wr, err := New(cc, logger, func(s types.NetworkStats) {}, func(s types.NetworkStats) {})
	require.NoError(t, err)
	wr.Start()
	defer wr.Stop()

	cc2 := types.ConnectionConfig{
		URL:           svr.URL,
		Timeout:       1 * time.Second,
		BatchCount:    20,
		FlushInterval: 5 * time.Second,
		Connections:   1,
	}
	ctx := context.Background()
	err = wr.UpdateConfig(ctx, cc2)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	for i := 0; i < 100; i++ {
		send(t, wr, ctx)
	}
	require.Eventuallyf(t, func() bool {
		return recordsFound.Load() == 100
	}, 20*time.Second, 1*time.Second, "record count should be 100 but is %d", recordsFound.Load())

	require.Truef(t, lastBatchSize.Load() == 20, "batch_count should be 20 but is %d", lastBatchSize.Load())
}

func TestRetry(t *testing.T) {
	retries := atomic.Uint32{}
	var previous *prompb.WriteRequest
	svr := httptest.NewServer(handler(t, http.StatusTooManyRequests, func(wr *prompb.WriteRequest) {
		retries.Add(1)
		// Check that we are getting the same sample back.
		if previous == nil {
			previous = wr
		} else {
			require.True(t, previous.Timeseries[0].Labels[0].Value == wr.Timeseries[0].Labels[0].Value)
		}
	}))
	defer svr.Close()
	ctx := context.Background()
	ctx, cncl := context.WithCancel(ctx)
	defer cncl()

	cc := types.ConnectionConfig{
		URL:           svr.URL,
		Timeout:       1 * time.Second,
		BatchCount:    1,
		FlushInterval: 1 * time.Second,
		RetryBackoff:  100 * time.Millisecond,
		Connections:   1,
	}

	logger := log.NewNopLogger()
	wr, err := New(cc, logger, func(s types.NetworkStats) {}, func(s types.NetworkStats) {})
	require.NoError(t, err)
	wr.Start()
	defer wr.Stop()
	send(t, wr, ctx)

	require.Eventually(t, func() bool {
		done := retries.Load() > 5
		return done
	}, 10*time.Second, 1*time.Second)
}

func TestRetryBounded(t *testing.T) {

	sends := atomic.Uint32{}
	svr := httptest.NewServer(handler(t, http.StatusTooManyRequests, func(wr *prompb.WriteRequest) {
		sends.Add(1)
	}))

	defer svr.Close()
	ctx := context.Background()
	ctx, cncl := context.WithCancel(ctx)
	defer cncl()

	cc := types.ConnectionConfig{
		URL:              svr.URL,
		Timeout:          1 * time.Second,
		BatchCount:       1,
		FlushInterval:    1 * time.Second,
		RetryBackoff:     100 * time.Millisecond,
		MaxRetryAttempts: 1,
		Connections:      1,
	}

	logger := log.NewNopLogger()
	wr, err := New(cc, logger, func(s types.NetworkStats) {}, func(s types.NetworkStats) {})
	wr.Start()
	defer wr.Stop()
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		send(t, wr, ctx)
	}
	require.Eventually(t, func() bool {
		// We send 10 but each one gets retried once so 20 total.
		return sends.Load() == 10*2
	}, 2*time.Second, 100*time.Millisecond)
	time.Sleep(2 * time.Second)
	// Ensure we dont get any more.
	require.True(t, sends.Load() == 10*2)
}

func TestRecoverable(t *testing.T) {

	recoverable := atomic.Uint32{}
	svr := httptest.NewServer(handler(t, http.StatusInternalServerError, func(wr *prompb.WriteRequest) {
	}))
	defer svr.Close()
	ctx := context.Background()
	ctx, cncl := context.WithCancel(ctx)
	defer cncl()

	cc := types.ConnectionConfig{
		URL:              svr.URL,
		Timeout:          1 * time.Second,
		BatchCount:       1,
		FlushInterval:    1 * time.Second,
		RetryBackoff:     100 * time.Millisecond,
		MaxRetryAttempts: 1,
		Connections:      1,
	}

	logger := log.NewNopLogger()
	wr, err := New(cc, logger, func(s types.NetworkStats) {
		recoverable.Add(uint32(s.Total5XX()))
	}, func(s types.NetworkStats) {})
	require.NoError(t, err)
	wr.Start()
	defer wr.Stop()
	for i := 0; i < 10; i++ {
		send(t, wr, ctx)
	}
	require.Eventually(t, func() bool {
		// We send 10 but each one gets retried once so 20 total.
		return recoverable.Load() == 10*2
	}, 2*time.Second, 100*time.Millisecond)
	time.Sleep(2 * time.Second)
	// Ensure we dont get any more.
	require.True(t, recoverable.Load() == 10*2)
}

func TestNonRecoverable(t *testing.T) {

	nonRecoverable := atomic.Uint32{}
	svr := httptest.NewServer(handler(t, http.StatusBadRequest, func(wr *prompb.WriteRequest) {
	}))

	defer svr.Close()
	ctx := context.Background()
	ctx, cncl := context.WithCancel(ctx)
	defer cncl()

	cc := types.ConnectionConfig{
		URL:              svr.URL,
		Timeout:          1 * time.Second,
		BatchCount:       1,
		FlushInterval:    1 * time.Second,
		RetryBackoff:     100 * time.Millisecond,
		MaxRetryAttempts: 1,
		Connections:      1,
	}

	logger := log.NewNopLogger()
	wr, err := New(cc, logger, func(s types.NetworkStats) {
		nonRecoverable.Add(uint32(s.TotalFailed()))
	}, func(s types.NetworkStats) {})
	wr.Start()
	defer wr.Stop()
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		send(t, wr, ctx)
	}
	require.Eventually(t, func() bool {
		return nonRecoverable.Load() == 10
	}, 2*time.Second, 100*time.Millisecond)
	time.Sleep(2 * time.Second)
	// Ensure we dont get any more.
	require.True(t, nonRecoverable.Load() == 10)
}

func send(t *testing.T, wr types.NetworkClient, ctx context.Context) {
	ts := createSeries(t)
	// The actual hash is only used for queueing into different buckets.
	err := wr.SendSeries(ctx, ts)
	require.NoError(t, err)
}

func handler(t *testing.T, code int, callback func(wr *prompb.WriteRequest)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		defer r.Body.Close()
		decoded, err := snappy.Decode(nil, buf)
		require.NoError(t, err)

		wr := &prompb.WriteRequest{}
		err = wr.Unmarshal(decoded)
		require.NoError(t, err)
		callback(wr)
		w.WriteHeader(code)
	})
}

func createSeries(_ *testing.T) *types.TimeSeriesBinary {
	ts := &types.TimeSeriesBinary{
		TS:    time.Now().Unix(),
		Value: 1,
		Labels: []labels.Label{
			{
				Name:  "__name__",
				Value: randSeq(10),
			},
		},
	}
	return ts
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

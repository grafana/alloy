package networkqueue

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

type loop struct {
	mut        sync.RWMutex
	queue      chan []byte
	client     *http.Client
	batchCount int
	flushTimer time.Duration
	series     []prompb.TimeSeries
	cfg        ConnectionConfig
	pbuf       *proto.Buffer
	buf        []byte
}

func (l *loop) isFull() bool {
	l.mut.RLock()
	defer l.mut.RUnlock()

	return len(l.series) == l.batchCount
}

func (l *loop) runLoop(ctx context.Context) {
	for {
		t := time.NewTimer(l.flushTimer)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.mut.Lock()
			l.series = l.series[:0]
			l.mut.Unlock()
		case buf := <-l.queue:
			l.enqueue(buf)
		}
	}
}

func (l *loop) enqueue(buf []byte) {
	l.mut.RLock()
	defer l.mut.RUnlock()

	ts := tsPool.Get().(*prompb.TimeSeries)
	err := ts.Unmarshal(buf)
	if err != nil {
		return
	}
	l.series = append(l.series, *ts)
	if len(l.series) == l.batchCount {
		attempts := 0
	attempt:
		result := l.send(l.series, attempts)
		if result.successful {
			l.series = l.series[:0]
			return
		}
		if !result.recoverableError {
			l.series = l.series[:0]
			return
		}
		attempts++
		if attempts > int(l.cfg.MaxRetryBackoffAttempts) {
			l.series = l.series[:0]
			return
		}
		goto attempt
	}
}

type sendResult struct {
	err              error
	successful       bool
	recoverableError bool
	retryAfter       time.Duration
}

func (l *loop) send(series []prompb.TimeSeries, retryCount int) sendResult {
	result := sendResult{}
	l.pbuf.Reset()
	req := &prompb.WriteRequest{
		Timeseries: series,
	}
	err := l.pbuf.Marshal(req)
	if err != nil {
		result.err = err
		return result
	}
	l.buf = l.buf[:0]
	l.buf = snappy.Encode(l.buf, l.pbuf.Bytes())
	httpReq, err := http.NewRequest("POST", l.cfg.URL, bytes.NewReader(l.buf))
	if err != nil {
		result.err = err
		return result
	}
	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("User-Agent", l.cfg.UserAgent)
	httpReq.SetBasicAuth(l.cfg.Username, l.cfg.Password)

	if retryCount > 0 {
		httpReq.Header.Set("Retry-Attempt", strconv.Itoa(retryCount))
	}
	ctx := context.Background()
	ctx, cncl := context.WithTimeout(ctx, l.cfg.Timeout)
	defer cncl()
	resp, err := l.client.Do(httpReq.WithContext(ctx))
	// Network errors are recoverable.
	if err != nil {
		result.err = err
		result.recoverableError = true
		return result
	}
	if resp.StatusCode/100 == 5 || resp.StatusCode == http.StatusTooManyRequests {
		result.retryAfter = retryAfterDuration(resp.Header.Get("Retry-After"))
		result.recoverableError = true
		return result
	}
	if resp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, 1_000))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		result.err = fmt.Errorf("server returned HTTP status %s: %s", resp.Status, line)
		return result

	}

	result.successful = true
	return result
}

func retryAfterDuration(t string) time.Duration {
	parsedDuration, err := time.Parse(http.TimeFormat, t)
	if err == nil {
		return parsedDuration.Sub(time.Now().UTC())
	}
	// The duration can be in seconds.
	d, err := strconv.Atoi(t)
	if err != nil {
		return 5
	}
	return time.Duration(d) * time.Second
}

var tsPool = sync.Pool{
	New: func() any {
		return &prompb.TimeSeries{}
	},
}

func putInPool(ts *prompb.TimeSeries) {
	ts.Labels = ts.Labels[:0]
	ts.Samples = ts.Samples[:0]
	ts.Exemplars = ts.Exemplars[:0]
	ts.Histograms = ts.Histograms[:0]
	tsPool.Put(ts)
}

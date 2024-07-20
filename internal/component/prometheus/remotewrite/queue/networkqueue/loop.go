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

	"github.com/go-kit/log"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/grafana/alloy/internal/alloy/logging/level"
	"github.com/prometheus/prometheus/prompb"
	"golang.design/x/chann"
)

type loop struct {
	mut        sync.RWMutex
	client     *http.Client
	batchCount int
	flushTimer time.Duration
	cfg        ConnectionConfig
	pbuf       *proto.Buffer
	buf        []byte
	log        log.Logger
	lastSend   time.Time
	ch         *chann.Chann[[]byte]
	seriesBuf  []prompb.TimeSeries
}

func (l *loop) runLoop(ctx context.Context) {
	series := make([][]byte, 0)
	for {
		checkTime := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			return
		case <-checkTime.C:
			if len(series) == 0 {
				continue
			}
			if time.Since(l.lastSend) > l.flushTimer {
				l.trySend(series)
				series = series[:0]
			}
		case buf := <-l.ch.Out():
			series = append(series, buf)
			if len(series) >= l.batchCount {
				l.trySend(series)
				series = series[:0]
			}
		}
	}
}

func (l *loop) Push(ctx context.Context, buf []byte) bool {
	select {
	case l.ch.In() <- buf:
		return true
	case <-ctx.Done():
		return false
	}
}

func (l *loop) trySend(series [][]byte) {
	attempts := 0
attempt:
	level.Debug(l.log).Log("msg", "sending data", "attempts", attempts, "len", len(series))
	result := l.send(series, attempts)
	level.Debug(l.log).Log("msg", "sending data result", "attempts", attempts, "successful", result.successful, "err", result.err)
	if result.successful {
		l.resetSeries()
		return
	}
	if !result.recoverableError {
		l.resetSeries()
		return
	}
	attempts++
	if attempts > int(l.cfg.MaxRetryBackoffAttempts) && l.cfg.MaxRetryBackoffAttempts > 0 {
		level.Debug(l.log).Log("msg", "max attempts reached", "attempts", attempts)
		l.resetSeries()
		return
	}
	goto attempt
}

type sendResult struct {
	err              error
	successful       bool
	recoverableError bool
	retryAfter       time.Duration
}

func (l *loop) resetSeries() {
	l.lastSend = time.Now()
}

func (l *loop) send(series [][]byte, retryCount int) sendResult {
	result := sendResult{}
	l.pbuf.Reset()
	l.seriesBuf = l.seriesBuf[:0]
	for _, tsBuf := range series {
		ts := prompb.TimeSeries{}
		err := proto.Unmarshal(tsBuf, &ts)
		if err != nil {
			continue
		}
		l.seriesBuf = append(l.seriesBuf, ts)
	}
	req := &prompb.WriteRequest{
		Timeseries: l.seriesBuf,
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

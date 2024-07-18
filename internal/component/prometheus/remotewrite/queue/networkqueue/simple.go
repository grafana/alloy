package networkqueue

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

type simple struct {
	queues          []chan (*prompb.TimeSeries)
	connectionCount int
}

// start kicks off a number of concurrent connections.
func (s *simple) start(ctx context.Context) error {
	for i := 0; i < s.connectionCount; i++ {
		go func() {
			for {
			}
		}()
	}
}

type loop struct {
	queue      chan *prompb.TimeSeries
	client     *http.Client
	batchCount int
	flushTimer time.Duration
}

func (l *loop) runLoop(ctx context.Context) {
	series := make([]*prompb.TimeSeries, 0)
	for {
		t := time.NewTimer(l.flushTimer)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			series = series[:0]

		case ts := <-l.queue:
			series = append(series, ts)
			if len(series) > l.batchCount {
				success := l.send(series)
				if !success {
					continue
				}
				series = series[:0]
			}

		}
	}
}

func (l *loop) send(series []*prompb.TimeSeries) bool {
	// Need to send here with retries.
}

package networkqueue

import (
	"context"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/types"
	"github.com/prometheus/prometheus/prompb"
)

type simple struct {
	connectionCount int
	loops           []*loop
}

var _ types.WriteClient = (*simple)(nil)

type ConnectionConfig struct {
	URL                     string
	Username                string
	Password                string
	UserAgent               string
	Timeout                 time.Duration
	RetryBackoff            time.Duration
	MaxRetryBackoffAttempts time.Duration
	BatchCount              int
	FlushDuration           time.Duration
}

func New(ctx context.Context, cc ConnectionConfig, connectionCount int) (*simple, error) {
	s := &simple{
		connectionCount: connectionCount,
		loops:           make([]*loop, 0),
	}

	// start kicks off a number of concurrent connections.
	for i := 0; i < s.connectionCount; i++ {
		l := &loop{
			queue:      make(chan []byte),
			batchCount: cc.BatchCount,
			flushTimer: cc.FlushDuration,
			series:     make([]prompb.TimeSeries, 0),
			client:     &http.Client{},
			cfg:        cc,
			pbuf:       proto.NewBuffer(nil),
			buf:        make([]byte, 0),
		}
		s.loops = append(s.loops, l)
		go l.runLoop(ctx)
	}
	return s, nil
}

func (s *simple) Queue(hash int64, buf []byte) bool {
	queueNum := hash % int64(s.connectionCount)
	if s.loops[queueNum].isFull() {
		return false
	}
	s.loops[queueNum].queue <- buf
	return true
}

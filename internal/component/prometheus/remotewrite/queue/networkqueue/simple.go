package networkqueue

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/prompb"
	"golang.design/x/chann"

	"github.com/gogo/protobuf/proto"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/types"
)

type simple struct {
	mut             sync.Mutex
	connectionCount uint64
	loops           []*loop
	metadata        *loop
	logger          log.Logger
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

func New(ctx context.Context, cc ConnectionConfig, connectionCount uint64, logger log.Logger) (types.WriteClient, error) {
	s := &simple{
		connectionCount: connectionCount,
		loops:           make([]*loop, 0),
		logger:          logger,
	}

	// start kicks off a number of concurrent connections.
	var i uint64
	for ; i < s.connectionCount; i++ {
		l := &loop{
			batchCount: cc.BatchCount,
			flushTimer: cc.FlushDuration,
			client:     &http.Client{},
			cfg:        cc,
			pbuf:       proto.NewBuffer(nil),
			buf:        make([]byte, 0),
			log:        logger,
			ch:         chann.New[[]byte](chann.Cap(cc.BatchCount * 2)),
			seriesBuf:  make([]prompb.TimeSeries, 0),
		}
		s.loops = append(s.loops, l)
		go l.runLoop(ctx)
	}
	return s, nil
}

func (s *simple) Queue(ctx context.Context, hash uint64, buf []byte) bool {
	queueNum := hash % s.connectionCount

	return s.loops[queueNum].Push(ctx, buf)
}

func (s *simple) QueueMetadata(ctx context.Context, buf []byte) bool {
	return s.metadata.Push(ctx, buf)
}

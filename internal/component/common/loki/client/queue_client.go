package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"

	"github.com/grafana/alloy/internal/component/common/loki"
	alloyWal "github.com/grafana/alloy/internal/component/common/loki/wal"
	"github.com/grafana/dskit/backoff"

	"github.com/grafana/loki/v3/pkg/ingester/wal"
)

// StoppableWriteTo is a mixing of the WAL's WriteTo interface, that is Stoppable as well.
type StoppableWriteTo interface {
	alloyWal.WriteTo
	Stop()
}

// MarkerHandler re-defines the interface of internal.MarkerHandler that the queue client interacts with, to contribute
// to the feedback loop of when data from a segment is read from the WAL, or delivered.
type MarkerHandler interface {
	UpdateReceivedData(segmentId, dataCount int) // Data queued for sending
	UpdateSentData(segmentId, dataCount int)     // Data which was sent or given up on sending
	Stop()
}

// queueClient is a WAL-specific remote write client implementation. This client attests to the wal.WriteTo interface,
// which allows it to be injected in the wal.Watcher as a destination where to write read series and entries. As the watcher
// reads from the WAL, batches are created and dispatched onto a send queue when ready to be sent.
type queueClient struct {
	qcMetrics *QueueClientMetrics
	logger    log.Logger
	cfg       Config
	shards    *shards

	ctx    context.Context
	cancel context.CancelFunc

	// series cache
	series        map[chunks.HeadSeriesRef]model.LabelSet
	seriesSegment map[chunks.HeadSeriesRef]int
	seriesLock    sync.RWMutex

	markerHandler MarkerHandler
}

// NewQueue creates a new queueClient.
func NewQueue(metrics *Metrics, queueClientMetrics *QueueClientMetrics, cfg Config, logger log.Logger, markerHandler MarkerHandler) (StoppableWriteTo, error) {
	return newQueueClient(metrics, queueClientMetrics, cfg, logger, markerHandler)
}

func newQueueClient(metrics *Metrics, qcMetrics *QueueClientMetrics, cfg Config, logger log.Logger, markerHandler MarkerHandler) (*queueClient, error) {
	logger = log.With(logger, "component", "client", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, markerHandler, cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &queueClient{
		logger:    log.With(logger, "component", "client", "host", cfg.URL.Host),
		cfg:       cfg,
		qcMetrics: qcMetrics,
		shards:    shards,

		ctx:    ctx,
		cancel: cancel,

		series:        make(map[chunks.HeadSeriesRef]model.LabelSet),
		seriesSegment: make(map[chunks.HeadSeriesRef]int),

		markerHandler: markerHandler,
	}

	// FIXME: resharding loop and better place for this
	c.shards.start(1)

	return c, nil
}

func (c *queueClient) SeriesReset(segmentNum int) {
	c.seriesLock.Lock()
	defer c.seriesLock.Unlock()
	for k, v := range c.seriesSegment {
		if v <= segmentNum {
			level.Debug(c.logger).Log("msg", fmt.Sprintf("reclaiming series under segment %d", segmentNum))
			delete(c.seriesSegment, k)
			delete(c.series, k)
		}
	}
}

func (c *queueClient) StoreSeries(series []record.RefSeries, segment int) {
	c.seriesLock.Lock()
	defer c.seriesLock.Unlock()
	for _, seriesRec := range series {
		c.seriesSegment[seriesRec.Ref] = segment
		c.series[seriesRec.Ref] = promLabelsToModelLabels(seriesRec.Labels)
	}
}

func (c *queueClient) AppendEntries(entries wal.RefEntries, segment int) error {
	c.seriesLock.RLock()
	l, ok := c.series[entries.Ref]
	c.seriesLock.RUnlock()
	var maxSeenTimestamp int64 = -1
	if ok {
		for _, e := range entries.Entries {
			ok := c.appendSingleEntry(loki.Entry{Labels: l, Entry: e}, segment)
			if !ok {
				return nil
			}

			if e.Timestamp.Unix() > maxSeenTimestamp {
				maxSeenTimestamp = e.Timestamp.Unix()
			}
		}
		// count all enqueued appended entries as received from WAL
		c.markerHandler.UpdateReceivedData(segment, len(entries.Entries))
	} else {
		// TODO(thepalbi): Add metric here
		level.Debug(c.logger).Log("msg", "series for entry not found")
	}

	// It's safe to assume that upon an AppendEntries call, there will always be at least
	// one entry.
	c.qcMetrics.lastReadTimestamp.WithLabelValues().Set(float64(maxSeenTimestamp))

	return nil
}

func (c *queueClient) appendSingleEntry(entry loki.Entry, segmentNum int) bool {
	backoff := backoff.New(c.ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 50 * time.Millisecond,
	})
	for {
		if c.shards.enqueue(entry, segmentNum) {
			return true
		}

		if !backoff.Ongoing() {
			// we could not enqueue and client is stopped.
			return false
		}
	}
}

// Stop the client, enqueueing pending batches and draining the send queue accordingly. Both closing operations are
// limited by a deadline, controlled by a configured drain timeout, which is global to the Stop call.
func (c *queueClient) Stop() {
	// drain shards
	c.shards.stop()
	c.markerHandler.Stop()
}

package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"

	alloyWal "github.com/grafana/alloy/internal/component/common/loki/wal"

	"github.com/grafana/loki/v3/pkg/ingester/wal"
	"github.com/grafana/loki/v3/pkg/logproto"
)

// StoppableWriteTo is a mixing of the WAL's WriteTo interface, that is Stoppable as well.
type StoppableWriteTo interface {
	alloyWal.WriteTo
	Stop()
	StopNow()
}

// MarkerHandler re-defines the interface of internal.MarkerHandler that the queue client interacts with, to contribute
// to the feedback loop of when data from a segment is read from the WAL, or delivered.
type MarkerHandler interface {
	UpdateReceivedData(segmentId, dataCount int) // Data queued for sending
	UpdateSentData(segmentId, dataCount int)     // Data which was sent or given up on sending
	Stop()
}

// queuedBatch is a batch specific to a tenant, that is considered ready to be sent.
type queuedBatch struct {
	TenantID string
	Batch    *batch
}

// queue wraps a buffered channel and a routine that reads from it, sending batches of entries.
type queue struct {
	client *queueClient
	q      chan queuedBatch
	quit   chan struct{}
	wg     sync.WaitGroup
	logger log.Logger
}

func newQueue(client *queueClient, size int, logger log.Logger) *queue {
	q := queue{
		client: client,
		q:      make(chan queuedBatch, size),
		quit:   make(chan struct{}),
		logger: logger,
	}

	q.wg.Add(1)
	go q.run()

	return &q
}

// enqueue adds to the send queue a batch ready to be sent. Note that if the backing queue is has no
// remaining capacity to enqueue the batch, calling enqueue might block.
func (q *queue) enqueue(qb queuedBatch) {
	q.q <- qb
}

// enqueueWithCancel tries to enqueue a batch, giving up if the supplied context times deadlines
// times out. If the batch is successfully enqueued, it returns true.
func (q *queue) enqueueWithCancel(ctx context.Context, qb queuedBatch) bool {
	select {
	case <-ctx.Done():
		return false
	case q.q <- qb:
	}
	return true
}

func (q *queue) run() {
	defer q.wg.Done()

	for {
		select {
		case <-q.quit:
			return
		case qb := <-q.q:
			// Since inside the actual send operation a context with time out is used, we should exceed that timeout
			// instead of cancelling this send operation, since that batch has been taken out of the queue.
			q.sendAndReport(context.Background(), qb.TenantID, qb.Batch)
		}
	}
}

// closeAndDrain stops gracefully the queue. The process first stops the main routine that reads batches to be sent,
// to instead drain the queue and send those batches from this thread, exiting if the supplied context deadline
// is exceeded. Also, if the underlying buffered channel is fully drain, this will exit promptly.
func (q *queue) closeAndDrain(ctx context.Context) {
	// defer main channel closing
	defer close(q.q)

	// first stop main routine, and wait for it to signal
	close(q.quit)
	q.wg.Wait()

	// keep reading messages from sendQueue until all have been consumed, or timeout is exceeded
	for {
		select {
		case qb := <-q.q:
			// drain uses the same timeout, so if a timeout was applied to the parent context, it can cancel the underlying
			// send operation preemptively.
			q.sendAndReport(ctx, qb.TenantID, qb.Batch)
		case <-ctx.Done():
			level.Warn(q.logger).Log("msg", "timeout exceeded while draining send queue")
			return
		default:
			level.Debug(q.logger).Log("msg", "drain queue exited because there were no batches left to send")
			return
			// if default clause is taken, it means there's nothing left in the send queue
		}
	}
}

// sendAndReport attempts to send the batch for the given tenant, and either way that operation succeeds or fails, reports
// the data as sent.
func (q *queue) sendAndReport(ctx context.Context, tenantId string, b *batch) {
	q.client.bc.sendBatch(ctx, tenantId, b)
	// mark segment data for that batch as sent, even if the send operation failed
	b.reportAsSentData(q.client.markerHandler)
}

// closeNow closes the queue, without draining batches that might be buffered to be sent.
func (q *queue) closeNow() {
	close(q.quit)
	q.wg.Wait()
	close(q.q)
}

// queueClient is a WAL-specific remote write client implementation. This client attests to the wal.WriteTo interface,
// which allows it to be injected in the wal.Watcher as a destination where to write read series and entries. As the watcher
// reads from the WAL, batches are created and dispatched onto a send queue when ready to be sent.
type queueClient struct {
	metrics   *Metrics
	qcMetrics *QueueClientMetrics
	logger    log.Logger
	cfg       Config
	bc        *batchClient

	batches      map[string]*batch
	batchesMtx   sync.Mutex
	sendQueue    *queue
	drainTimeout time.Duration

	wg sync.WaitGroup

	// series cache
	series        map[chunks.HeadSeriesRef]model.LabelSet
	seriesSegment map[chunks.HeadSeriesRef]int
	seriesLock    sync.RWMutex

	quit          chan struct{}
	markerHandler MarkerHandler
}

// NewQueue creates a new queueClient.
func NewQueue(metrics *Metrics, queueClientMetrics *QueueClientMetrics, cfg Config, logger log.Logger, markerHandler MarkerHandler) (StoppableWriteTo, error) {
	return newQueueClient(metrics, queueClientMetrics, cfg, logger, markerHandler)
}

func newQueueClient(metrics *Metrics, qcMetrics *QueueClientMetrics, cfg Config, logger log.Logger, markerHandler MarkerHandler) (*queueClient, error) {
	logger = log.With(logger, "component", "client", "host", cfg.URL.Host)

	bc, err := newBatchClient(metrics, logger, cfg)
	if err != nil {
		return nil, err
	}

	c := &queueClient{
		logger:       log.With(logger, "component", "client", "host", cfg.URL.Host),
		cfg:          cfg,
		metrics:      metrics,
		qcMetrics:    qcMetrics,
		bc:           bc,
		drainTimeout: cfg.Queue.DrainTimeout,
		quit:         make(chan struct{}),

		batches:       make(map[string]*batch),
		markerHandler: markerHandler,

		series:        make(map[chunks.HeadSeriesRef]model.LabelSet),
		seriesSegment: make(map[chunks.HeadSeriesRef]int),
	}

	// The buffered channel size is calculated using the configured capacity, which is the worst case number of bytes
	// the send queue can consume.
	var queueBufferSize = cfg.Queue.Capacity / cfg.BatchSize
	c.sendQueue = newQueue(c, queueBufferSize, logger)

	c.wg.Add(1)
	go c.runSendOldBatches()
	return c, nil
}

func (c *queueClient) initBatchMetrics(tenantID string) {
	// Initialize counters to 0 so the metrics are exported before the first
	// occurrence of incrementing to avoid missing metrics.
	for _, counter := range c.metrics.countersWithHostTenantReason {
		for _, reason := range Reasons {
			counter.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Add(0)
		}
	}

	for _, counter := range c.metrics.countersWithHostTenant {
		counter.WithLabelValues(c.cfg.URL.Host, tenantID).Add(0)
	}
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
			c.appendSingleEntry(segment, l, e)
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

func (c *queueClient) appendSingleEntry(segmentNum int, lbs model.LabelSet, e logproto.Entry) {
	lbs, tenantID := c.processLabels(lbs)

	// TODO: can I make this locking more fine grained?
	c.batchesMtx.Lock()

	batch, ok := c.batches[tenantID]

	// If the batch doesn't exist yet, we create a new one with the entry
	if !ok {
		nb := newBatch(c.cfg.MaxStreams)
		// since the batch is new, adding a new entry, and hence a new stream, won't fail since there aren't any stream
		// registered in the batch.
		_ = nb.addFromWAL(lbs, e, segmentNum)

		c.batches[tenantID] = nb
		c.batchesMtx.Unlock()

		c.initBatchMetrics(tenantID)
		return
	}

	// If adding the entry to the batch will increase the size over the max
	// size allowed, we do send the current batch and then create a new one
	if batch.sizeBytesAfter(e) > c.cfg.BatchSize {
		c.sendQueue.enqueue(queuedBatch{
			TenantID: tenantID,
			Batch:    batch,
		})

		nb := newBatch(c.cfg.MaxStreams)
		_ = nb.addFromWAL(lbs, e, segmentNum)
		c.batches[tenantID] = nb
		c.batchesMtx.Unlock()

		return
	}

	// The max size of the batch isn't reached, so we can add the entry
	err := batch.addFromWAL(lbs, e, segmentNum)
	c.batchesMtx.Unlock()

	if err != nil {
		level.Error(c.logger).Log("msg", "batch add err", "tenant", tenantID, "error", err)
		reason := ReasonGeneric
		if errors.Is(err, errMaxStreamsLimitExceeded) {
			reason = ReasonStreamLimited
		}
		c.metrics.droppedBytes.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Add(float64(len(e.Line)))
		c.metrics.droppedEntries.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Inc()
	}
}

func (c *queueClient) runSendOldBatches() {
	// Given the client handles multiple batches (1 per tenant) and each batch
	// can be created at a different point in time, we look for batches whose
	// max wait time has been reached every 10 times per BatchWait, so that the
	// maximum delay we have sending batches is 10% of the max waiting time.
	// We apply a cap of 10ms to the ticker, to avoid too frequent checks in
	// case the BatchWait is very low.
	minWaitCheckFrequency := 10 * time.Millisecond
	maxWaitCheckFrequency := max(c.cfg.BatchWait/10, minWaitCheckFrequency)

	maxWaitCheck := time.NewTicker(maxWaitCheckFrequency)

	// pablo: maybe this should be moved out
	defer func() {
		maxWaitCheck.Stop()
		c.wg.Done()
	}()

	var batchesToFlush []queuedBatch

	for {
		select {
		case <-c.quit:
			return

		case <-maxWaitCheck.C:
			c.batchesMtx.Lock()
			// Send all batches whose max wait time has been reached
			for tenantID, b := range c.batches {
				if b.age() < c.cfg.BatchWait {
					continue
				}

				// add to batches to flush, so we can enqueue them later and release the batches lock
				// as early as possible
				batchesToFlush = append(batchesToFlush, queuedBatch{
					TenantID: tenantID,
					Batch:    b,
				})

				// deleting assuming that since the batch expired the wait time, it
				// hasn't been written for some time
				delete(c.batches, tenantID)
			}

			c.batchesMtx.Unlock()

			// enqueue batches that were marked as too old
			for _, qb := range batchesToFlush {
				c.sendQueue.enqueue(qb)
			}

			batchesToFlush = batchesToFlush[:0] // renew slide
		}
	}
}

// enqueuePendingBatches will go over the pending batches, and enqueue them in the send queue. If the context's
// deadline is exceeded in any enqueue operation, this routine exits.
func (c *queueClient) enqueuePendingBatches(ctx context.Context) {
	c.batchesMtx.Lock()
	defer c.batchesMtx.Unlock()

	for tenantID, batch := range c.batches {
		if !c.sendQueue.enqueueWithCancel(ctx, queuedBatch{
			TenantID: tenantID,
			Batch:    batch,
		}) {
			// if enqueue times out due to the context timing out, cancel all
			return
		}
	}
}

func (c *queueClient) getTenantID(labels model.LabelSet) string {
	// Check if it has been overridden while processing the pipeline stages
	if value, ok := labels[ReservedLabelTenantID]; ok {
		return string(value)
	}

	// Check if has been specified in the config
	if c.cfg.TenantID != "" {
		return c.cfg.TenantID
	}

	// Defaults to an empty string, which means the X-Scope-OrgID header
	// will not be sent
	return ""
}

// Stop the client, enqueueing pending batches and draining the send queue accordingly. Both closing operations are
// limited by a deadline, controlled by a configured drain timeout, which is global to the Stop call.
func (c *queueClient) Stop() {
	// first close main queue routine
	close(c.quit)
	c.wg.Wait()

	// fire timeout timer
	ctx, cancel := context.WithTimeout(context.Background(), c.drainTimeout)
	defer cancel()

	// enqueue batches that might be pending in the batches map
	c.enqueuePendingBatches(ctx)

	// drain sendQueue with timeout in context
	c.sendQueue.closeAndDrain(ctx)

	// stop request after drain times out or exits
	c.bc.stop()

	c.markerHandler.Stop()
}

// StopNow stops the client without retries or draining the send queue
func (c *queueClient) StopNow() {
	// stop batch client from retrying requests.
	c.bc.stop()
	close(c.quit)
	c.sendQueue.closeNow()
	c.wg.Wait()
	c.markerHandler.Stop()
}

func (c *queueClient) processLabels(lbs model.LabelSet) (model.LabelSet, string) {
	tenantID := c.getTenantID(lbs)
	return lbs, tenantID
}

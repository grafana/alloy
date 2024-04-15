// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package batch

import (
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/atomic"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/scrape"
)

const (
	// We track samples in/out and how long pushes take using an Exponentially
	// Weighted Moving Average.
	ewmaWeight          = 0.2
	shardUpdateDuration = 10 * time.Second

	// Allow 30% too many shards before scaling down.
	shardToleranceFraction = 0.3
)

// WriteClient defines an interface for sending a batch of samples to an
// external timeseries database.
type WriteClient interface {
	// Store stores the given samples in the remote storage.
	Store(context.Context, []byte) error
	// Name uniquely identifies the remote storage.
	Name() string
	// Endpoint is the remote read or write endpoint for the storage client.
	Endpoint() string
}

// QueueManager manages a queue of samples to be sent to the Storage
// indicated by the provided WriteClient. Implements writeTo interface
// used by WAL Watcher.
type QueueManager struct {
	lastSendTimestamp atomic.Int64

	logger               log.Logger
	flushDeadline        time.Duration
	cfg                  QueueOptions
	mcfg                 MetadataOptions
	sendExemplars        bool
	sendNativeHistograms bool

	clientMtx   sync.RWMutex
	storeClient WriteClient

	seriesMtx sync.Mutex // Covers seriesLabels and droppedSeries.

	shards      *shards
	numShards   int
	reshardChan chan int
	quit        chan struct{}
	wg          sync.WaitGroup

	dataIn, dataDropped, dataOut, dataOutDuration *ewmaRate

	metrics              *queueManagerMetrics
	highestRecvTimestamp *maxTimestamp
}

// NewQueueManager builds a new QueueManager and starts a new
// WAL watcher with queue manager as the WriteTo destination.
// The WAL watcher takes the dir parameter as the base directory
// for where the WAL shall be located. Note that the full path to
// the WAL directory will be constructed as <dir>/wal.
func NewQueueManager(
	r prometheus.Registerer,
	logger log.Logger,
	cfg QueueOptions,
	mCfg MetadataOptions,
	client WriteClient,
	flushDeadline time.Duration,
	highestRecvTimestamp *maxTimestamp,
	enableExemplarRemoteWrite bool,
	enableNativeHistogramRemoteWrite bool,
) *QueueManager {

	if logger == nil {
		logger = log.NewNopLogger()
	}

	logger = log.With(logger, remoteName, client.Name(), endpoint, client.Endpoint())
	t := &QueueManager{
		logger:               logger,
		flushDeadline:        flushDeadline,
		cfg:                  cfg,
		mcfg:                 mCfg,
		storeClient:          client,
		sendExemplars:        enableExemplarRemoteWrite,
		sendNativeHistograms: enableNativeHistogramRemoteWrite,

		numShards:   cfg.MinShards,
		reshardChan: make(chan int),
		quit:        make(chan struct{}),

		dataIn:          newEWMARate(ewmaWeight, shardUpdateDuration),
		dataDropped:     newEWMARate(ewmaWeight, shardUpdateDuration),
		dataOut:         newEWMARate(ewmaWeight, shardUpdateDuration),
		dataOutDuration: newEWMARate(ewmaWeight, shardUpdateDuration),

		metrics:              newQueueManagerMetrics(r, client.Name(), client.Endpoint()),
		highestRecvTimestamp: highestRecvTimestamp,
	}

	t.shards = t.newShards()

	return t
}

// AppendMetadata sends metadata to the remote storage. Metadata is sent in batches, but is not parallelized.
func (t *QueueManager) AppendMetadata(ctx context.Context, metadata []scrape.MetricMetadata) {
	mm := make([]prompb.MetricMetadata, 0, len(metadata))
	for _, entry := range metadata {
		mm = append(mm, prompb.MetricMetadata{
			MetricFamilyName: entry.Metric,
			Help:             entry.Help,
			Type:             metricTypeToMetricTypeProto(entry.Type),
			Unit:             entry.Unit,
		})
	}

	pBuf := proto.NewBuffer(nil)
	numSends := int(math.Ceil(float64(len(metadata)) / float64(t.mcfg.MaxSamplesPerSend)))
	for i := 0; i < numSends; i++ {
		last := (i + 1) * t.mcfg.MaxSamplesPerSend
		if last > len(metadata) {
			last = len(metadata)
		}
		err := t.sendMetadataWithBackoff(ctx, mm[i*t.mcfg.MaxSamplesPerSend:last], pBuf)
		if err != nil {
			t.metrics.failedMetadataTotal.Add(float64(last - (i * t.mcfg.MaxSamplesPerSend)))
			level.Error(t.logger).Log("msg", "non-recoverable error while sending metadata", "count", last-(i*t.mcfg.MaxSamplesPerSend), "err", err)
		}
	}
}

func (t *QueueManager) sendMetadataWithBackoff(ctx context.Context, metadata []prompb.MetricMetadata, pBuf *proto.Buffer) error {
	// Build the WriteRequest with no samples.
	req, _, err := buildWriteRequest(nil, metadata, pBuf, nil)
	if err != nil {
		return err
	}

	metadataCount := len(metadata)

	attemptStore := func(try int) error {
		ctx, span := otel.Tracer("").Start(ctx, "Remote Metadata Send Batch")
		defer span.End()

		span.SetAttributes(
			attribute.Int("metadata", metadataCount),
			attribute.Int("try", try),
			attribute.String("remote_name", t.storeClient.Name()),
			attribute.String("remote_url", t.storeClient.Endpoint()),
		)

		begin := time.Now()
		err := t.storeClient.Store(ctx, req)
		t.metrics.sentBatchDuration.Observe(time.Since(begin).Seconds())

		if err != nil {
			span.RecordError(err)
			return err
		}

		return nil
	}

	retry := func() {
		t.metrics.retriedMetadataTotal.Add(float64(len(metadata)))
	}
	err = sendWriteRequestWithBackoff(ctx, t.cfg, t.logger, attemptStore, retry)
	if err != nil {
		return err
	}
	t.metrics.metadataTotal.Add(float64(len(metadata)))
	t.metrics.metadataBytesTotal.Add(float64(len(req)))
	return nil
}

// Append queues a sample to be sent to the remote storage. Blocks until all samples are
// enqueued on their shards or a shutdown signal is received.
func (t *QueueManager) Append(samples []*TimeSeries) bool {
outer:
	for _, s := range samples {
		// Start with a very small backoff. This should not be t.cfg.MinBackoff
		// as it can happen without errors, and we want to pickup work after
		// filling a queue/resharding as quickly as possible.
		// TODO: Consider using the average duration of a request as the backoff.
		backoff := model.Duration(5 * time.Millisecond)
		for {
			select {
			case <-t.quit:
				return false
			default:
			}
			if t.shards.enqueue(s) {
				continue outer
			}

			t.metrics.enqueueRetriesTotal.Inc()
			time.Sleep(time.Duration(backoff))
			backoff *= 2
			// It is reasonable to use t.cfg.MaxBackoff here, as if we have hit
			// the full backoff we are likely waiting for external resources.
			if backoff > model.Duration(t.cfg.MaxBackoff) {
				backoff = model.Duration(t.cfg.MaxBackoff)
			}
		}
	}
	return true
}

func (t *QueueManager) AppendExemplars(exemplars []*TimeSeries) bool {
	if !t.sendExemplars {
		return true
	}

outer:
	for _, e := range exemplars {
		// This will only loop if the queues are being resharded.
		backoff := t.cfg.MinBackoff
		for {
			select {
			case <-t.quit:
				return false
			default:
			}
			if t.shards.enqueue(e) {
				continue outer
			}

			t.metrics.enqueueRetriesTotal.Inc()
			time.Sleep(time.Duration(backoff))
			backoff *= 2
			if backoff > t.cfg.MaxBackoff {
				backoff = t.cfg.MaxBackoff
			}
		}
	}
	return true
}

func (t *QueueManager) AppendHistograms(histograms []*TimeSeries) bool {
	if !t.sendNativeHistograms {
		return true
	}

outer:
	for _, h := range histograms {
		backoff := model.Duration(5 * time.Millisecond)
		for {
			select {
			case <-t.quit:
				return false
			default:
			}
			if t.shards.enqueue(h) {
				continue outer
			}

			t.metrics.enqueueRetriesTotal.Inc()
			time.Sleep(time.Duration(backoff))
			backoff *= 2
			if backoff > model.Duration(t.cfg.MaxBackoff) {
				backoff = model.Duration(t.cfg.MaxBackoff)
			}
		}
	}
	return true
}

func (t *QueueManager) AppendFloatHistograms(floatHistograms []*TimeSeries) bool {
	if !t.sendNativeHistograms {
		return true
	}

outer:
	for _, h := range floatHistograms {
		backoff := model.Duration(5 * time.Millisecond)
		for {
			select {
			case <-t.quit:
				return false
			default:
			}
			if t.shards.enqueue(h) {
				continue outer
			}

			t.metrics.enqueueRetriesTotal.Inc()
			time.Sleep(time.Duration(backoff))
			backoff *= 2
			if backoff > model.Duration(t.cfg.MaxBackoff) {
				backoff = model.Duration(t.cfg.MaxBackoff)
			}
		}
	}
	return true
}

// Start the queue manager sending samples to the remote storage.
// Does not block.
func (t *QueueManager) Start(started chan struct{}) {
	// Register and initialise some metrics.
	t.metrics.register()
	t.metrics.shardCapacity.Set(float64(t.cfg.Capacity))
	t.metrics.maxNumShards.Set(float64(t.cfg.MaxShards))
	t.metrics.minNumShards.Set(float64(t.cfg.MinShards))
	t.metrics.desiredNumShards.Set(float64(t.cfg.MinShards))
	t.metrics.maxSamplesPerSend.Set(float64(t.cfg.MaxSamplesPerSend))

	t.shards.start(t.numShards)

	t.wg.Add(2)
	go t.updateShardsLoop()
	go t.reshardLoop()
	started <- struct{}{}

}

// Stop stops sending samples to the remote storage and waits for pending
// sends to complete.
func (t *QueueManager) Stop() {
	level.Info(t.logger).Log("msg", "Stopping remote storage...")
	defer level.Info(t.logger).Log("msg", "Remote storage stopped.")

	close(t.quit)
	t.wg.Wait()
	// Wait for all QueueManager routines to end before stopping shards, metadata watcher, and WAL watcher. This
	// is to ensure we don't end up executing a reshard and shards.stop() at the same time, which
	// causes a closed channel panic.
	t.shards.stop()

	// On shutdown, release the strings in the labels from the intern pool.
	t.seriesMtx.Unlock()
}

// SetClient updates the client used by a queue. Used when only client specific
// fields are updated to avoid restarting the queue.
func (t *QueueManager) SetClient(c WriteClient) {
	t.clientMtx.Lock()
	t.storeClient = c
	t.clientMtx.Unlock()
}

func (t *QueueManager) client() WriteClient {
	t.clientMtx.RLock()
	defer t.clientMtx.RUnlock()
	return t.storeClient
}

func (t *QueueManager) updateShardsLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(shardUpdateDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			desiredShards := t.calculateDesiredShards()
			if !t.shouldReshard(desiredShards) {
				continue
			}
			// Resharding can take some time, and we want this loop
			// to stay close to shardUpdateDuration.
			select {
			case t.reshardChan <- desiredShards:
				level.Info(t.logger).Log("msg", "Remote storage resharding", "from", t.numShards, "to", desiredShards)
				t.numShards = desiredShards
			default:
				level.Info(t.logger).Log("msg", "Currently resharding, skipping.")
			}
		case <-t.quit:
			return
		}
	}
}

// shouldReshard returns whether resharding should occur.
func (t *QueueManager) shouldReshard(desiredShards int) bool {
	if desiredShards == t.numShards {
		return false
	}
	// We shouldn't reshard if Prometheus hasn't been able to send to the
	// remote endpoint successfully within some period of time.
	minSendTimestamp := time.Now().Add(-2 * time.Duration(t.cfg.BatchSendDeadline)).Unix()
	lsts := t.lastSendTimestamp.Load()
	if lsts < minSendTimestamp {
		level.Warn(t.logger).Log("msg", "Skipping resharding, last successful send was beyond threshold", "lastSendTimestamp", lsts, "minSendTimestamp", minSendTimestamp)
		return false
	}
	return true
}

// calculateDesiredShards returns the number of desired shards, which will be
// the current QueueManager.numShards if resharding should not occur for reasons
// outlined in this functions implementation. It is up to the caller to reshard, or not,
// based on the return value.
func (t *QueueManager) calculateDesiredShards() int {
	t.dataOut.tick()
	t.dataDropped.tick()
	t.dataOutDuration.tick()

	// We use the number of incoming samples as a prediction of how much work we
	// will need to do next iteration.  We add to this any pending samples
	// (received - send) so we can catch up with any backlog. We use the average
	// outgoing batch latency to work out how many shards we need.
	var (
		dataInRate      = t.dataIn.rate()
		dataOutRate     = t.dataOut.rate()
		dataKeptRatio   = dataOutRate / (t.dataDropped.rate() + dataOutRate)
		dataOutDuration = t.dataOutDuration.rate() / float64(time.Second)
		dataPendingRate = dataInRate*dataKeptRatio - dataOutRate
		highestSent     = t.metrics.highestSentTimestamp.Get()
		highestRecv     = t.highestRecvTimestamp.Get()
		delay           = highestRecv - highestSent
		dataPending     = delay * dataInRate * dataKeptRatio
	)

	if dataOutRate <= 0 {
		return t.numShards
	}

	var (
		// When behind we will try to catch up on 5% of samples per second.
		backlogCatchup = 0.05 * dataPending
		// Calculate Time to send one sample, averaged across all sends done this tick.
		timePerSample = dataOutDuration / dataOutRate
		desiredShards = timePerSample * (dataInRate*dataKeptRatio + backlogCatchup)
	)
	t.metrics.desiredNumShards.Set(desiredShards)
	level.Debug(t.logger).Log("msg", "QueueManager.calculateDesiredShards",
		"dataInRate", dataInRate,
		"dataOutRate", dataOutRate,
		"dataKeptRatio", dataKeptRatio,
		"dataPendingRate", dataPendingRate,
		"dataPending", dataPending,
		"dataOutDuration", dataOutDuration,
		"timePerSample", timePerSample,
		"desiredShards", desiredShards,
		"highestSent", highestSent,
		"highestRecv", highestRecv,
	)

	// Changes in the number of shards must be greater than shardToleranceFraction.
	var (
		lowerBound = float64(t.numShards) * (1. - shardToleranceFraction)
		upperBound = float64(t.numShards) * (1. + shardToleranceFraction)
	)
	level.Debug(t.logger).Log("msg", "QueueManager.updateShardsLoop",
		"lowerBound", lowerBound, "desiredShards", desiredShards, "upperBound", upperBound)

	desiredShards = math.Ceil(desiredShards) // Round up to be on the safe side.
	if lowerBound <= desiredShards && desiredShards <= upperBound {
		return t.numShards
	}

	numShards := int(desiredShards)
	// Do not downshard if we are more than ten seconds back.
	if numShards < t.numShards && delay > 10.0 {
		level.Debug(t.logger).Log("msg", "Not downsharding due to being too far behind")
		return t.numShards
	}

	switch {
	case numShards > t.cfg.MaxShards:
		numShards = t.cfg.MaxShards
	case numShards < t.cfg.MinShards:
		numShards = t.cfg.MinShards
	}
	return numShards
}

func (t *QueueManager) reshardLoop() {
	defer t.wg.Done()

	for {
		select {
		case numShards := <-t.reshardChan:
			// We start the newShards after we have stopped (the therefore completely
			// flushed) the oldShards, to guarantee we only every deliver samples in
			// order.
			t.shards.stop()
			t.shards.start(numShards)
		case <-t.quit:
			return
		}
	}
}

func (t *QueueManager) newShards() *shards {
	s := &shards{
		qm:   t,
		done: make(chan struct{}),
	}
	return s
}

type shards struct {
	mtx sync.RWMutex // With the WAL, this is never actually contended.

	qm     *QueueManager
	queues []*queue
	// So we can accurately track how many of each are lost during shard shutdowns.
	enqueuedSamples    atomic.Int64
	enqueuedExemplars  atomic.Int64
	enqueuedHistograms atomic.Int64

	// Emulate a wait group with a channel and an atomic int, as you
	// cannot select on a wait group.
	done    chan struct{}
	running atomic.Int32

	// Soft shutdown context will prevent new enqueues and deadlocks.
	softShutdown chan struct{}

	// Hard shutdown context is used to terminate outgoing HTTP connections
	// after giving them a chance to terminate.
	hardShutdown                    context.CancelFunc
	samplesDroppedOnHardShutdown    atomic.Uint32
	exemplarsDroppedOnHardShutdown  atomic.Uint32
	histogramsDroppedOnHardShutdown atomic.Uint32
}

// start the shards; must be called before any call to enqueue.
func (s *shards) start(n int) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.qm.metrics.pendingSamples.Set(0)
	s.qm.metrics.numShards.Set(float64(n))

	newQueues := make([]*queue, n)
	for i := 0; i < n; i++ {
		newQueues[i] = newQueue(s.qm.cfg.MaxSamplesPerSend, s.qm.cfg.Capacity)
	}

	s.queues = newQueues

	var hardShutdownCtx context.Context
	hardShutdownCtx, s.hardShutdown = context.WithCancel(context.Background())
	s.softShutdown = make(chan struct{})
	s.running.Store(int32(n))
	s.done = make(chan struct{})
	s.enqueuedSamples.Store(0)
	s.enqueuedExemplars.Store(0)
	s.enqueuedHistograms.Store(0)
	s.samplesDroppedOnHardShutdown.Store(0)
	s.exemplarsDroppedOnHardShutdown.Store(0)
	s.histogramsDroppedOnHardShutdown.Store(0)
	for i := 0; i < n; i++ {
		go s.runShard(hardShutdownCtx, i, newQueues[i])
	}
}

// stop the shards; subsequent call to enqueue will return false.
func (s *shards) stop() {
	// Attempt a clean shutdown, but only wait flushDeadline for all the shards
	// to cleanly exit. As we're doing RPCs, enqueue can block indefinitely.
	// We must be able so call stop concurrently, hence we can only take the
	// RLock here.
	s.mtx.RLock()
	close(s.softShutdown)
	s.mtx.RUnlock()

	// Enqueue should now be unblocked, so we can take the write lock.  This
	// also ensures we don't race with writes to the queues, and get a panic:
	// send on closed channel.
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, queue := range s.queues {
		go queue.FlushAndShutdown(s.done)
	}
	select {
	case <-s.done:
		return
	case <-time.After(s.qm.flushDeadline):
	}

	// Force an unclean shutdown.
	s.hardShutdown()
	<-s.done
	if dropped := s.samplesDroppedOnHardShutdown.Load(); dropped > 0 {
		level.Error(s.qm.logger).Log("msg", "Failed to flush all samples on shutdown", "count", dropped)
	}
	if dropped := s.exemplarsDroppedOnHardShutdown.Load(); dropped > 0 {
		level.Error(s.qm.logger).Log("msg", "Failed to flush all exemplars on shutdown", "count", dropped)
	}
}

// enqueue data (sample or exemplar). If the shard is full, shutting down, or
// resharding, it will return false; in this case, you should back off and
// retry. A shard is full when its configured capacity has been reached,
// specifically, when s.queues[shard] has filled its batchQueue channel and the
// partial batch has also been filled.
func (s *shards) enqueue(data *TimeSeries) bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	shard := data.SeriesLabels.Hash() % uint64(len(s.queues))
	select {
	case <-s.softShutdown:
		return false
	default:
		appended := s.queues[shard].Append(data)
		if !appended {
			return false
		}
		switch data.SeriesType {
		case tSample:
			s.qm.metrics.pendingSamples.Inc()
			s.enqueuedSamples.Inc()
		case tExemplar:
			s.qm.metrics.pendingExemplars.Inc()
			s.enqueuedExemplars.Inc()
		case tHistogram, tFloatHistogram:
			s.qm.metrics.pendingHistograms.Inc()
			s.enqueuedHistograms.Inc()
		}
		return true
	}
}

type queue struct {
	// batchMtx covers operations appending to or publishing the partial batch.
	batchMtx   sync.Mutex
	batch      []*TimeSeries
	batchQueue chan []*TimeSeries
	batchSize  int
}

type TimeSeries struct {
	SeriesLabels   labels.Labels
	Value          float64
	Histogram      *histogram.Histogram
	FloatHistogram *histogram.FloatHistogram
	Timestamp      int64
	ExemplarLabels labels.Labels
	// The type of series: sample, exemplar, or histogram.
	SeriesType seriesType
}

type seriesType int

const (
	tSample seriesType = iota
	tExemplar
	tHistogram
	tFloatHistogram
)

func newQueue(batchSize, capacity int) *queue {
	batches := capacity / batchSize
	// Always create an unbuffered channel even if capacity is configured to be
	// less than max_samples_per_send.
	if batches == 0 {
		batches = 1
	}
	return &queue{
		batch:      make([]*TimeSeries, 0, batchSize),
		batchQueue: make(chan []*TimeSeries, batches),
		batchSize:  batchSize,
	}
}

// Append the timeSeries to the buffered batch. Returns false if it
// cannot be added and must be retried.
func (q *queue) Append(datum *TimeSeries) bool {
	q.batchMtx.Lock()
	defer q.batchMtx.Unlock()
	q.batch = append(q.batch, datum)
	if len(q.batch) == cap(q.batch) {
		select {
		case q.batchQueue <- q.batch:
			q.batch = make([]*TimeSeries, 0, q.batchSize)
			return true
		default:
			// Remove the sample we just appended. It will get retried.
			q.batch = q.batch[:len(q.batch)-1]
			return false
		}
	}
	return true
}

func (q *queue) Chan() <-chan []*TimeSeries {
	return q.batchQueue
}

// Batch returns the current batch and allocates a new batch.
func (q *queue) Batch() []*TimeSeries {
	q.batchMtx.Lock()
	defer q.batchMtx.Unlock()

	select {
	case batch := <-q.batchQueue:
		return batch
	default:
		batch := q.batch
		q.batch = make([]*TimeSeries, 0)
		return batch
	}
}

// ReturnForReuse adds the batch buffer back to the internal pool.
func (q *queue) ReturnForReuse(batch []*TimeSeries) {
	for _, b := range batch {
		deserializeMetrics.Put(b)
	}
}

// FlushAndShutdown stops the queue and flushes any samples. No appends can be
// made after this is called.
func (q *queue) FlushAndShutdown(done <-chan struct{}) {
	for q.tryEnqueueingBatch(done) {
		time.Sleep(time.Second)
	}
	q.batch = nil
	close(q.batchQueue)
}

// tryEnqueueingBatch tries to send a batch if necessary. If sending needs to
// be retried it will return true.
func (q *queue) tryEnqueueingBatch(done <-chan struct{}) bool {
	q.batchMtx.Lock()
	defer q.batchMtx.Unlock()
	if len(q.batch) == 0 {
		return false
	}

	select {
	case q.batchQueue <- q.batch:
		return false
	case <-done:
		// The shard has been hard shut down, so no more samples can be sent.
		// No need to try again as we will drop everything left in the queue.
		return false
	default:
		// The batchQueue is full, so we need to try again later.
		return true
	}
}

func (s *shards) runShard(ctx context.Context, shardID int, queue *queue) {
	defer func() {
		if s.running.Dec() == 0 {
			close(s.done)
		}
	}()

	shardNum := strconv.Itoa(shardID)

	// Send batches of at most MaxSamplesPerSend samples to the remote storage.
	// If we have fewer samples than that, flush them out after a deadline anyways.
	var (
		max = s.qm.cfg.MaxSamplesPerSend

		pBuf = proto.NewBuffer(nil)
		buf  []byte
	)
	if s.qm.sendExemplars {
		max += int(float64(max) * 0.1)
	}

	batchQueue := queue.Chan()
	pendingData := make([]prompb.TimeSeries, max)
	for i := range pendingData {
		pendingData[i].Samples = []prompb.Sample{{}}
		if s.qm.sendExemplars {
			pendingData[i].Exemplars = []prompb.Exemplar{{}}
		}
	}

	timer := time.NewTimer(time.Duration(s.qm.cfg.BatchSendDeadline))
	stop := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
	defer stop()

	for {
		select {
		case <-ctx.Done():
			// In this case we drop all samples in the buffer and the queue.
			// Remove them from pending and mark them as failed.
			droppedSamples := int(s.enqueuedSamples.Load())
			droppedExemplars := int(s.enqueuedExemplars.Load())
			droppedHistograms := int(s.enqueuedHistograms.Load())
			s.qm.metrics.pendingSamples.Sub(float64(droppedSamples))
			s.qm.metrics.pendingExemplars.Sub(float64(droppedExemplars))
			s.qm.metrics.pendingHistograms.Sub(float64(droppedHistograms))
			s.qm.metrics.failedSamplesTotal.Add(float64(droppedSamples))
			s.qm.metrics.failedExemplarsTotal.Add(float64(droppedExemplars))
			s.qm.metrics.failedHistogramsTotal.Add(float64(droppedHistograms))
			s.samplesDroppedOnHardShutdown.Add(uint32(droppedSamples))
			s.exemplarsDroppedOnHardShutdown.Add(uint32(droppedExemplars))
			s.histogramsDroppedOnHardShutdown.Add(uint32(droppedHistograms))
			return

		case batch, ok := <-batchQueue:
			if !ok {
				return
			}
			nPendingSamples, nPendingExemplars, nPendingHistograms := s.populateTimeSeries(batch, pendingData)
			queue.ReturnForReuse(batch)
			n := nPendingSamples + nPendingExemplars + nPendingHistograms
			s.sendSamples(ctx, pendingData[:n], nPendingSamples, nPendingExemplars, nPendingHistograms, pBuf, &buf)

			stop()
			timer.Reset(time.Duration(s.qm.cfg.BatchSendDeadline))

		case <-timer.C:
			batch := queue.Batch()
			if len(batch) > 0 {
				nPendingSamples, nPendingExemplars, nPendingHistograms := s.populateTimeSeries(batch, pendingData)
				n := nPendingSamples + nPendingExemplars + nPendingHistograms
				level.Debug(s.qm.logger).Log("msg", "runShard timer ticked, sending buffered data", "samples", nPendingSamples,
					"exemplars", nPendingExemplars, "shard", shardNum, "histograms", nPendingHistograms)
				s.sendSamples(ctx, pendingData[:n], nPendingSamples, nPendingExemplars, nPendingHistograms, pBuf, &buf)
			}
			queue.ReturnForReuse(batch)
			timer.Reset(time.Duration(s.qm.cfg.BatchSendDeadline))
		}
	}
}

func (s *shards) populateTimeSeries(batch []*TimeSeries, pendingData []prompb.TimeSeries) (int, int, int) {
	var nPendingSamples, nPendingExemplars, nPendingHistograms int
	for nPending, d := range batch {
		pendingData[nPending].Samples = pendingData[nPending].Samples[:0]
		if s.qm.sendExemplars {
			pendingData[nPending].Exemplars = pendingData[nPending].Exemplars[:0]
		}
		if s.qm.sendNativeHistograms {
			pendingData[nPending].Histograms = pendingData[nPending].Histograms[:0]
		}

		// Number of pending samples is limited by the fact that sendSamples (via sendSamplesWithBackoff)
		// retries endlessly, so once we reach max samples, if we can never send to the endpoint we'll
		// stop reading from the queue. This makes it safe to reference pendingSamples by index.
		pendingData[nPending].Labels = labelsToLabelsProto(d.SeriesLabels, pendingData[nPending].Labels)
		switch d.SeriesType {
		case tSample:
			pendingData[nPending].Samples = append(pendingData[nPending].Samples, prompb.Sample{
				Value:     d.Value,
				Timestamp: d.Timestamp,
			})
			nPendingSamples++
		case tExemplar:
			pendingData[nPending].Exemplars = append(pendingData[nPending].Exemplars, prompb.Exemplar{
				Labels:    labelsToLabelsProto(d.ExemplarLabels, nil),
				Value:     d.Value,
				Timestamp: d.Timestamp,
			})
			nPendingExemplars++
		case tHistogram:
			pendingData[nPending].Histograms = append(pendingData[nPending].Histograms, HistogramToHistogramProto(d.Timestamp, d.Histogram))
			nPendingHistograms++
		case tFloatHistogram:
			pendingData[nPending].Histograms = append(pendingData[nPending].Histograms, FloatHistogramToHistogramProto(d.Timestamp, d.FloatHistogram))
			nPendingHistograms++
		}
	}
	return nPendingSamples, nPendingExemplars, nPendingHistograms
}

func (s *shards) sendSamples(ctx context.Context, samples []prompb.TimeSeries, sampleCount, exemplarCount, histogramCount int, pBuf *proto.Buffer, buf *[]byte) {
	begin := time.Now()
	err := s.sendSamplesWithBackoff(ctx, samples, sampleCount, exemplarCount, histogramCount, pBuf, buf)
	if err != nil {
		level.Error(s.qm.logger).Log("msg", "non-recoverable error", "count", sampleCount, "exemplarCount", exemplarCount, "err", err)
		s.qm.metrics.failedSamplesTotal.Add(float64(sampleCount))
		s.qm.metrics.failedExemplarsTotal.Add(float64(exemplarCount))
		s.qm.metrics.failedHistogramsTotal.Add(float64(histogramCount))
	}

	// These counters are used to calculate the dynamic sharding, and as such
	// should be maintained irrespective of success or failure.
	s.qm.dataOut.incr(int64(len(samples)))
	s.qm.dataOutDuration.incr(int64(time.Since(begin)))
	s.qm.lastSendTimestamp.Store(time.Now().Unix())
	// Pending samples/exemplars/histograms also should be subtracted, as an error means
	// they will not be retried.
	s.qm.metrics.pendingSamples.Sub(float64(sampleCount))
	s.qm.metrics.pendingExemplars.Sub(float64(exemplarCount))
	s.qm.metrics.pendingHistograms.Sub(float64(histogramCount))
	s.enqueuedSamples.Sub(int64(sampleCount))
	s.enqueuedExemplars.Sub(int64(exemplarCount))
	s.enqueuedHistograms.Sub(int64(histogramCount))
}

// sendSamples to the remote storage with backoff for recoverable errors.
func (s *shards) sendSamplesWithBackoff(ctx context.Context, samples []prompb.TimeSeries, sampleCount, exemplarCount, histogramCount int, pBuf *proto.Buffer, buf *[]byte) error {
	// Build the WriteRequest with no metadata.
	req, highest, err := buildWriteRequest(samples, nil, pBuf, *buf)
	if err != nil {
		// Failing to build the write request is non-recoverable, since it will
		// only error if marshaling the proto to bytes fails.
		return err
	}

	reqSize := len(req)
	*buf = req

	// An anonymous function allows us to defer the completion of our per-try spans
	// without causing a memory leak, and it has the nice effect of not propagating any
	// parameters for sendSamplesWithBackoff/3.
	attemptStore := func(try int) error {
		ctx, span := otel.Tracer("").Start(ctx, "Remote Send Batch")
		defer span.End()

		span.SetAttributes(
			attribute.Int("request_size", reqSize),
			attribute.Int("samples", sampleCount),
			attribute.Int("try", try),
			attribute.String("remote_name", s.qm.storeClient.Name()),
			attribute.String("remote_url", s.qm.storeClient.Endpoint()),
		)

		if exemplarCount > 0 {
			span.SetAttributes(attribute.Int("exemplars", exemplarCount))
		}
		if histogramCount > 0 {
			span.SetAttributes(attribute.Int("histograms", histogramCount))
		}

		begin := time.Now()
		s.qm.metrics.samplesTotal.Add(float64(sampleCount))
		s.qm.metrics.exemplarsTotal.Add(float64(exemplarCount))
		s.qm.metrics.histogramsTotal.Add(float64(histogramCount))
		err := s.qm.client().Store(ctx, *buf)
		s.qm.metrics.sentBatchDuration.Observe(time.Since(begin).Seconds())

		if err != nil {
			span.RecordError(err)
			return err
		}

		return nil
	}

	onRetry := func() {
		s.qm.metrics.retriedSamplesTotal.Add(float64(sampleCount))
		s.qm.metrics.retriedExemplarsTotal.Add(float64(exemplarCount))
		s.qm.metrics.retriedHistogramsTotal.Add(float64(histogramCount))
	}

	err = sendWriteRequestWithBackoff(ctx, s.qm.cfg, s.qm.logger, attemptStore, onRetry)
	if errors.Is(err, context.Canceled) {
		// When there is resharding, we cancel the context for this queue, which means the data is not sent.
		// So we exit early to not update the metrics.
		return err
	}

	s.qm.metrics.sentBytesTotal.Add(float64(reqSize))
	s.qm.metrics.highestSentTimestamp.Set(float64(highest / 1000))

	return err
}

func sendWriteRequestWithBackoff(ctx context.Context, cfg QueueOptions, l log.Logger, attempt func(int) error, onRetry func()) error {
	backoff := cfg.MinBackoff
	sleepDuration := model.Duration(0)
	try := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := attempt(try)

		if err == nil {
			return nil
		}

		// If the error is unrecoverable, we should not retry.
		var backoffErr RecoverableError
		if !errors.As(err, &backoffErr) {
			return err
		}

		sleepDuration = model.Duration(backoff)
		switch {
		case backoffErr.retryAfter > 0:
			sleepDuration = model.Duration(backoffErr.retryAfter)
			level.Info(l).Log("msg", "Retrying after duration specified by Retry-After header", "duration", sleepDuration)
		case backoffErr.retryAfter < 0:
			level.Debug(l).Log("msg", "retry-after cannot be in past, retrying using default backoff mechanism")
		}

		select {
		case <-ctx.Done():
		case <-time.After(time.Duration(sleepDuration)):
		}

		// If we make it this far, we've encountered a recoverable error and will retry.
		onRetry()
		level.Warn(l).Log("msg", "Failed to send batch, retrying", "err", err)

		backoff = time.Duration(sleepDuration) * 2

		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}

		try++
	}
}

func buildWriteRequest(samples []prompb.TimeSeries, metadata []prompb.MetricMetadata, pBuf *proto.Buffer, buf []byte) ([]byte, int64, error) {
	var highest int64
	for _, ts := range samples {
		// At the moment we only ever append a TimeSeries with a single sample or exemplar in it.
		if len(ts.Samples) > 0 && ts.Samples[0].Timestamp > highest {
			highest = ts.Samples[0].Timestamp
		}
		if len(ts.Exemplars) > 0 && ts.Exemplars[0].Timestamp > highest {
			highest = ts.Exemplars[0].Timestamp
		}
		if len(ts.Histograms) > 0 && ts.Histograms[0].Timestamp > highest {
			highest = ts.Histograms[0].Timestamp
		}
	}

	req := &prompb.WriteRequest{
		Timeseries: samples,
		Metadata:   metadata,
	}

	if pBuf == nil {
		pBuf = proto.NewBuffer(nil) // For convenience in tests. Not efficient.
	} else {
		pBuf.Reset()
	}
	err := pBuf.Marshal(req)
	if err != nil {
		return nil, highest, err
	}

	// snappy uses len() to see if it needs to allocate a new slice. Make the
	// buffer as long as possible.
	if buf != nil {
		buf = buf[0:cap(buf)]
	}
	compressed := snappy.Encode(buf, pBuf.Bytes())
	return compressed, highest, nil
}

type RecoverableError struct {
	error
	retryAfter time.Duration
}

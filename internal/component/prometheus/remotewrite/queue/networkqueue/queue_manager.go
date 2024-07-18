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

package networkqueue

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
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite/queue/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/atomic"

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

	reasonTooOld                     = "too_old"
	reasonDroppedSeries              = "dropped_series"
	reasonUnintentionalDroppedSeries = "unintentionally_dropped_series"
)

type queueManagerMetrics struct {
	reg prometheus.Registerer

	samplesTotal           prometheus.Counter
	exemplarsTotal         prometheus.Counter
	histogramsTotal        prometheus.Counter
	metadataTotal          prometheus.Counter
	failedSamplesTotal     prometheus.Counter
	failedExemplarsTotal   prometheus.Counter
	failedHistogramsTotal  prometheus.Counter
	failedMetadataTotal    prometheus.Counter
	retriedSamplesTotal    prometheus.Counter
	retriedExemplarsTotal  prometheus.Counter
	retriedHistogramsTotal prometheus.Counter
	retriedMetadataTotal   prometheus.Counter
	droppedSamplesTotal    *prometheus.CounterVec
	droppedExemplarsTotal  *prometheus.CounterVec
	droppedHistogramsTotal *prometheus.CounterVec
	enqueueRetriesTotal    prometheus.Counter
	sentBatchDuration      prometheus.Histogram
	highestSentTimestamp   *types.MaxTimestamp
	pendingSamples         prometheus.Gauge
	pendingExemplars       prometheus.Gauge
	pendingHistograms      prometheus.Gauge
	shardCapacity          prometheus.Gauge
	numShards              prometheus.Gauge
	maxNumShards           prometheus.Gauge
	minNumShards           prometheus.Gauge
	desiredNumShards       prometheus.Gauge
	sentBytesTotal         prometheus.Counter
	metadataBytesTotal     prometheus.Counter
	maxSamplesPerSend      prometheus.Gauge
}

// String constants for instrumentation.
const (
	RemoteName = "remote_name"
	EndPoint   = "url"
)

func newQueueManagerMetrics(r prometheus.Registerer, rn, e string) *queueManagerMetrics {
	m := &queueManagerMetrics{
		reg: r,
	}
	constLabels := prometheus.Labels{
		RemoteName: rn,
		EndPoint:   e,
	}

	m.samplesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "samples_total",
		Help:        "Total number of samples sent to remote storage.",
		ConstLabels: constLabels,
	})
	m.exemplarsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "exemplars_total",
		Help:        "Total number of exemplars sent to remote storage.",
		ConstLabels: constLabels,
	})
	m.histogramsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "histograms_total",
		Help:        "Total number of histograms sent to remote storage.",
		ConstLabels: constLabels,
	})
	m.metadataTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "metadata_total",
		Help:        "Total number of metadata entries sent to remote storage.",
		ConstLabels: constLabels,
	})
	m.failedSamplesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "samples_failed_total",
		Help:        "Total number of samples which failed on send to remote storage, non-recoverable errors.",
		ConstLabels: constLabels,
	})
	m.failedExemplarsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "exemplars_failed_total",
		Help:        "Total number of exemplars which failed on send to remote storage, non-recoverable errors.",
		ConstLabels: constLabels,
	})
	m.failedHistogramsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "histograms_failed_total",
		Help:        "Total number of histograms which failed on send to remote storage, non-recoverable errors.",
		ConstLabels: constLabels,
	})
	m.failedMetadataTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "metadata_failed_total",
		Help:        "Total number of metadata entries which failed on send to remote storage, non-recoverable errors.",
		ConstLabels: constLabels,
	})
	m.retriedSamplesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "samples_retried_total",
		Help:        "Total number of samples which failed on send to remote storage but were retried because the send error was recoverable.",
		ConstLabels: constLabels,
	})
	m.retriedExemplarsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "exemplars_retried_total",
		Help:        "Total number of exemplars which failed on send to remote storage but were retried because the send error was recoverable.",
		ConstLabels: constLabels,
	})
	m.retriedHistogramsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "histograms_retried_total",
		Help:        "Total number of histograms which failed on send to remote storage but were retried because the send error was recoverable.",
		ConstLabels: constLabels,
	})
	m.retriedMetadataTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "metadata_retried_total",
		Help:        "Total number of metadata entries which failed on send to remote storage but were retried because the send error was recoverable.",
		ConstLabels: constLabels,
	})
	m.droppedSamplesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "samples_dropped_total",
		Help:        "Total number of samples which were dropped after being read from the WAL before being sent via remote write, either via relabelling, due to being too old or unintentionally because of an unknown reference ID.",
		ConstLabels: constLabels,
	}, []string{"reason"})
	m.droppedExemplarsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "exemplars_dropped_total",
		Help:        "Total number of exemplars which were dropped after being read from the WAL before being sent via remote write, either via relabelling, due to being too old or unintentionally because of an unknown reference ID.",
		ConstLabels: constLabels,
	}, []string{"reason"})
	m.droppedHistogramsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "histograms_dropped_total",
		Help:        "Total number of histograms which were dropped after being read from the WAL before being sent via remote write, either via relabelling, due to being too old or unintentionally because of an unknown reference ID.",
		ConstLabels: constLabels,
	}, []string{"reason"})
	m.enqueueRetriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "enqueue_retries_total",
		Help:        "Total number of times enqueue has failed because a shards queue was full.",
		ConstLabels: constLabels,
	})
	m.sentBatchDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "sent_batch_duration_seconds",
		Help:        "Duration of send calls to the remote storage.",
		Buckets:     append(prometheus.DefBuckets, 25, 60, 120, 300),
		ConstLabels: constLabels,
	})
	m.highestSentTimestamp = &types.MaxTimestamp{
		Gauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   types.Namespace,
			Subsystem:   types.Subsystem,
			Name:        "queue_highest_sent_timestamp_seconds",
			Help:        "Timestamp from a WAL sample, the highest timestamp successfully sent by this queue, in seconds since epoch.",
			ConstLabels: constLabels,
		}),
	}
	m.pendingSamples = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "samples_pending",
		Help:        "The number of samples pending in the queues shards to be sent to the remote storage.",
		ConstLabels: constLabels,
	})
	m.pendingExemplars = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "exemplars_pending",
		Help:        "The number of exemplars pending in the queues shards to be sent to the remote storage.",
		ConstLabels: constLabels,
	})
	m.pendingHistograms = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "histograms_pending",
		Help:        "The number of histograms pending in the queues shards to be sent to the remote storage.",
		ConstLabels: constLabels,
	})
	m.shardCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "shard_capacity",
		Help:        "The capacity of each shard of the queue used for parallel sending to the remote storage.",
		ConstLabels: constLabels,
	})
	m.numShards = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "shards",
		Help:        "The number of shards used for parallel sending to the remote storage.",
		ConstLabels: constLabels,
	})
	m.maxNumShards = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "shards_max",
		Help:        "The maximum number of shards that the queue is allowed to run.",
		ConstLabels: constLabels,
	})
	m.minNumShards = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "shards_min",
		Help:        "The minimum number of shards that the queue is allowed to run.",
		ConstLabels: constLabels,
	})
	m.desiredNumShards = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "shards_desired",
		Help:        "The number of shards that the queues shard calculation wants to run based on the rate of samples in vs. samples out.",
		ConstLabels: constLabels,
	})
	m.sentBytesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "bytes_total",
		Help:        "The total number of bytes of data (not metadata) sent by the queue after compression. Note that when exemplars over remote write is enabled the exemplars included in a remote write request count towards this metric.",
		ConstLabels: constLabels,
	})
	m.metadataBytesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "metadata_bytes_total",
		Help:        "The total number of bytes of metadata sent by the queue after compression.",
		ConstLabels: constLabels,
	})
	m.maxSamplesPerSend = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   types.Namespace,
		Subsystem:   types.Subsystem,
		Name:        "max_samples_per_send",
		Help:        "The maximum number of samples to be sent, in a single request, to the remote storage. Note that, when sending of exemplars over remote write is enabled, exemplars count towards this limt.",
		ConstLabels: constLabels,
	})

	return m
}

func (m *queueManagerMetrics) register() {
	if m.reg != nil {
		m.reg.MustRegister(
			m.samplesTotal,
			m.exemplarsTotal,
			m.histogramsTotal,
			m.metadataTotal,
			m.failedSamplesTotal,
			m.failedExemplarsTotal,
			m.failedHistogramsTotal,
			m.failedMetadataTotal,
			m.retriedSamplesTotal,
			m.retriedExemplarsTotal,
			m.retriedHistogramsTotal,
			m.retriedMetadataTotal,
			m.droppedSamplesTotal,
			m.droppedExemplarsTotal,
			m.droppedHistogramsTotal,
			m.enqueueRetriesTotal,
			m.sentBatchDuration,
			m.highestSentTimestamp,
			m.pendingSamples,
			m.pendingExemplars,
			m.pendingHistograms,
			m.shardCapacity,
			m.numShards,
			m.maxNumShards,
			m.minNumShards,
			m.desiredNumShards,
			m.sentBytesTotal,
			m.metadataBytesTotal,
			m.maxSamplesPerSend,
		)
	}
}

func (m *queueManagerMetrics) unregister() {
	if m.reg != nil {
		m.reg.Unregister(m.samplesTotal)
		m.reg.Unregister(m.exemplarsTotal)
		m.reg.Unregister(m.histogramsTotal)
		m.reg.Unregister(m.metadataTotal)
		m.reg.Unregister(m.failedSamplesTotal)
		m.reg.Unregister(m.failedExemplarsTotal)
		m.reg.Unregister(m.failedHistogramsTotal)
		m.reg.Unregister(m.failedMetadataTotal)
		m.reg.Unregister(m.retriedSamplesTotal)
		m.reg.Unregister(m.retriedExemplarsTotal)
		m.reg.Unregister(m.retriedHistogramsTotal)
		m.reg.Unregister(m.retriedMetadataTotal)
		m.reg.Unregister(m.droppedSamplesTotal)
		m.reg.Unregister(m.droppedExemplarsTotal)
		m.reg.Unregister(m.droppedHistogramsTotal)
		m.reg.Unregister(m.enqueueRetriesTotal)
		m.reg.Unregister(m.sentBatchDuration)
		m.reg.Unregister(m.highestSentTimestamp)
		m.reg.Unregister(m.pendingSamples)
		m.reg.Unregister(m.pendingExemplars)
		m.reg.Unregister(m.pendingHistograms)
		m.reg.Unregister(m.shardCapacity)
		m.reg.Unregister(m.numShards)
		m.reg.Unregister(m.maxNumShards)
		m.reg.Unregister(m.minNumShards)
		m.reg.Unregister(m.desiredNumShards)
		m.reg.Unregister(m.sentBytesTotal)
		m.reg.Unregister(m.metadataBytesTotal)
		m.reg.Unregister(m.maxSamplesPerSend)
	}
}

// QueueManager manages a queue of samples to be sent to the Storage
// indicated by the provided WriteClient. Implements writeTo interface
// used by WAL Watcher.
type QueueManager struct {
	lastSendTimestamp          atomic.Int64
	buildRequestLimitTimestamp atomic.Int64

	logger               log.Logger
	flushDeadline        time.Duration
	cfg                  QueueOptions
	externalLabels       []labels.Label
	sendExemplars        bool
	sendNativeHistograms bool

	clientMtx   sync.RWMutex
	storeClient types.WriteClient

	shards      *shards
	numShards   int
	reshardChan chan int
	quit        chan struct{}
	wg          sync.WaitGroup

	dataIn, dataDropped, dataOut, dataOutDuration *ewmaRate

	metrics              *queueManagerMetrics
	interner             *pool
	highestRecvTimestamp *types.MaxTimestamp
}

// NewQueueManager builds a new QueueManager and starts a new
// WAL watcher with queue manager as the WriteTo destination.
// The WAL watcher takes the dir parameter as the base directory
// for where the WAL shall be located. Note that the full path to
// the WAL directory will be constructed as <dir>/wal.
func NewQueueManager(
	metrics *queueManagerMetrics,
	logger log.Logger,
	samplesIn *ewmaRate,
	cfg QueueOptions,
	externalLabels labels.Labels,
	client types.WriteClient,
	flushDeadline time.Duration,
	interner *pool,
	highestRecvTimestamp *types.MaxTimestamp,
	enableExemplarRemoteWrite bool,
	enableNativeHistogramRemoteWrite bool,
) *QueueManager {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	// Copy externalLabels into a slice, which we need for processExternalLabels.
	extLabelsSlice := make([]labels.Label, 0, externalLabels.Len())
	externalLabels.Range(func(l labels.Label) {
		extLabelsSlice = append(extLabelsSlice, l)
	})

	logger = log.With(logger, RemoteName, client.Name(), EndPoint, client.Endpoint())
	t := &QueueManager{
		logger:               logger,
		flushDeadline:        flushDeadline,
		cfg:                  cfg,
		externalLabels:       extLabelsSlice,
		storeClient:          client,
		sendExemplars:        enableExemplarRemoteWrite,
		sendNativeHistograms: enableNativeHistogramRemoteWrite,

		numShards:   cfg.MinShards,
		reshardChan: make(chan int),
		quit:        make(chan struct{}),

		dataIn:          samplesIn,
		dataDropped:     newEWMARate(ewmaWeight, shardUpdateDuration),
		dataOut:         newEWMARate(ewmaWeight, shardUpdateDuration),
		dataOutDuration: newEWMARate(ewmaWeight, shardUpdateDuration),

		metrics:              metrics,
		interner:             interner,
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
	numSends := int(math.Ceil(float64(len(metadata)) / float64(t.cfg.MaxSamplesPerSend)))
	for i := 0; i < numSends; i++ {
		last := (i + 1) * t.cfg.MaxSamplesPerSend
		if last > len(metadata) {
			last = len(metadata)
		}
		err := t.sendMetadataWithBackoff(ctx, mm[i*t.cfg.MaxSamplesPerSend:last], pBuf)
		if err != nil {
			t.metrics.failedMetadataTotal.Add(float64(last - (i * t.cfg.MaxSamplesPerSend)))
			level.Error(t.logger).Log("msg", "non-recoverable error while sending metadata", "count", last-(i*t.cfg.MaxSamplesPerSend), "err", err)
		}
	}
}

func (t *QueueManager) sendMetadataWithBackoff(ctx context.Context, metadata []prompb.MetricMetadata, pBuf *proto.Buffer) error {
	// Build the WriteRequest with no samples.
	req, _, _, err := buildWriteRequest(t.logger, nil, metadata, pBuf, nil, nil)
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
		// Attributes defined by OpenTelemetry semantic conventions.
		if try > 0 {
			span.SetAttributes(semconv.HTTPResendCount(try))
		}

		begin := time.Now()
		err := t.storeClient.Store(ctx, req, try)
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
func (t *QueueManager) Append(samples []*types.TimeSeries) bool {
outer:
	for _, s := range samples {
		s.SeriesLabels = processExternalLabels(s.SeriesLabels, t.externalLabels)
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

func (t *QueueManager) AppendExemplars(exemplars []*types.TimeSeries) bool {
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

func (t *QueueManager) AppendHistograms(histograms []*types.TimeSeries) bool {
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

func (t *QueueManager) AppendFloatHistograms(floatHistograms []*types.TimeSeries) bool {
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

	t.metrics.unregister()
}

// SetClient updates the client used by a queue. Used when only client specific
// fields are updated to avoid restarting the queue.
func (t *QueueManager) SetClient(c types.WriteClient) {
	t.clientMtx.Lock()
	t.storeClient = c
	t.clientMtx.Unlock()
}

func (t *QueueManager) client() types.WriteClient {
	t.clientMtx.RLock()
	defer t.clientMtx.RUnlock()
	return t.storeClient
}

func (t *QueueManager) internLabels(lbls labels.Labels) {
	lbls.InternStrings(t.interner.intern)
}

func (t *QueueManager) releaseLabels(ls labels.Labels) {
	ls.ReleaseStrings(t.interner.release)
}

// processExternalLabels merges externalLabels into ls. If ls contains
// a label in externalLabels, the value in ls wins.
func processExternalLabels(ls labels.Labels, externalLabels []labels.Label) labels.Labels {
	if len(externalLabels) == 0 {
		return ls
	}

	b := labels.NewScratchBuilder(ls.Len() + len(externalLabels))
	j := 0
	ls.Range(func(l labels.Label) {
		for j < len(externalLabels) && l.Name > externalLabels[j].Name {
			b.Add(externalLabels[j].Name, externalLabels[j].Value)
			j++
		}
		if j < len(externalLabels) && l.Name == externalLabels[j].Name {
			j++
		}
		b.Add(l.Name, l.Value)
	})
	for ; j < len(externalLabels); j++ {
		b.Add(externalLabels[j].Name, externalLabels[j].Value)
	}

	return b.Labels()
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
func (s *shards) enqueue(data *types.TimeSeries) bool {
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
		switch data.Type {
		case types.Sample:
			s.qm.metrics.pendingSamples.Inc()
			s.enqueuedSamples.Inc()
		case types.Exemplar:
			s.qm.metrics.pendingExemplars.Inc()
			s.enqueuedExemplars.Inc()
		case types.Histogram, types.FloatHistogram:
			s.qm.metrics.pendingHistograms.Inc()
			s.enqueuedHistograms.Inc()
		}
		return true
	}
}

type queue struct {
	// batchMtx covers operations appending to or publishing the partial batch.
	batchMtx   sync.Mutex
	batch      []*types.TimeSeries
	batchQueue chan []*types.TimeSeries

	// Since we know there are a limited number of batches out, using a stack
	// is easy and safe so a sync.Pool is not necessary.
	// poolMtx covers adding and removing batches from the batchPool.
	poolMtx   sync.Mutex
	batchPool [][]*types.TimeSeries
}

func newQueue(batchSize, capacity int) *queue {
	batches := capacity / batchSize
	// Always create an unbuffered channel even if capacity is configured to be
	// less than max_samples_per_send.
	if batches == 0 {
		batches = 1
	}
	return &queue{
		batch:      make([]*types.TimeSeries, 0, batchSize),
		batchQueue: make(chan []*types.TimeSeries, batches),
		// batchPool should have capacity for everything in the channel + 1 for
		// the batch being processed.
		batchPool: make([][]*types.TimeSeries, 0, batches+1),
	}
}

// Append the TimeSeries to the buffered batch. Returns false if it
// cannot be added and must be retried.
func (q *queue) Append(datum *types.TimeSeries) bool {
	q.batchMtx.Lock()
	defer q.batchMtx.Unlock()
	q.batch = append(q.batch, datum)
	if len(q.batch) == cap(q.batch) {
		select {
		case q.batchQueue <- q.batch:
			q.batch = q.newBatch(cap(q.batch))
			return true
		default:
			// Remove the sample we just appended. It will get retried.
			q.batch = q.batch[:len(q.batch)-1]
			return false
		}
	}

	return true
}

func (q *queue) Chan() <-chan []*types.TimeSeries {
	return q.batchQueue
}

// Batch returns the current batch and allocates a new batch.
func (q *queue) Batch() []*types.TimeSeries {
	q.batchMtx.Lock()
	defer q.batchMtx.Unlock()

	select {
	case batch := <-q.batchQueue:
		return batch
	default:
		batch := q.batch
		q.batch = q.newBatch(cap(batch))
		return batch
	}
}

// ReturnForReuse adds the batch buffer back to the internal pool.
func (q *queue) ReturnForReuse(batch []*types.TimeSeries) {
	q.poolMtx.Lock()
	defer q.poolMtx.Unlock()
	for _, ts := range batch {
		ts.Histogram = nil
		ts.FloatHistogram = nil
		ts.Value = 0
		ts.Timestamp = 0
		ts.ExemplarLabels = ts.ExemplarLabels[:0]
		ts.SeriesLabels = ts.SeriesLabels[:0]
		types.TimeSeriesPool.Put(ts)
	}
	if len(q.batchPool) < cap(q.batchPool) {
		q.batchPool = append(q.batchPool, batch[:0])
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

func (q *queue) newBatch(capacity int) []*types.TimeSeries {
	q.poolMtx.Lock()
	defer q.poolMtx.Unlock()
	batches := len(q.batchPool)
	if batches > 0 {
		batch := q.batchPool[batches-1]
		q.batchPool = q.batchPool[:batches-1]
		return batch
	}
	return make([]*types.TimeSeries, 0, capacity)
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

func (s *shards) populateTimeSeries(batch []*types.TimeSeries, pendingData []prompb.TimeSeries) (int, int, int) {
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
		pendingData[nPending].Labels = labelsToLabelsProto(d.seriesLabels, pendingData[nPending].Labels)
		switch d.Type {
		case types.Sample:
			pendingData[nPending].Samples = append(pendingData[nPending].Samples, prompb.Sample{
				Value:     d.Value,
				Timestamp: d.Timestamp,
			})
			nPendingSamples++
		case tExemplar:
			pendingData[nPending].Exemplars = append(pendingData[nPending].Exemplars, prompb.Exemplar{
				Labels:    labelsToLabelsProto(d.exemplarLabels, nil),
				Value:     d.value,
				Timestamp: d.timestamp,
			})
			nPendingExemplars++
		case tHistogram:
			pendingData[nPending].Histograms = append(pendingData[nPending].Histograms, HistogramToHistogramProto(d.timestamp, d.histogram))
			nPendingHistograms++
		case tFloatHistogram:
			pendingData[nPending].Histograms = append(pendingData[nPending].Histograms, FloatHistogramToHistogramProto(d.timestamp, d.floatHistogram))
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
	req, highest, lowest, err := buildWriteRequest(s.qm.logger, samples, nil, pBuf, *buf, nil)
	s.qm.buildRequestLimitTimestamp.Store(lowest)
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
		currentTime := time.Now()
		lowest := s.qm.buildRequestLimitTimestamp.Load()
		if isSampleOld(currentTime, time.Duration(s.qm.cfg.SampleAgeLimit), lowest) {
			// This will filter out old samples during retries.
			req, _, lowest, err := buildWriteRequest(
				s.qm.logger,
				samples,
				nil,
				pBuf,
				*buf,
				isTimeSeriesOldFilter(s.qm.metrics, currentTime, time.Duration(s.qm.cfg.SampleAgeLimit)),
			)
			s.qm.buildRequestLimitTimestamp.Store(lowest)
			if err != nil {
				return err
			}
			*buf = req
		}

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
		err := s.qm.client().Store(ctx, *buf, try)
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
			sleepDuration = backoffErr.retryAfter
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

		backoff = time.Duration(sleepDuration * 2)

		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}

		try++
	}
}

func buildTimeSeries(timeSeries []prompb.TimeSeries, filter func(prompb.TimeSeries) bool) (int64, int64, []prompb.TimeSeries, int, int, int) {
	var highest int64
	var lowest int64
	var droppedSamples, droppedExemplars, droppedHistograms int

	keepIdx := 0
	lowest = math.MaxInt64
	for i, ts := range timeSeries {
		if filter != nil && filter(ts) {
			if len(ts.Samples) > 0 {
				droppedSamples++
			}
			if len(ts.Exemplars) > 0 {
				droppedExemplars++
			}
			if len(ts.Histograms) > 0 {
				droppedHistograms++
			}
			continue
		}

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

		// Get lowest timestamp
		if len(ts.Samples) > 0 && ts.Samples[0].Timestamp < lowest {
			lowest = ts.Samples[0].Timestamp
		}
		if len(ts.Exemplars) > 0 && ts.Exemplars[0].Timestamp < lowest {
			lowest = ts.Exemplars[0].Timestamp
		}
		if len(ts.Histograms) > 0 && ts.Histograms[0].Timestamp < lowest {
			lowest = ts.Histograms[0].Timestamp
		}

		// Move the current element to the write position and increment the write pointer
		timeSeries[keepIdx] = timeSeries[i]
		keepIdx++
	}

	timeSeries = timeSeries[:keepIdx]
	return highest, lowest, timeSeries, droppedSamples, droppedExemplars, droppedHistograms
}

func buildWriteRequest(logger log.Logger, timeSeries []prompb.TimeSeries, metadata []prompb.MetricMetadata, pBuf *proto.Buffer, buf []byte, filter func(prompb.TimeSeries) bool) ([]byte, int64, int64, error) {
	highest, lowest, timeSeries,
		droppedSamples, droppedExemplars, droppedHistograms := buildTimeSeries(timeSeries, filter)

	if droppedSamples > 0 || droppedExemplars > 0 || droppedHistograms > 0 {
		level.Debug(logger).Log("msg", "dropped data due to their age", "droppedSamples", droppedSamples, "droppedExemplars", droppedExemplars, "droppedHistograms", droppedHistograms)
	}

	req := &prompb.WriteRequest{
		Timeseries: timeSeries,
		Metadata:   metadata,
	}

	if pBuf == nil {
		pBuf = proto.NewBuffer(nil) // For convenience in tests. Not efficient.
	} else {
		pBuf.Reset()
	}
	err := pBuf.Marshal(req)
	if err != nil {
		return nil, highest, lowest, err
	}

	// snappy uses len() to see if it needs to allocate a new slice. Make the
	// buffer as long as possible.
	if buf != nil {
		buf = buf[0:cap(buf)]
	}
	compressed := snappy.Encode(buf, pBuf.Bytes())
	return compressed, highest, lowest, nil
}

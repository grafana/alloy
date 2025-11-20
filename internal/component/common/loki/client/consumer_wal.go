package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
)

func NewWALConsumer(logger log.Logger, reg prometheus.Registerer, walCfg wal.Config, cfgs ...Config) (*WALConsumer, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("at least one endpoint config must be provided")
	}

	writer, err := wal.NewWriter(walCfg, logger, reg)
	if err != nil {
		return nil, fmt.Errorf("error creating wal writer: %w", err)
	}

	m := &WALConsumer{
		writer: writer,
		pairs:  make([]endpointWatcherPair, 0, len(cfgs)),
	}

	var (
		metrics        = NewMetrics(reg)
		endpointsCheck = make(map[string]struct{})

		walWatcherMetrics  = wal.NewWatcherMetrics(reg)
		walMarkerMetrics   = internal.NewMarkerMetrics(reg)
		walEndpointMetrics = NewWALEndpointMetrics(reg)
	)

	for _, cfg := range cfgs {
		// Don't allow duplicate endpoints, we have endpoint specific metrics that need at least one unique label value (name).
		name := getEndpointName(cfg)
		if _, ok := endpointsCheck[name]; ok {
			return nil, fmt.Errorf("duplicate endpoint configs are not allowed, found duplicate for name: %s", cfg.Name)
		}
		endpointsCheck[name] = struct{}{}

		markerFileHandler, err := internal.NewMarkerFileHandler(logger, walCfg.Dir)
		if err != nil {
			return nil, err
		}
		markerHandler := internal.NewMarkerHandler(markerFileHandler, walCfg.MaxSegmentAge, logger, walMarkerMetrics.WithCurriedId(name))

		endpoint, err := newWalEndpoint(metrics, walEndpointMetrics.CurryWithId(name), cfg, logger, markerHandler)
		if err != nil {
			return nil, fmt.Errorf("error starting wal endpoint: %w", err)
		}

		// subscribe watcher's wal.WriteTo to writer events. This will make the writer trigger the cleanup of the wal.WriteTo
		// series cache whenever a segment is deleted.
		writer.SubscribeCleanup(endpoint)

		watcher := wal.NewWatcher(walCfg.Dir, name, walWatcherMetrics, endpoint, log.With(logger, "component", name), walCfg.WatchConfig, markerHandler)

		// subscribe watcher to wal write events
		writer.SubscribeWrite(watcher)

		level.Debug(logger).Log("msg", "starting WAL watcher for endpoint", "endpoint", name)
		watcher.Start()

		m.pairs = append(m.pairs, endpointWatcherPair{
			watcher:  watcher,
			endpoint: endpoint,
		})
	}

	writer.Start(walCfg.MaxSegmentAge)

	return m, nil
}

type endpointWatcherPair struct {
	watcher  *wal.Watcher
	endpoint *walEndpoint
}

// Stop will proceed to stop, in order, watcher and the endpoint.
func (p endpointWatcherPair) Stop(drain bool) {
	// If drain enabled, drain the WAL.
	if drain {
		p.watcher.Drain()
	}
	p.watcher.Stop()

	// Subsequently stop the endpoint.
	p.endpoint.Stop()
}

var _ DrainableConsumer = (*WALConsumer)(nil)

type WALConsumer struct {
	writer *wal.Writer
	pairs  []endpointWatcherPair
}

func (m *WALConsumer) Chan() chan<- loki.Entry {
	return m.writer.Chan()
}

func (m *WALConsumer) Stop() {
	m.stop(false)
}

// StopAndDrain will stop the manager, its WalWriter, Write-Ahead Log watchers,
// and queues accordingly. It attempt to drain the WAL completely.
func (m *WALConsumer) StopAndDrain() {
	m.stop(true)
}

func (m *WALConsumer) stop(drain bool) {
	m.writer.Stop()

	var stopWG sync.WaitGroup

	// Depending on whether drain is enabled, the maximum time stopping a watcher and it's queue can take is
	// the drain time of the watcher + drain time queue. To minimize this, and since we keep a separate WAL for each
	// endpoint config, each (watcher, queue) pair is stopped concurrently.
	for _, pair := range m.pairs {
		stopWG.Go(func() {
			pair.Stop(drain)
		})
	}

	// wait for all pairs to be stopped
	stopWG.Wait()
}

func newWalEndpoint(metrics *Metrics, wcMetrics *WALEndpointMetrics, cfg Config, logger log.Logger, markerHandler internal.MarkerHandler) (*walEndpoint, error) {
	logger = log.With(logger, "component", "endpoint", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, markerHandler, cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &walEndpoint{
		logger:    logger,
		cfg:       cfg,
		weMetrics: wcMetrics,
		shards:    shards,

		ctx:    ctx,
		cancel: cancel,

		series:        make(map[chunks.HeadSeriesRef]model.LabelSet),
		seriesSegment: make(map[chunks.HeadSeriesRef]int),

		markerHandler: markerHandler,
	}

	c.shards.start(cfg.QueueConfig.MinShards)

	return c, nil
}

// walEndpoint is a WAL-specific remote write implementation. This endpoint attests to the wal.WriteTo interface,
// which allows it to be injected in the wal.Watcher as a destination where to write read series and entries. As the watcher
// reads from the WAL, batches are created and dispatched onto a send queue when ready to be sent.
type walEndpoint struct {
	weMetrics *WALEndpointMetrics
	logger    log.Logger
	cfg       Config
	shards    *shards

	ctx    context.Context
	cancel context.CancelFunc

	// series cache
	series        map[chunks.HeadSeriesRef]model.LabelSet
	seriesSegment map[chunks.HeadSeriesRef]int
	seriesLock    sync.RWMutex

	markerHandler internal.MarkerHandler
}

func (c *walEndpoint) SeriesReset(segmentNum int) {
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

func (c *walEndpoint) StoreSeries(series []record.RefSeries, segment int) {
	c.seriesLock.Lock()
	defer c.seriesLock.Unlock()
	for _, seriesRec := range series {
		c.seriesSegment[seriesRec.Ref] = segment
		c.series[seriesRec.Ref] = promLabelsToModelLabels(seriesRec.Labels)
	}
}

func (c *walEndpoint) AppendEntries(entries wal.RefEntries, segment int) error {
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
	c.weMetrics.lastReadTimestamp.WithLabelValues().Set(float64(maxSeenTimestamp))

	return nil
}

func (c *walEndpoint) appendSingleEntry(entry loki.Entry, segmentNum int) bool {
	backoff := backoff.New(c.ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 50 * time.Millisecond,
	})
	for !c.shards.enqueue(entry, segmentNum) {
		if !backoff.Ongoing() {
			// we could not enqueue and endpoint is stopped.
			return false
		}
	}
	return true
}

// Stop the endpoint, enqueueing pending batches and draining the send queue accordingly. Both closing operations are
// limited by a deadline, controlled by a configured drain timeout, which is global to the Stop call.
func (c *walEndpoint) Stop() {
	// drain shards
	c.shards.stop()
	c.markerHandler.Stop()
}

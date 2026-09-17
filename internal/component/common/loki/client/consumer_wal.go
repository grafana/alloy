package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/marker"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
)

func NewWALConsumer(logger *slog.Logger, reg prometheus.Registerer, wl wal.WAL, walCfg wal.Config, cfgs ...Config) (*WALConsumer, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("at least one endpoint config must be provided")
	}

	writer := wal.NewWriter(logger, wal.NewWriterMetrics(reg), wl, walCfg)

	c := &WALConsumer{
		writer: writer,
		pairs:  make([]endpointWatcherPair, 0, len(cfgs)),
	}

	var (
		metrics        = newMetrics(reg)
		endpointsCheck = make(map[string]struct{})

		walWatcherMetrics  = wal.NewWatcherMetrics(reg)
		walMarkerMetrics   = marker.NewMetrics(reg)
		walEndpointMetrics = newWALEndpointMetrics(reg)
	)

	for _, cfg := range cfgs {
		// Don't allow duplicate endpoints, we have endpoint specific metrics that need at least one unique label value (name).
		name := getEndpointName(cfg)
		if _, ok := endpointsCheck[name]; ok {
			return nil, fmt.Errorf("duplicate endpoint configs are not allowed, found duplicate for name: %s", cfg.Name)
		}
		endpointsCheck[name] = struct{}{}

		pair, err := newEndpointWatcherPair(
			name,
			logger,
			wl.Dir(),
			walCfg,
			cfg,
			writer,
			metrics,
			walMarkerMetrics,
			walWatcherMetrics,
			walEndpointMetrics,
		)
		if err != nil {
			return nil, err
		}

		c.pairs = append(c.pairs, pair)
	}

	return c, nil
}

func newEndpointWatcherPair(
	name string,
	logger *slog.Logger,
	walDir string,
	walCfg wal.Config,
	cfg Config,
	writer *wal.Writer,
	metrics *metrics,
	markerMetrics *marker.Metrics,
	watcherMetrics *wal.WatcherMetrics,
	endpointMetrics *walEndpointMetrics,
) (endpointWatcherPair, error) {

	markerFile, err := marker.NewFile(logger, walDir)
	if err != nil {
		return endpointWatcherPair{}, err
	}
	tracker := marker.NewSegmentTracker(markerFile, walCfg.MaxSegmentAge, logger, markerMetrics.CurryWithId(name))

	endpoint, err := newEndpoint(metrics, cfg, logger, tracker)
	if err != nil {
		return endpointWatcherPair{}, fmt.Errorf("failed to create endpoint %s: %w", name, err)
	}

	adapter := newWalEndpointAdapter(endpoint, logger, endpointMetrics.CurryWithId(name), tracker)

	// The adapter caches series per segment, so it has to be told when the writer deletes
	// a segment to be able to reclaim them.
	writer.SubscribeCleanup(adapter)

	watcher := wal.NewWatcher(
		walDir,
		name,
		watcherMetrics,
		adapter,
		logger.With("component", name),
		walCfg.WatchConfig,
		tracker,
	)

	// subscribe watcher to wal write events
	writer.SubscribeWrite(watcher)

	return endpointWatcherPair{
		name:     name,
		logger:   logger,
		watcher:  watcher,
		endpoint: adapter,
	}, nil
}

type endpointWatcherPair struct {
	name     string
	logger   *slog.Logger
	watcher  *wal.Watcher
	endpoint *walEndpointAdapter
}

func (p endpointWatcherPair) start() {
	// The watcher forwards entries to the endpoint as soon as it runs, so the endpoint
	// has to be started first.
	p.endpoint.start()
	p.logger.Debug("starting WAL watcher for endpoint", "endpoint", p.name)
	p.watcher.Start()
}

func (p endpointWatcherPair) stop(drain bool) {
	// If drain enabled, drain the WAL.
	if drain {
		p.watcher.Drain()
	}

	// Watcher is stopped before the endpoint so it is not handed more entries
	// while the endpoint drains its queues.
	p.watcher.Stop()

	p.endpoint.stop()
}

var _ DrainableConsumer = (*WALConsumer)(nil)

type WALConsumer struct {
	writer *wal.Writer
	pairs  []endpointWatcherPair
}

func (c *WALConsumer) Start() {
	c.writer.Start()
	for _, e := range c.pairs {
		e.start()
	}
}

func (c *WALConsumer) ConsumeEntry(_ context.Context, entry loki.Entry) error {
	return c.writer.WriteEntry(entry)
}

// Stop stops the consumer without draining the WAL.
func (c *WALConsumer) Stop() {
	c.stop(false)
}

// StopAndDrain stops the writer first so nothing new enters the WAL, then drains
// what is left through the watchers and endpoints before stopping them.
func (c *WALConsumer) StopAndDrain() {
	c.stop(true)
}

func (c *WALConsumer) stop(drain bool) {
	c.writer.Stop()

	var stopWG sync.WaitGroup

	// Stopping a pair costs up to the watcher's drain timeout plus the queue's, so pairs
	// are stopped concurrently to pay that once rather than once per endpoint.
	for _, pair := range c.pairs {
		stopWG.Go(func() {
			pair.stop(drain)
		})
	}

	stopWG.Wait()
}

func newWalEndpointAdapter(endpoint *endpoint, logger *slog.Logger, metrics *walEndpointMetrics, tracker marker.Tracker) *walEndpointAdapter {
	c := &walEndpointAdapter{
		logger:   logger.With("component", "waladapter"),
		metrics:  metrics,
		endpoint: endpoint,

		series:        make(map[chunks.HeadSeriesRef]model.LabelSet),
		seriesSegment: make(map[chunks.HeadSeriesRef]int),

		tracker: tracker,
	}

	return c
}

// walEndpointAdapter is an adapter between watcher and endpoint. This component attests to the wal.WriteTo interface,
// which allows it to be injected in the wal.Watcher as a destination where to write series and entries. As the watcher
// reads from the WAL, entries are forwarded here so it can be written to endpoint.
type walEndpointAdapter struct {
	logger  *slog.Logger
	metrics *walEndpointMetrics

	endpoint *endpoint

	// series cache
	series        map[chunks.HeadSeriesRef]model.LabelSet
	seriesSegment map[chunks.HeadSeriesRef]int
	seriesLock    sync.RWMutex

	tracker marker.Tracker
}

func (c *walEndpointAdapter) SeriesReset(segmentNum int) {
	c.seriesLock.Lock()
	defer c.seriesLock.Unlock()
	for k, v := range c.seriesSegment {
		if v <= segmentNum {
			c.logger.Debug("reclaiming series", "segment", segmentNum)
			delete(c.seriesSegment, k)
			delete(c.series, k)
		}
	}
}

func (c *walEndpointAdapter) StoreSeries(series []record.RefSeries, segment int) {
	c.seriesLock.Lock()
	defer c.seriesLock.Unlock()
	for _, seriesRec := range series {
		c.seriesSegment[seriesRec.Ref] = segment
		c.series[seriesRec.Ref] = promLabelsToModelLabels(seriesRec.Labels)
	}
}

func (c *walEndpointAdapter) AppendEntries(ctx context.Context, entries wal.RefEntries, segment int) error {
	c.seriesLock.RLock()
	l, ok := c.series[entries.Ref]
	c.seriesLock.RUnlock()

	var (
		queuedEntries    int
		maxSeenTimestamp time.Time
	)

	if !ok {
		// TODO(thepalbi): Add metric here
		c.logger.Debug("series for entries not found")
		return nil
	}

	for i := range entries.Entries {
		entry := entries.EntryAt(l, i)
		err := c.endpoint.enqueue(ctx, entry, segment)

		// If we get errQueueIsFull we skipped the entry and should
		// not count it as queued and should move on to the next one.
		if errors.Is(err, errQueueIsFull) {
			continue
		}

		if err != nil {
			return err
		}

		queuedEntries += 1

		if entry.Timestamp.After(maxSeenTimestamp) {
			maxSeenTimestamp = entry.Timestamp
		}
	}
	// update tracker with all successfully queued entries.
	c.tracker.UpdateReceivedData(segment, queuedEntries)

	if queuedEntries > 0 {
		c.metrics.lastReadTimestamp.WithLabelValues().Set(float64(maxSeenTimestamp.Unix()))
	}

	return nil
}

func (c *walEndpointAdapter) start() {
	c.tracker.Start()
	c.endpoint.start()
}

func (c *walEndpointAdapter) stop() {
	// tracker is stopped after endpoint since endpoint will report sent data while it drains.
	c.endpoint.stop()
	c.tracker.Stop()
}

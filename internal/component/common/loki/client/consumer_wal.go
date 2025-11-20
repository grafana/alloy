package client

import (
	"fmt"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
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

		endpoint, err := newEndpoint(metrics, cfg, logger, markerHandler)
		if err != nil {
			return nil, fmt.Errorf("error starting endpoint: %w", err)
		}

		adapter := newWalEndpointAdapter(endpoint, logger, walEndpointMetrics, markerHandler)

		// subscribe watcher's wal.WriteTo to writer events. This will make the writer trigger the cleanup of the wal.WriteTo
		// series cache whenever a segment is deleted.
		writer.SubscribeCleanup(adapter)

		watcher := wal.NewWatcher(walCfg.Dir, name, walWatcherMetrics, adapter, log.With(logger, "component", name), walCfg.WatchConfig, markerHandler)

		// subscribe watcher to wal write events
		writer.SubscribeWrite(watcher)

		level.Debug(logger).Log("msg", "starting WAL watcher for endpoint", "endpoint", name)
		watcher.Start()

		m.pairs = append(m.pairs, endpointWatcherPair{
			watcher:  watcher,
			endpoint: adapter,
		})
	}

	writer.Start(walCfg.MaxSegmentAge)

	return m, nil
}

type endpointWatcherPair struct {
	watcher  *wal.Watcher
	endpoint *walEndpointAdapter
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

// StopAndDrain will stop the consumer, its WalWriter, Write-Ahead Log watchers,
// and endpoints accordingly. It attempt to drain the WAL completely.
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

func newWalEndpointAdapter(endpoint *endpoint, logger log.Logger, metrics *WALEndpointMetrics, markerHandler internal.MarkerHandler) *walEndpointAdapter {
	c := &walEndpointAdapter{
		logger:   log.With(logger, "component", "waladapter"),
		metrics:  metrics,
		endpoint: endpoint,

		series:        make(map[chunks.HeadSeriesRef]model.LabelSet),
		seriesSegment: make(map[chunks.HeadSeriesRef]int),

		markerHandler: markerHandler,
	}

	return c
}

// walEndpointAdapter is an adapter between watcher and endpoint. This component attests to the wal.WriteTo interface,
// which allows it to be injected in the wal.Watcher as a destination where to write series and entries. As the watcher
// reads from the WAL, entires are forwarded here so it can be written to endpoint.
type walEndpointAdapter struct {
	logger  log.Logger
	metrics *WALEndpointMetrics

	endpoint *endpoint

	// series cache
	series        map[chunks.HeadSeriesRef]model.LabelSet
	seriesSegment map[chunks.HeadSeriesRef]int
	seriesLock    sync.RWMutex

	markerHandler internal.MarkerHandler
}

func (c *walEndpointAdapter) SeriesReset(segmentNum int) {
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

func (c *walEndpointAdapter) StoreSeries(series []record.RefSeries, segment int) {
	c.seriesLock.Lock()
	defer c.seriesLock.Unlock()
	for _, seriesRec := range series {
		c.seriesSegment[seriesRec.Ref] = segment
		c.series[seriesRec.Ref] = promLabelsToModelLabels(seriesRec.Labels)
	}
}

func (c *walEndpointAdapter) AppendEntries(entries wal.RefEntries, segment int) error {
	c.seriesLock.RLock()
	l, ok := c.series[entries.Ref]
	c.seriesLock.RUnlock()
	var maxSeenTimestamp int64 = -1
	if ok {
		for _, e := range entries.Entries {
			ok := c.endpoint.enqueue(loki.Entry{Labels: l, Entry: e}, segment)
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
	c.metrics.lastReadTimestamp.WithLabelValues().Set(float64(maxSeenTimestamp))
	return nil
}

// Stop the endpoint, enqueueing pending batches and draining the send queue accordingly. Both closing operations are
// limited by a deadline, controlled by a configured drain timeout, which is global to the Stop call.
func (c *walEndpointAdapter) Stop() {
	c.endpoint.stop()
	c.markerHandler.Stop()
}

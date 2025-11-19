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

func NewWALConsumer(logger log.Logger, reg prometheus.Registerer, walCfg wal.Config, clientCfgs ...Config) (*WALConsumer, error) {
	if len(clientCfgs) == 0 {
		return nil, fmt.Errorf("at least one client config must be provided")
	}

	writer, err := wal.NewWriter(walCfg, logger, reg)
	if err != nil {
		return nil, fmt.Errorf("error creating wal writer: %w", err)
	}

	m := &WALConsumer{
		writer: writer,
		pairs:  make([]clientWatcherPair, 0, len(clientCfgs)),
	}

	var (
		metrics      = NewMetrics(reg)
		clientsCheck = make(map[string]struct{})

		walWatcherMetrics = wal.NewWatcherMetrics(reg)
		walMarkerMetrics  = internal.NewMarkerMetrics(reg)
		walClientMetrics  = NewWALClientMetrics(reg)
	)

	for _, cfg := range clientCfgs {
		// Don't allow duplicate clients, we have client specific metrics that need at least one unique label value (name).
		clientName := getClientName(cfg)
		if _, ok := clientsCheck[clientName]; ok {
			return nil, fmt.Errorf("duplicate client configs are not allowed, found duplicate for name: %s", cfg.Name)
		}
		clientsCheck[clientName] = struct{}{}

		// add some context information for the logger the watcher uses
		wlog := log.With(logger, "client", clientName)

		markerFileHandler, err := internal.NewMarkerFileHandler(logger, walCfg.Dir)
		if err != nil {
			return nil, err
		}
		markerHandler := internal.NewMarkerHandler(markerFileHandler, walCfg.MaxSegmentAge, logger, walMarkerMetrics.WithCurriedId(clientName))

		client, err := newWalClient(metrics, walClientMetrics.CurryWithId(clientName), cfg, logger, markerHandler)
		if err != nil {
			return nil, fmt.Errorf("error starting wal client: %w", err)
		}

		// subscribe watcher's wal.WriteTo to writer events. This will make the writer trigger the cleanup of the wal.WriteTo
		// series cache whenever a segment is deleted.
		writer.SubscribeCleanup(client)

		watcher := wal.NewWatcher(walCfg.Dir, clientName, walWatcherMetrics, client, wlog, walCfg.WatchConfig, markerHandler)

		// subscribe watcher to wal write events
		writer.SubscribeWrite(watcher)

		level.Debug(logger).Log("msg", "starting WAL watcher for client", "client", clientName)
		watcher.Start()

		m.pairs = append(m.pairs, clientWatcherPair{
			watcher: watcher,
			client:  client,
		})
	}

	writer.Start(walCfg.MaxSegmentAge)

	return m, nil
}

type clientWatcherPair struct {
	watcher *wal.Watcher
	client  *walClient
}

// Stop will proceed to stop, in order, watcher and the client.
func (p clientWatcherPair) Stop(drain bool) {
	// If drain enabled, drain the WAL.
	if drain {
		p.watcher.Drain()
	}
	p.watcher.Stop()

	// Subsequently stop the client.
	p.client.Stop()
}

var _ DrainableConsumer = (*WALConsumer)(nil)

type WALConsumer struct {
	writer *wal.Writer
	pairs  []clientWatcherPair
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
	// client config, each (watcher, queue) pair is stopped concurrently.
	for _, pair := range m.pairs {
		stopWG.Go(func() {
			pair.Stop(drain)
		})
	}

	// wait for all pairs to be stopped
	stopWG.Wait()
}

func newWalClient(metrics *Metrics, wcMetrics *WALClientMetrics, cfg Config, logger log.Logger, markerHandler internal.MarkerHandler) (*walClient, error) {
	logger = log.With(logger, "component", "client", "host", cfg.URL.Host)

	shards, err := newShards(metrics, logger, markerHandler, cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &walClient{
		logger:    logger,
		cfg:       cfg,
		wcMetrics: wcMetrics,
		shards:    shards,

		ctx:    ctx,
		cancel: cancel,

		series:        make(map[chunks.HeadSeriesRef]model.LabelSet),
		seriesSegment: make(map[chunks.HeadSeriesRef]int),

		markerHandler: markerHandler,
	}

	c.shards.start(cfg.Queue.MinShards)

	return c, nil
}

// walClient is a WAL-specific remote write client implementation. This client attests to the wal.WriteTo interface,
// which allows it to be injected in the wal.Watcher as a destination where to write read series and entries. As the watcher
// reads from the WAL, batches are created and dispatched onto a send queue when ready to be sent.
type walClient struct {
	wcMetrics *WALClientMetrics
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

func (c *walClient) SeriesReset(segmentNum int) {
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

func (c *walClient) StoreSeries(series []record.RefSeries, segment int) {
	c.seriesLock.Lock()
	defer c.seriesLock.Unlock()
	for _, seriesRec := range series {
		c.seriesSegment[seriesRec.Ref] = segment
		c.series[seriesRec.Ref] = promLabelsToModelLabels(seriesRec.Labels)
	}
}

func (c *walClient) AppendEntries(entries wal.RefEntries, segment int) error {
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
	c.wcMetrics.lastReadTimestamp.WithLabelValues().Set(float64(maxSeenTimestamp))

	return nil
}

func (c *walClient) appendSingleEntry(entry loki.Entry, segmentNum int) bool {
	backoff := backoff.New(c.ctx, backoff.Config{
		MinBackoff: 5 * time.Millisecond,
		MaxBackoff: 50 * time.Millisecond,
	})
	for !c.shards.enqueue(entry, segmentNum) {
		if !backoff.Ongoing() {
			// we could not enqueue and client is stopped.
			return false
		}
	}
	return true
}

// Stop the client, enqueueing pending batches and draining the send queue accordingly. Both closing operations are
// limited by a deadline, controlled by a configured drain timeout, which is global to the Stop call.
func (c *walClient) Stop() {
	// drain shards
	c.shards.stop()
	c.markerHandler.Stop()
}

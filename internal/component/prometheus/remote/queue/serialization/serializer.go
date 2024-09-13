package serialization

import (
	"context"
	"strconv"
	"time"

	snappy "github.com/eapache/go-xerial-snappy"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/vladopajic/go-actor/actor"
)

// serializer collects data from multiple appenders in-memory and will periodically flush the data to file.Storage.
// serializer will flush based on configured time duration OR if it hits a certain number of items.
type serializer struct {
	inbox               actor.Mailbox[*types.TimeSeriesBinary]
	metaInbox           actor.Mailbox[*types.TimeSeriesBinary]
	cfgInbox            actor.Mailbox[types.SerializerConfig]
	maxItemsBeforeFlush int
	flushFrequency      time.Duration
	queue               types.FileStorage
	lastFlush           time.Time
	logger              log.Logger
	self                actor.Actor
	// Every 1 second we should check if we need to flush.
	flushTestTimer *time.Ticker
	series         []*types.TimeSeriesBinary
	meta           []*types.TimeSeriesBinary
	msgpBuffer     []byte
	stats          func(stats types.SerializerStats)
}

func NewSerializer(cfg types.SerializerConfig, q types.FileStorage, stats func(stats types.SerializerStats), l log.Logger) (types.Serializer, error) {
	s := &serializer{
		maxItemsBeforeFlush: int(cfg.MaxSignalsInBatch),
		flushFrequency:      cfg.FlushFrequency,
		queue:               q,
		series:              make([]*types.TimeSeriesBinary, 0),
		logger:              l,
		inbox:               actor.NewMailbox[*types.TimeSeriesBinary](),
		metaInbox:           actor.NewMailbox[*types.TimeSeriesBinary](),
		cfgInbox:            actor.NewMailbox[types.SerializerConfig](),
		flushTestTimer:      time.NewTicker(1 * time.Second),
		msgpBuffer:          make([]byte, 0),
		lastFlush:           time.Now(),
		stats:               stats,
	}

	return s, nil
}
func (s *serializer) Start() {
	// All the actors and mailboxes need to start.
	s.queue.Start()
	s.self = actor.Combine(actor.New(s), s.inbox, s.metaInbox, s.cfgInbox).Build()
	s.self.Start()
}

func (s *serializer) Stop() {
	s.queue.Stop()
	s.self.Stop()
}

func (s *serializer) SendSeries(ctx context.Context, data *types.TimeSeriesBinary) error {
	return s.inbox.Send(ctx, data)
}

func (s *serializer) SendMetadata(ctx context.Context, data *types.TimeSeriesBinary) error {
	return s.metaInbox.Send(ctx, data)
}

func (s *serializer) UpdateConfig(ctx context.Context, cfg types.SerializerConfig) error {
	return s.cfgInbox.Send(ctx, cfg)
}

func (s *serializer) DoWork(ctx actor.Context) actor.WorkerStatus {
	// Check for config which should have priority. Selector is random but since incoming
	// series will always have a queue by explicitly checking the config here we always give it a chance.
	// By pulling the config from the mailbox we ensure it does NOT need a mutex around access.
	select {
	case <-ctx.Done():
		return actor.WorkerEnd
	case cfg, ok := <-s.cfgInbox.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		s.maxItemsBeforeFlush = int(cfg.MaxSignalsInBatch)
		s.flushFrequency = cfg.FlushFrequency
		return actor.WorkerContinue
	default:
	}

	select {
	case <-ctx.Done():
		return actor.WorkerEnd
	case item, ok := <-s.inbox.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		s.series = append(s.series, item)
		// If we would go over the max size then send, or if we have hit the flush duration then send.
		if len(s.meta)+len(s.series) >= s.maxItemsBeforeFlush {
			err := s.store(ctx)
			if err != nil {
				level.Error(s.logger).Log("msg", "unable to append to serializer", "err", err)
			}
		}

		return actor.WorkerContinue
	case item, ok := <-s.metaInbox.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		s.meta = append(s.meta, item)
		if len(s.meta)+len(s.series) >= s.maxItemsBeforeFlush {
			err := s.store(ctx)
			if err != nil {
				level.Error(s.logger).Log("msg", "unable to append metadata to serializer", "err", err)
			}
		}
		return actor.WorkerContinue
	case <-s.flushTestTimer.C:
		if time.Since(s.lastFlush) > s.flushFrequency {
			err := s.store(ctx)
			if err != nil {
				level.Error(s.logger).Log("msg", "unable to store data", "err", err)
			}
		}
		return actor.WorkerContinue
	}
}

func (s *serializer) store(ctx actor.Context) error {
	var err error
	defer func() {
		s.lastFlush = time.Now()
	}()
	// Do nothing if there is nothing.
	if len(s.series) == 0 && len(s.meta) == 0 {
		return nil
	}
	group := &types.SeriesGroup{
		Series:   make([]*types.TimeSeriesBinary, len(s.series)),
		Metadata: make([]*types.TimeSeriesBinary, len(s.meta)),
	}
	defer func() {
		s.storeStats(err)
		// Return series to the pool, this is key to reducing allocs.
		types.PutTimeSeriesSliceIntoPool(s.series)
		types.PutTimeSeriesSliceIntoPool(s.meta)
		s.series = s.series[:0]
		s.meta = s.series[:0]
	}()

	// This maps strings to index position in a slice. This is doing to reduce the file size of the data.
	strMapToInt := make(map[string]uint32)
	for i, ts := range s.series {
		ts.FillLabelMapping(strMapToInt)
		group.Series[i] = ts
	}
	for i, ts := range s.meta {
		ts.FillLabelMapping(strMapToInt)
		group.Metadata[i] = ts
	}

	stringsSlice := make([]string, len(strMapToInt))
	for stringValue, index := range strMapToInt {
		stringsSlice[index] = stringValue
	}
	group.Strings = stringsSlice

	buf, err := group.MarshalMsg(s.msgpBuffer)
	if err != nil {
		return err
	}

	out := snappy.Encode(buf)
	meta := map[string]string{
		// product.signal_type.schema.version
		"version":       "alloy.metrics.queue.v1",
		"compression":   "snappy",
		"series_count":  strconv.Itoa(len(group.Series)),
		"meta_count":    strconv.Itoa(len(group.Metadata)),
		"strings_count": strconv.Itoa(len(group.Strings)),
	}
	err = s.queue.Store(ctx, meta, out)
	return err
}

func (s *serializer) storeStats(err error) {
	hasError := 0
	if err != nil {
		hasError = 1
	}
	newestTS := int64(0)
	for _, ts := range s.series {
		if ts.TS > newestTS {
			newestTS = ts.TS

		}
	}
	s.stats(types.SerializerStats{
		SeriesStored:    len(s.series),
		MetadataStored:  len(s.meta),
		Errors:          hasError,
		NewestTimestamp: newestTS,
	})
}

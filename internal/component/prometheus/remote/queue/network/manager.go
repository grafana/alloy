package network

import (
	"context"

	"github.com/grafana/alloy/internal/runtime/logging/level"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/vladopajic/go-actor/actor"
)

// manager manages loops. Mostly it exists to control their lifecycle and send work to them.
type manager struct {
	connectionCount uint64
	loops           []*loop
	metadata        *loop
	logger          log.Logger
	inbox           actor.Mailbox[*types.TimeSeriesBinary]
	metaInbox       actor.Mailbox[*types.TimeSeriesBinary]
	configInbox     actor.Mailbox[ConnectionConfig]
	self            actor.Actor
	cfg             ConnectionConfig
	stats           func(types.NetworkStats)
	metaStats       func(types.NetworkStats)
}

var _ types.NetworkClient = (*manager)(nil)

var _ actor.Worker = (*manager)(nil)

func New(cc ConnectionConfig, logger log.Logger, seriesStats, metadataStats func(types.NetworkStats)) (types.NetworkClient, error) {
	s := &manager{
		connectionCount: cc.Connections,
		loops:           make([]*loop, 0),
		logger:          logger,
		// This provides blocking to only handle one at a time, so that if a queue blocks
		// it will stop the filequeue from feeding more.
		inbox:       actor.NewMailbox[*types.TimeSeriesBinary](actor.OptCapacity(1)),
		metaInbox:   actor.NewMailbox[*types.TimeSeriesBinary](actor.OptCapacity(1)),
		configInbox: actor.NewMailbox[ConnectionConfig](),
		stats:       seriesStats,
	}

	// start kicks off a number of concurrent connections.
	var i uint64
	for ; i < s.connectionCount; i++ {
		l := newLoop(cc, false, logger, seriesStats)
		l.self = actor.New(l)
		s.loops = append(s.loops, l)
	}

	s.metadata = newLoop(cc, true, logger, metadataStats)
	s.metadata.self = actor.New(s.metadata)
	return s, nil
}

func (s *manager) Start() {
	actors := make([]actor.Actor, 0)
	for _, l := range s.loops {
		l.Start()
	}
	actors = append(actors, s.metadata.actors()...)
	actors = append(actors, s.inbox)
	actors = append(actors, s.metaInbox)
	actors = append(actors, actor.New(s))
	actors = append(actors, s.configInbox)
	s.self = actor.Combine(actors...).Build()
	s.self.Start()
}

func (s *manager) SendSeries(ctx context.Context, data *types.TimeSeriesBinary) error {
	return s.inbox.Send(ctx, data)
}

func (s *manager) SendMetadata(ctx context.Context, data *types.TimeSeriesBinary) error {
	return s.metaInbox.Send(ctx, data)
}

func (s *manager) UpdateConfig(ctx context.Context, cc ConnectionConfig) error {
	return s.configInbox.Send(ctx, cc)
}

func (s *manager) DoWork(ctx actor.Context) actor.WorkerStatus {
	// This acts as a priority queue, always check for configuration changes first.
	select {
	case cfg, ok := <-s.configInbox.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		s.updateConfig(cfg)
		return actor.WorkerContinue
	default:
	}
	select {
	case <-ctx.Done():
		s.Stop()
		return actor.WorkerEnd
	case ts, ok := <-s.inbox.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		s.queue(ctx, ts)
		return actor.WorkerContinue
	case ts, ok := <-s.metaInbox.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		err := s.metadata.seriesMbx.Send(ctx, ts)
		if err != nil {
			level.Error(s.logger).Log("msg", "failed to send to metadata loop", "err", err)
		}
		return actor.WorkerContinue
	}
}

func (s *manager) updateConfig(cc ConnectionConfig) {
	// No need to do anything if the configuration is the same.
	if s.cfg.Equals(cc) {
		return
	}
	// TODO @mattdurham make this smarter, at the moment any samples in the loops are lost.
	// Ideally we would drain the queues and re add them but that is a future need.
	// In practice this shouldn't change often so data loss should be minimal.
	// For the moment we will stop all the items and recreate them.
	for _, l := range s.loops {
		l.Stop()
	}
	s.metadata.Stop()

	s.loops = make([]*loop, 0)
	var i uint64
	for ; i < s.connectionCount; i++ {
		l := newLoop(cc, false, s.logger, s.stats)
		l.self = actor.New(l)
		s.loops = append(s.loops, l)
	}

	s.metadata = newLoop(cc, true, s.logger, s.metaStats)
	s.metadata.self = actor.New(s.metadata)
}

func (s *manager) Stop() {
	level.Debug(s.logger).Log("msg", "stopping manager")
	for _, l := range s.loops {
		l.Stop()
		l.stopCalled.Store(true)
	}
	s.metadata.stopCalled.Store(true)
	s.self.Stop()
}

// Queue adds anything thats not metadata to the queue.
func (s *manager) queue(ctx context.Context, ts *types.TimeSeriesBinary) {
	// Based on a hash which is the label hash add to the queue.
	queueNum := ts.Hash % s.connectionCount
	// This will block if the queue is full.
	err := s.loops[queueNum].seriesMbx.Send(ctx, ts)
	if err != nil {
		level.Error(s.logger).Log("msg", "failed to send to loop", "err", err)
	}
}

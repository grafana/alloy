package network

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/write/queue/types"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/vladopajic/go-actor/actor"
)

// manager manages loops. Mostly it exists to control their lifecycle and send work to them.
type manager struct {
	loops       []*loop
	metadata    *loop
	logger      log.Logger
	inbox       *types.Mailbox[*types.TimeSeriesBinary]
	metaInbox   *types.Mailbox[*types.TimeSeriesBinary]
	configInbox *types.SyncMailbox[types.ConnectionConfig]
	self        actor.Actor
	cfg         types.ConnectionConfig
	stats       func(types.NetworkStats)
	metaStats   func(types.NetworkStats)
}

var _ types.NetworkClient = (*manager)(nil)

var _ actor.Worker = (*manager)(nil)

func New(cc types.ConnectionConfig, logger log.Logger, seriesStats, metadataStats func(types.NetworkStats)) (types.NetworkClient, error) {
	s := &manager{
		loops:  make([]*loop, 0, cc.Connections),
		logger: logger,
		// This provides blocking to only handle one at a time, so that if a queue blocks
		// it will stop the filequeue from feeding more. Without passing true the minimum is actually 64 instead of 1.
		inbox:       types.NewMailbox[*types.TimeSeriesBinary](1, true),
		metaInbox:   types.NewMailbox[*types.TimeSeriesBinary](1, true),
		configInbox: types.NewSyncMailbox[types.ConnectionConfig](),
		stats:       seriesStats,
		metaStats:   metadataStats,
		cfg:         cc,
	}

	// start kicks off a number of concurrent connections.
	for i := uint(0); i < s.cfg.Connections; i++ {
		l := newLoop(cc, false, logger, seriesStats)
		l.self = actor.New(l)
		s.loops = append(s.loops, l)
	}

	s.metadata = newLoop(cc, true, logger, metadataStats)
	s.metadata.self = actor.New(s.metadata)
	return s, nil
}

func (s *manager) Start() {
	s.startLoops()
	s.configInbox.Start()
	s.metaInbox.Start()
	s.inbox.Start()
	s.self = actor.New(s)
	s.self.Start()
}

func (s *manager) SendSeries(ctx context.Context, data *types.TimeSeriesBinary) error {
	return s.inbox.Send(ctx, data)
}

func (s *manager) SendMetadata(ctx context.Context, data *types.TimeSeriesBinary) error {
	return s.metaInbox.Send(ctx, data)
}

func (s *manager) UpdateConfig(ctx context.Context, cc types.ConnectionConfig) error {
	return s.configInbox.Send(ctx, cc)
}

func (s *manager) DoWork(ctx actor.Context) actor.WorkerStatus {
	// This acts as a priority queue, always check for configuration changes first.
	select {
	case cfg, ok := <-s.configInbox.ReceiveC():
		defer cfg.Notify()
		if !ok {
			level.Debug(s.logger).Log("msg", "config inbox closed")
			return actor.WorkerEnd
		}
		s.updateConfig(cfg.Value)
		return actor.WorkerContinue
	default:
	}

	// main work queue.
	select {
	case <-ctx.Done():
		s.Stop()
		return actor.WorkerEnd
	case ts, ok := <-s.inbox.ReceiveC():
		if !ok {
			level.Debug(s.logger).Log("msg", "series inbox closed")
			return actor.WorkerEnd
		}
		s.queue(ctx, ts)
		return actor.WorkerContinue
	case ts, ok := <-s.metaInbox.ReceiveC():
		if !ok {
			level.Debug(s.logger).Log("msg", "meta inbox closed")
			return actor.WorkerEnd
		}
		err := s.metadata.seriesMbx.Send(ctx, ts)
		if err != nil {
			level.Error(s.logger).Log("msg", "failed to send to metadata loop", "err", err)
		}
		return actor.WorkerContinue
	case cfg, ok := <-s.configInbox.ReceiveC():
		// We need to also check the config here, else its possible this will deadlock.
		if !ok {
			level.Debug(s.logger).Log("msg", "config inbox closed")
			return actor.WorkerEnd
		}
		defer cfg.Notify()
		s.updateConfig(cfg.Value)
		return actor.WorkerContinue
	}
}

func (s *manager) updateConfig(cc types.ConnectionConfig) {
	// No need to do anything if the configuration is the same.
	if s.cfg.Equals(cc) {
		return
	}
	s.cfg = cc
	// TODO @mattdurham make this smarter, at the moment any samples in the loops are lost.
	// Ideally we would drain the queues and re add them but that is a future need.
	// In practice this shouldn't change often so data loss should be minimal.
	// For the moment we will stop all the items and recreate them.
	level.Debug(s.logger).Log("msg", "dropping all series in loops and creating queue due to config change")
	s.stopLoops()
	s.loops = make([]*loop, 0, s.cfg.Connections)
	for i := uint(0); i < s.cfg.Connections; i++ {
		l := newLoop(cc, false, s.logger, s.stats)
		l.self = actor.New(l)
		s.loops = append(s.loops, l)
	}

	s.metadata = newLoop(cc, true, s.logger, s.metaStats)
	s.metadata.self = actor.New(s.metadata)
	level.Debug(s.logger).Log("msg", "starting loops")
	s.startLoops()
	level.Debug(s.logger).Log("msg", "loops started")
}

func (s *manager) Stop() {
	s.stopLoops()
	s.configInbox.Stop()
	s.metaInbox.Stop()
	s.inbox.Stop()
	s.self.Stop()
}

func (s *manager) stopLoops() {
	for _, l := range s.loops {
		l.Stop()
	}
	s.metadata.Stop()
}

func (s *manager) startLoops() {
	for _, l := range s.loops {
		l.Start()
	}
	s.metadata.Start()
}

// Queue adds anything thats not metadata to the queue.
func (s *manager) queue(ctx context.Context, ts *types.TimeSeriesBinary) {
	// Based on a hash which is the label hash add to the queue.
	queueNum := ts.Hash % uint64(s.cfg.Connections)
	// This will block if the queue is full.
	err := s.loops[queueNum].seriesMbx.Send(ctx, ts)
	if err != nil {
		level.Error(s.logger).Log("msg", "failed to send to loop", "err", err)
	}
}

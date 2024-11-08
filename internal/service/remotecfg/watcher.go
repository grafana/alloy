package remotecfg

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
)

type Watcher struct {
	logger log.Logger
	client collectorv1connect.CollectorServiceClient
	reg    *collectorv1.RegisterCollectorRequest
}

type RemoteConfigUpdate struct {
	Content string
	Err     error
}

func NewWatcher(
	l log.Logger,
	client collectorv1connect.CollectorServiceClient,
	def *collectorv1.RegisterCollectorRequest,
) *Watcher {
	return &Watcher{
		logger: l,
		client: client,
		reg:    def,
	}
}

func (w *Watcher) Run(ctx context.Context, updates chan<- RemoteConfigUpdate) {
	// TODO: make sure in all cases we return asap if ctx is cancelled

	for {
		err := w.registerCollector(ctx)
		if err == nil {
			break
		}
		level.Error(w.logger).Log("msg", "error registering collector", "err", err)
		// if error was because of context, return now
		if ctx.Err() != nil {
			return
		}
		updates <- RemoteConfigUpdate{
			Err: err,
		}
		// todo: exponential backoff with "github.com/grafana/dskit/backoff"
		time.Sleep(5 * time.Second)
	}
	// todo: jitter ticker with correct interval
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticker.C:
			err := w.fetch(ctx)
			if err != nil {
				level.Error(w.logger).Log("msg", "error fetching remote config", "err", err)
				// if error was because of context, return now
				if ctx.Err() != nil {
					return
				}
				updates <- RemoteConfigUpdate{
					Err: err,
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *Watcher) registerCollector(ctx context.Context) error {
	req := connect.NewRequest(w.reg)

	_, err := w.client.RegisterCollector(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (w *Watcher) fetch(ctx context.Context) error {
	return nil
}

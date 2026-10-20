package cloudflare

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/concurrency"
	"github.com/grafana/dskit/multierror"
	"github.com/grafana/jsonparser"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
)

// The minimum window size is 1 minute.
const minDelay = time.Minute

var cloudflareTooEarlyError = regexp.MustCompile(`too early: logs older than \S+ are not available`)

var defaultBackoff = backoff.Config{
	MinBackoff: 1 * time.Second,
	MaxBackoff: 10 * time.Second,
	MaxRetries: 5,
}

// tailerConfig defines how to connect to Cloudflare's Logpull API.
type tailerConfig struct {
	APIToken         string
	ZoneID           string
	APIURL           string
	Labels           model.LabelSet
	Workers          int
	PullRange        time.Duration
	FieldsType       FieldsType
	AdditionalFields []string
	Backoff          backoff.Config
}

// tailer is responsible to pull log messages from cloudflare using Logpull API.
type tailer struct {
	logger    *slog.Logger
	metrics   *metrics
	handler   loki.LogsReceiver
	positions positions.Positions
	config    *tailerConfig
	fields    []string

	client  Client
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running *atomic.Bool
	err     *atomic.Error
}

// newTailer creates and runs a Cloudflare tailer.
func newTailer(metrics *metrics, logger *slog.Logger, handler loki.LogsReceiver, position positions.Positions, config *tailerConfig) (*tailer, error) {
	fields, err := fieldsForType(config.FieldsType, config.AdditionalFields)
	if err != nil {
		return nil, err
	}
	client, err := getClient(config.APIURL, config.APIToken, config.ZoneID, fields)
	if err != nil {
		return nil, err
	}
	pos, err := position.Get(positions.CursorKey(config.ZoneID), config.Labels.String())
	if err != nil {
		return nil, err
	}
	to := time.Now()
	if pos != 0 {
		to = time.Unix(0, pos)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &tailer{
		logger:    logger,
		metrics:   metrics,
		positions: position,
		handler:   handler,
		config:    config,
		fields:    fields,

		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		running: atomic.NewBool(false),
		err:     atomic.NewError(nil),
	}

	t.start(to)

	return t, nil
}

// start pulls windows from to, the end of the next pull interval.
func (t *tailer) start(to time.Time) {
	t.running.Store(true)

	t.wg.Go(func() {
		defer t.running.Store(false)

		for {
			if t.ctx.Err() != nil {
				return
			}

			end := to
			maxEnd := time.Now().Add(-minDelay)
			if end.After(maxEnd) {
				end = maxEnd
			}
			start := end.Add(-t.config.PullRange)
			requests := splitRequests(start, end, t.config.Workers)

			// Stopping cancels the window halfway through. Its position is not saved, so it is pulled again on restart.
			// In case of errors we stop the target, each worker has its own retry logic.
			err := concurrency.ForEachJob(t.ctx, len(requests), t.config.Workers, func(ctx context.Context, idx int) error {
				request := requests[idx]
				return t.pull(ctx, request.start, request.end)
			})

			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				t.logger.Error("failed to pull logs", "err", err, "start", start, "end", end)
				t.err.Store(err)
				return
			}

			// Sets current timestamp metrics, move to the next interval and saves the position.
			t.metrics.LastEnd.Set(float64(end.UnixNano()) / 1e9)
			to = end.Add(t.config.PullRange)
			t.positions.Put(positions.CursorKey(t.config.ZoneID), t.config.Labels.String(), to.UnixNano())

			// If the next window can be fetched do it, if not sleep for a while.
			// This is because Cloudflare logs should never be pulled between now-1m and now.
			diff := to.Sub(time.Now().Add(-minDelay))
			if diff > 0 {
				select {
				case <-time.After(diff):
				case <-t.ctx.Done():
				}
			}
		}
	})
}

// pull pulls logs from cloudflare for a given time range.
// It will retry on errors.
func (t *tailer) pull(ctx context.Context, start, end time.Time) error {
	var (
		errs    = multierror.New()
		backoff = backoff.New(ctx, t.config.Backoff)
	)

	for backoff.Ongoing() {
		it, err := t.client.LogpullReceived(ctx, start, end)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if cloudflareTooEarlyError.MatchString(err.Error()) {
				t.logger.Warn("failed iterating over logs, out of cloudflare range, not retrying", "err", err, "start", start, "end", end, "retries", backoff.NumRetries())
				return nil
			}
			errs.Add(err)
			backoff.Wait()
			continue
		}

		consume := func() error {
			defer it.Close()
			var lineRead int64
			for it.Next() {
				line := it.Line()
				ts, err := jsonparser.GetInt(line, "EdgeStartTimestamp")
				if err != nil {
					ts = time.Now().UnixNano()
				}

				// TODO(kalleep): pretty sure we don't have to clone here.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case t.handler.Chan() <- loki.NewEntry(t.config.Labels.Clone(), push.Entry{
					Timestamp: time.Unix(0, ts),
					Line:      string(line),
				}):
				}

				lineRead++
				t.metrics.Entries.Inc()
			}
			if it.Err() != nil {
				t.logger.Warn("failed iterating over logs", "err", it.Err(), "start", start, "end", end, "retries", backoff.NumRetries(), "lineRead", lineRead)
				return it.Err()
			}
			return nil
		}

		if err := consume(); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}

			errs.Add(err)
			backoff.Wait()
			continue
		}

		return nil
	}

	return errs.Err()
}

// stop shuts down the target.
func (t *tailer) stop() {
	t.cancel()
	t.wg.Wait()
}

// ready reports whether the target is ready.
func (t *tailer) ready() bool {
	return t.running.Load()
}

// details returns debug details about the Cloudflare target.
func (t *tailer) details() map[string]string {
	var errMsg string
	if err := t.err.Load(); err != nil {
		errMsg = err.Error()
	}

	var lastTimestamp string
	if pos, _ := t.positions.Get(positions.CursorKey(t.config.ZoneID), t.config.Labels.String()); pos != 0 {
		lastTimestamp = time.Unix(0, pos).String()
	}

	return map[string]string{
		"zone_id":        t.config.ZoneID,
		"error":          errMsg,
		"position":       t.positions.GetString(positions.CursorKey(t.config.ZoneID), t.config.Labels.String()),
		"last_timestamp": lastTimestamp,
		"fields":         strings.Join(t.fields, ","),
	}
}

type pullRequest struct {
	start time.Time
	end   time.Time
}

func splitRequests(start, end time.Time, workers int) []pullRequest {
	perWorker := end.Sub(start) / time.Duration(workers)
	var requests []pullRequest
	for i := range workers {
		r := pullRequest{
			start: start.Add(time.Duration(i) * perWorker),
			end:   start.Add(time.Duration(i+1) * perWorker),
		}
		// If the last worker is smaller than the others, we need to make sure it gets the last chunk.
		if i == workers-1 && r.end != end {
			r.end = end
		}
		requests = append(requests, r)
	}
	return requests
}

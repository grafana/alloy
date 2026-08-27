package usagestats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/multierror"

	"github.com/grafana/alloy/internal/alloyseed"
)

var (
	reportCheckInterval = time.Minute
	reportInterval      = 4 * time.Hour

	// reportCheckIntervalOverride and reportIntervalOverride are empty in production.
	// They exist only so an integration test build can shorten the reporting cadence via
	// `-ldflags -X` (which can set string vars, not time.Duration);
	reportCheckIntervalOverride = ""
	reportIntervalOverride      = ""

	reportBackoffConfig = backoff.Config{
		MinBackoff: time.Second,
		MaxBackoff: 30 * time.Second,
		MaxRetries: 5,
	}
)

// reporter holds the Alloy seed information and sends report of usage
type reporter struct {
	logger *slog.Logger

	seed       *alloyseed.Seed
	lastReport time.Time
}

// StartReporter launches a usage stats reporter in a background goroutine that
// reports the given tracker's metrics until ctx is cancelled. It owns seed
// initialization, reporter construction and the goroutine.
// seedDir is where the anonymous instance seed is persisted (empty means the legacy default path).
func StartReporter(ctx context.Context, logger *slog.Logger, seedDir string, tracker *Tracker) {
	alloyseed.Init(seedDir, logger)
	rep := &reporter{logger: logger}
	go func() {
		if err := rep.run(ctx, tracker.Metrics); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("failed to run usage stats reporter", "err", err)
		}
	}()
}

// run inits the reporter seed and start sending report for every interval
func (rep *reporter) run(ctx context.Context, metricsFunc func() map[string]any) error {
	rep.logger.Info("running usage stats reporter")
	rep.seed = alloyseed.Get()

	checkInterval, err := resolveInterval("reportCheckIntervalOverride", reportCheckIntervalOverride, reportCheckInterval)
	if err != nil {
		return err
	}
	interval, err := resolveInterval("reportIntervalOverride", reportIntervalOverride, reportInterval)
	if err != nil {
		return err
	}

	// check every checkInterval if we should report.
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// find when to send the next report.
	next := nextReport(interval, rep.seed.CreatedAt, time.Now())
	if rep.lastReport.IsZero() {
		// if we never reported assumed it was the last interval.
		rep.lastReport = next.Add(-interval)
	}
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			if !next.Equal(now) && now.Sub(rep.lastReport) < interval {
				continue
			}
			rep.logger.Info("reporting Alloy stats", "date", time.Now())
			err := rep.reportUsage(ctx, next, metricsFunc())
			if err != nil {
				rep.logger.Warn("failed to report usage", "err", err)
			}
			// Advance the schedule whether or not the report succeeded.
			// reportUsage already retries with backoff; on persistent failure
			// (e.g. the endpoint won't resolve) we treat the window as spent and
			// wait a full interval rather than re-attempting on every tick, which
			// would otherwise hammer the endpoint indefinitely.
			rep.lastReport = next
			next = next.Add(interval)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// reportUsage reports the usage to grafana.com.
func (rep *reporter) reportUsage(ctx context.Context, interval time.Time, metrics map[string]any) error {
	backoff := backoff.New(ctx, reportBackoffConfig)
	var errs multierror.MultiError
	for backoff.Ongoing() {
		if err := sendReport(ctx, rep.seed, interval, metrics); err != nil {
			rep.logger.Debug("failed to send usage report", "retries", backoff.NumRetries(), "err", err)
			errs.Add(err)
			backoff.Wait()
			continue
		}
		rep.logger.Info("usage report sent with success")
		return nil
	}
	return errs.Err()
}

// nextReport compute the next report time based on the interval. The interval
// is based off the creation of the Alloy seed to avoid all Alloy instances
// reporting at the same time.
func nextReport(interval time.Duration, createdAt, now time.Time) time.Time {
	duration := math.Ceil(float64(now.Sub(createdAt)) / float64(interval))
	return createdAt.Add(time.Duration(duration) * interval)
}

func resolveInterval(name, override string, def time.Duration) (time.Duration, error) {
	if override == "" {
		return def, nil
	}
	d, err := time.ParseDuration(override)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, override, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %q", name, override)
	}
	return d, nil
}

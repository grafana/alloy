package usagestats

import (
	"context"
	"math"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/agentseed"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/multierror"
)

var (
	reportCheckInterval = time.Minute
	reportInterval      = 4 * time.Hour
)

// Reporter holds the agent seed information and sends report of usage
type Reporter struct {
	logger log.Logger

	agentSeed  *agentseed.AgentSeed
	lastReport time.Time
}

// NewReporter creates a Reporter that will send periodically reports to grafana.com
func NewReporter(logger log.Logger) (*Reporter, error) {
	r := &Reporter{
		logger: logger,
	}
	return r, nil
}

// Start inits the reporter seed and start sending report for every interval
func (rep *Reporter) Start(ctx context.Context, metricsFunc func() map[string]interface{}) error {
	level.Info(rep.logger).Log("msg", "running usage stats reporter")
	rep.agentSeed = agentseed.Get()

	// check every minute if we should report.
	ticker := time.NewTicker(reportCheckInterval)
	defer ticker.Stop()

	// find  when to send the next report.
	next := nextReport(reportInterval, rep.agentSeed.CreatedAt, time.Now())
	if rep.lastReport.IsZero() {
		// if we never reported assumed it was the last interval.
		rep.lastReport = next.Add(-reportInterval)
	}
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			if !next.Equal(now) && now.Sub(rep.lastReport) < reportInterval {
				continue
			}
			level.Info(rep.logger).Log("msg", "reporting agent stats", "date", time.Now())
			if err := rep.reportUsage(ctx, next, metricsFunc()); err != nil {
				level.Info(rep.logger).Log("msg", "failed to report usage", "err", err)
				continue
			}
			rep.lastReport = next
			next = next.Add(reportInterval)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// reportUsage reports the usage to grafana.com.
func (rep *Reporter) reportUsage(ctx context.Context, interval time.Time, metrics map[string]interface{}) error {
	backoff := backoff.New(ctx, backoff.Config{
		MinBackoff: time.Second,
		MaxBackoff: 30 * time.Second,
		MaxRetries: 5,
	})
	var errs multierror.MultiError
	for backoff.Ongoing() {
		if err := sendReport(ctx, rep.agentSeed, interval, metrics); err != nil {
			level.Info(rep.logger).Log("msg", "failed to send usage report", "retries", backoff.NumRetries(), "err", err)
			errs.Add(err)
			backoff.Wait()
			continue
		}
		level.Info(rep.logger).Log("msg", "usage report sent with success")
		return nil
	}
	return errs.Err()
}

// nextReport compute the next report time based on the interval.
// The interval is based off the creation of the agent seed to avoid all agents reporting at the same time.
func nextReport(interval time.Duration, createdAt, now time.Time) time.Time {
	duration := math.Ceil(float64(now.Sub(createdAt)) / float64(interval))
	return createdAt.Add(time.Duration(duration) * interval)
}

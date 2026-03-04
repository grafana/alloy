package database_observability

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ConnectionCheckInterval is how often the connection_info collector pings the DB to verify connectivity.
const ConnectionCheckInterval = 60 * time.Second

// ConnectionChecksThreshold is the number of consecutive failed pings before unregistering the metric,
// and the number of consecutive successful pings before re-registering it after a disconnect.
const ConnectionChecksThreshold = 3

// ConnectionInfoMonitorConfig optionally overrides the default check interval and threshold.
// Used by tests to run the monitor with shorter intervals. If nil, defaults are used.
type ConnectionInfoMonitorConfig struct {
	CheckInterval   time.Duration
	ChecksThreshold int
}

// RunConnectionInfoMonitor starts a goroutine that pings db every ConnectionCheckInterval.
// After ConnectionChecksThreshold consecutive ping failures it unregisters infoMetric from registry.
// After ConnectionChecksThreshold consecutive ping successes (when the metric is unregistered) it re-registers
// infoMetric and sets it to 1 with the given labelValues.
// The goroutine runs until ctx is done. onStopped is called when the goroutine exits (e.g. when ctx is cancelled).
// RunConnectionInfoMonitor returns a cancel function that cancels the context passed to the goroutine; the caller
// should call cancel in Stop() to ensure the goroutine exits.
// labelValues must contain exactly 6 values in order: provider_name, provider_region, provider_account,
// db_instance_identifier, engine, engine_version.
// If config is non-nil, its CheckInterval and ChecksThreshold override the default constants (used for testing).
func RunConnectionInfoMonitor(ctx context.Context, db *sql.DB, registry *prometheus.Registry, infoMetric *prometheus.GaugeVec, labelValues []string, onStopped func(), config *ConnectionInfoMonitorConfig) (cancel context.CancelFunc) {
	interval := ConnectionCheckInterval
	threshold := ConnectionChecksThreshold
	if config != nil {
		if config.CheckInterval > 0 {
			interval = config.CheckInterval
		}
		if config.ChecksThreshold > 0 {
			threshold = config.ChecksThreshold
		}
	}
	ctx, cancel = context.WithCancel(ctx)
	go func() {
		defer onStopped()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var consecutiveFailures, consecutiveSuccesses int
		metricRegistered := true
		for {
			if err := db.PingContext(ctx); err != nil {
				consecutiveFailures++
				consecutiveSuccesses = 0
				if metricRegistered && consecutiveFailures >= threshold {
					registry.Unregister(infoMetric)
					metricRegistered = false
					consecutiveFailures = 0
				}
			} else {
				consecutiveFailures = 0
				if metricRegistered {
					consecutiveSuccesses = 0
				} else {
					consecutiveSuccesses++
					if consecutiveSuccesses >= threshold {
						registry.MustRegister(infoMetric)
						infoMetric.WithLabelValues(labelValues[0], labelValues[1], labelValues[2], labelValues[3], labelValues[4], labelValues[5]).Set(1)
						metricRegistered = true
						consecutiveSuccesses = 0
					}
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// continue loop
			}
		}
	}()
	return cancel
}

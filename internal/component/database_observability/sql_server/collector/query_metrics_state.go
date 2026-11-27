package collector

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// queryMetricsOrigin identifies the set of cumulative totals a query's metrics
// were read from. Totals from different origins aren't comparable, for example
// after a failover changes the connected database's replica role.
type queryMetricsOrigin struct {
	mode     queryStoreMode
	roleType int
}

// queryMetricSource is one query_hash row returned by the Query Store query.
// The executions/errors/duration values are the cumulative totals retained by
// Query Store for the query, not per-poll deltas.
type queryMetricSource struct {
	database        string
	queryHash       string
	executions      int64
	errors          int64
	durationSeconds float64
	origin          queryMetricsOrigin
}

type queryMetricsKey struct {
	database  string
	queryHash string
}

type queryMetricsEntry struct {
	rawExecutions      int64
	rawErrors          int64
	rawDurationSeconds float64
	origin             queryMetricsOrigin
	lastSeen           time.Time
}

// queryMetricsState turns the cumulative totals reported by Query Store into
// monotonic Prometheus counters. Each poll's absolute totals are folded into
// non-negative deltas; when any total decreases (Query Store cleared, retention
// or plan cleanup) or the totals are read from a different origin (the replica
// role changed) the series is re-baselined instead of emitting a spurious
// spike. Series that have not been observed within the TTL window are deleted so
// registry cardinality stays bounded when a query leaves the top-N.
type queryMetricsState struct {
	executions *prometheus.CounterVec
	errors     *prometheus.CounterVec
	duration   *prometheus.CounterVec

	mu      sync.RWMutex
	entries map[queryMetricsKey]queryMetricsEntry
	ttl     time.Duration
	now     func() time.Time
}

func newQueryMetricsState(executions, errors, duration *prometheus.CounterVec, ttl time.Duration) *queryMetricsState {
	return &queryMetricsState{
		executions: executions,
		errors:     errors,
		duration:   duration,
		entries:    make(map[queryMetricsKey]queryMetricsEntry),
		ttl:        ttl,
		now:        time.Now,
	}
}

// update folds one poll's cumulative totals into the counters and prunes any
// series that has aged out of the TTL window.
func (s *queryMetricsState) update(sources []queryMetricSource) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()

	for _, source := range sources {
		key := queryMetricsKey{database: source.database, queryHash: source.queryHash}

		previous, exists := s.entries[key]
		if !exists || previous.origin != source.origin {
			// First sighting, or the totals now come from a different origin,
			// for example after a failover changed the replica role, so they
			// aren't comparable with the stored baseline: create all three
			// series at zero so downstream ratios resolve to 0 rather than
			// no-data, keep the values already published on existing series,
			// and store the baseline without emitting the totals as a spike.
			s.executions.WithLabelValues(source.database, source.queryHash).Add(0)
			s.errors.WithLabelValues(source.database, source.queryHash).Add(0)
			s.duration.WithLabelValues(source.database, source.queryHash).Add(0)
			s.entries[key] = queryMetricsEntry{
				rawExecutions:      source.executions,
				rawErrors:          source.errors,
				rawDurationSeconds: source.durationSeconds,
				origin:             source.origin,
				lastSeen:           now,
			}
			continue
		}

		executionsDelta := source.executions - previous.rawExecutions
		errorsDelta := source.errors - previous.rawErrors
		durationDelta := source.durationSeconds - previous.rawDurationSeconds

		// A negative delta in any series means Query Store was cleared or pruned
		// between polls; re-baseline and emit nothing this cycle. Normal interval
		// rollover never triggers this because the query sums full history.
		//
		// NOTE: this is intentionally strict and can potentially "undercount",
		// but we might relax this in the future. The three series are coupled here:
		// a decrease in any one skips the increments for all three; e.g. in a case
		// when an interval that had errors ends while new executions are all successful,
		// gives errorsDelta < 0 but executionsDelta > 0, dropping all three counts for this cycle.
		if executionsDelta >= 0 && errorsDelta >= 0 && durationDelta >= 0 {
			s.executions.WithLabelValues(source.database, source.queryHash).Add(float64(executionsDelta))
			s.errors.WithLabelValues(source.database, source.queryHash).Add(float64(errorsDelta))
			s.duration.WithLabelValues(source.database, source.queryHash).Add(durationDelta)
		}

		s.entries[key] = queryMetricsEntry{
			rawExecutions:      source.executions,
			rawErrors:          source.errors,
			rawDurationSeconds: source.durationSeconds,
			origin:             source.origin,
			lastSeen:           now,
		}
	}

	s.prune(now)
}

// GetQueryHashes returns a snapshot of the query_hashes currently tracked for
// the given database, i.e. those observed within the TTL window.
func (s *queryMetricsState) GetQueryHashes(database string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hashes := make([]string, 0, len(s.entries))
	for key := range s.entries {
		if key.database == database {
			hashes = append(hashes, key.queryHash)
		}
	}
	return hashes
}

// prune deletes state and series for hashes not observed within the TTL window.
// A hash that flaps around the top-N boundary keeps its baseline until
// then, so activity accrued while it was absent is counted on re-entry.
// Callers must hold s.mu.
func (s *queryMetricsState) prune(now time.Time) {
	for key, entry := range s.entries {
		if now.Sub(entry.lastSeen) <= s.ttl {
			continue
		}
		s.executions.DeleteLabelValues(key.database, key.queryHash)
		s.errors.DeleteLabelValues(key.database, key.queryHash)
		s.duration.DeleteLabelValues(key.database, key.queryHash)
		delete(s.entries, key)
	}
}

// reset drops all accumulated state and series. Used when the collector stops.
func (s *queryMetricsState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executions.Reset()
	s.errors.Reset()
	s.duration.Reset()
	clear(s.entries)
}

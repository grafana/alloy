package collector

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func newTestCounterVecs() (executions, errors, duration *prometheus.CounterVec) {
	labels := []string{"database", "query_hash"}
	newVec := func(name string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "database_observability",
			Name:      name,
		}, labels)
	}
	return newVec("query_executions_total"), newVec("query_errors_total"), newVec("query_duration_seconds_total")
}

// counterValue reads the value of the single (testDatabase, testHash) series
// tracked across these tests.
func counterValue(t *testing.T, vec *prometheus.CounterVec) float64 {
	t.Helper()
	metric, err := vec.GetMetricWithLabelValues(testDatabase, testHash)
	require.NoError(t, err)
	var out dto.Metric
	require.NoError(t, metric.Write(&out))
	return out.Counter.GetValue()
}

func TestQueryMetricsState_BaselineAndDeltas(t *testing.T) {
	t.Parallel()

	executions, errCounter, duration := newTestCounterVecs()
	state := newQueryMetricsState(executions, errCounter, duration, time.Hour)

	// First sighting: series created at zero, historical totals not emitted.
	state.update([]queryMetricSource{{
		database: testDatabase, queryHash: testHash, executions: 10, errors: 2, durationSeconds: 5,
	}})
	require.Equal(t, 0.0, counterValue(t, executions))
	require.Equal(t, 0.0, counterValue(t, errCounter))
	require.Equal(t, 0.0, counterValue(t, duration))

	// Second poll: only the increase is added.
	state.update([]queryMetricSource{{
		database: testDatabase, queryHash: testHash, executions: 14, errors: 3, durationSeconds: 8.5,
	}})
	require.Equal(t, 4.0, counterValue(t, executions))
	require.Equal(t, 1.0, counterValue(t, errCounter))
	require.InDelta(t, 3.5, counterValue(t, duration), 1e-9)

	// Third poll: unchanged totals add nothing.
	state.update([]queryMetricSource{{
		database: testDatabase, queryHash: testHash, executions: 14, errors: 3, durationSeconds: 8.5,
	}})
	require.Equal(t, 4.0, counterValue(t, executions))
	require.Equal(t, 1.0, counterValue(t, errCounter))
	require.InDelta(t, 3.5, counterValue(t, duration), 1e-9)
}

func TestQueryMetricsState_NegativeDeltaRebaselines(t *testing.T) {
	t.Parallel()

	executions, errCounter, duration := newTestCounterVecs()
	state := newQueryMetricsState(executions, errCounter, duration, time.Hour)

	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 100, errors: 10, durationSeconds: 50}})
	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 130, errors: 12, durationSeconds: 65}})
	require.Equal(t, 30.0, counterValue(t, executions))
	require.Equal(t, 2.0, counterValue(t, errCounter))
	require.InDelta(t, 15.0, counterValue(t, duration), 1e-9)

	// Query Store was cleared/pruned: totals drop. Nothing is added and the
	// series re-baselines to the new, lower totals.
	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 5, errors: 1, durationSeconds: 2}})
	require.Equal(t, 30.0, counterValue(t, executions))
	require.Equal(t, 2.0, counterValue(t, errCounter))
	require.InDelta(t, 15.0, counterValue(t, duration), 1e-9)

	// Growth after the reset is measured from the new baseline.
	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 9, errors: 3, durationSeconds: 6}})
	require.Equal(t, 34.0, counterValue(t, executions))
	require.Equal(t, 4.0, counterValue(t, errCounter))
	require.InDelta(t, 19.0, counterValue(t, duration), 1e-9)
}

func TestQueryMetricsState_PrunesAfterTTL(t *testing.T) {
	t.Parallel()

	executions, errCounter, duration := newTestCounterVecs()
	state := newQueryMetricsState(executions, errCounter, duration, 5*time.Minute)

	now := time.Now()
	state.now = func() time.Time { return now }

	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 10, errors: 0, durationSeconds: 1}})
	require.Equal(t, 1, testutil.CollectAndCount(executions))

	// Not seen for longer than the TTL: series and state are dropped.
	now = now.Add(6 * time.Minute)
	state.update(nil)
	require.Equal(t, 0, testutil.CollectAndCount(executions))
	require.Equal(t, 0, testutil.CollectAndCount(errCounter))
	require.Equal(t, 0, testutil.CollectAndCount(duration))
	require.Empty(t, state.entries)
}

func TestQueryMetricsState_FlappingHashKeepsBaselineWithinTTL(t *testing.T) {
	t.Parallel()

	executions, errCounter, duration := newTestCounterVecs()
	state := newQueryMetricsState(executions, errCounter, duration, 10*time.Minute)

	now := time.Now()
	state.now = func() time.Time { return now }

	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 10, errors: 0, durationSeconds: 1}})

	// Drops out of the top-N for a couple of cycles, but stays within the TTL.
	now = now.Add(1 * time.Minute)
	state.update(nil)
	now = now.Add(1 * time.Minute)
	state.update(nil)
	require.Equal(t, 1, testutil.CollectAndCount(executions), "series should be retained within the TTL window")

	// Re-enters the top-N: activity accrued while absent is counted from the
	// retained baseline, not lost to a re-baseline.
	now = now.Add(1 * time.Minute)
	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 25, errors: 4, durationSeconds: 9}})
	require.Equal(t, 15.0, counterValue(t, executions))
	require.Equal(t, 4.0, counterValue(t, errCounter))
	require.InDelta(t, 8.0, counterValue(t, duration), 1e-9)
}

func TestQueryMetricsState_Reset(t *testing.T) {
	t.Parallel()

	executions, errCounter, duration := newTestCounterVecs()
	state := newQueryMetricsState(executions, errCounter, duration, time.Hour)

	state.update([]queryMetricSource{{database: testDatabase, queryHash: testHash, executions: 10, errors: 1, durationSeconds: 5}})
	state.reset()

	require.Empty(t, state.entries)
	require.Equal(t, 0, testutil.CollectAndCount(executions))
	require.Equal(t, 0, testutil.CollectAndCount(errCounter))
	require.Equal(t, 0, testutil.CollectAndCount(duration))
}

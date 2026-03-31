package database_observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

const testCheckInterval = 15 * time.Millisecond
const testThreshold = 3

func TestRunConnectionInfoMonitor_UnregistersAfterConsecutiveFailures(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "provider_account", "db_instance_identifier", "engine", "engine_version"})
	require.NoError(t, registry.Register(infoMetric))

	labelValues := []string{"aws", "us-east-1", "123456789", "my-db", "postgres", "15.0"}
	infoMetric.WithLabelValues(labelValues...).Set(1)

	// Expect 3 pings, all failing
	pingErr := errors.New("connection refused")
	for i := 0; i < testThreshold; i++ {
		mock.ExpectPing().WillReturnError(pingErr)
	}

	ctx := context.Background()
	onStopped := func() {}
	config := &ConnectionInfoMonitorConfig{
		CheckInterval:   testCheckInterval,
		ChecksThreshold: testThreshold,
	}
	cancel := RunConnectionInfoMonitor(ctx, db, registry, infoMetric, labelValues, onStopped, config)
	defer cancel()

	// Wait for at least 3 tick intervals so the monitor performs 3 failed pings and unregisters
	time.Sleep(testCheckInterval*time.Duration(testThreshold) + 20*time.Millisecond)

	// Metric should have been unregistered (not present in gather)
	metrics, err := registry.Gather()
	require.NoError(t, err)
	var found bool
	for _, mf := range metrics {
		if mf.GetName() == "database_observability_connection_info" {
			found = true
			break
		}
	}
	require.False(t, found, "metric should be unregistered after %d consecutive ping failures", testThreshold)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunConnectionInfoMonitor_ReregistersAfterConsecutiveSuccesses(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "provider_account", "db_instance_identifier", "engine", "engine_version"})
	require.NoError(t, registry.Register(infoMetric))

	labelValues := []string{"aws", "us-east-1", "123456789", "my-db", "mysql", "8.0.32"}
	infoMetric.WithLabelValues(labelValues...).Set(1)

	// First 3 pings fail (metric gets unregistered), then many succeed (metric re-registers and stays up).
	// Extra success expectations prevent sqlmock from returning errors for pings that occur while
	// require.Eventually polls, which would re-trigger the failure threshold and unregister again.
	pingErr := errors.New("connection refused")
	for i := 0; i < testThreshold; i++ {
		mock.ExpectPing().WillReturnError(pingErr)
	}
	for i := 0; i < 30; i++ {
		mock.ExpectPing()
	}

	ctx := context.Background()
	onStopped := func() {}
	config := &ConnectionInfoMonitorConfig{
		CheckInterval:   testCheckInterval,
		ChecksThreshold: testThreshold,
	}
	cancel := RunConnectionInfoMonitor(ctx, db, registry, infoMetric, labelValues, onStopped, config)
	defer cancel()

	// Poll until the metric is re-registered rather than sleeping a fixed duration, which is
	// unreliable: extra pings after the mock expectations are exhausted return errors, causing
	// the failure threshold to be hit again and the metric to be unregistered before we check.
	var mf *dto.MetricFamily
	require.Eventually(t, func() bool {
		metrics, err := registry.Gather()
		if err != nil {
			return false
		}
		for _, m := range metrics {
			if m.GetName() == "database_observability_connection_info" {
				mf = m
				return true
			}
		}
		return false
	}, 2*time.Second, testCheckInterval, "metric should be re-registered after %d consecutive successes", testThreshold)

	require.Len(t, mf.Metric, 1, "metric should have one series when present")
	require.Equal(t, float64(1), mf.Metric[0].GetGauge().GetValue())
}

func TestRunConnectionInfoMonitor_MetricRemainsRegisteredWhilePingsSucceed(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "provider_account", "db_instance_identifier", "engine", "engine_version"})
	require.NoError(t, registry.Register(infoMetric))

	labelValues := []string{"unknown", "unknown", "unknown", "unknown", "postgres", "15.0"}
	infoMetric.WithLabelValues(labelValues...).Set(1)

	// All pings succeed (allow at least 4 successful pings)
	for i := 0; i < 4; i++ {
		mock.ExpectPing()
	}

	ctx := context.Background()
	onStopped := func() {}
	config := &ConnectionInfoMonitorConfig{
		CheckInterval:   testCheckInterval,
		ChecksThreshold: testThreshold,
	}
	cancel := RunConnectionInfoMonitor(ctx, db, registry, infoMetric, labelValues, onStopped, config)
	defer cancel()

	// Wait for a few tick intervals
	time.Sleep(testCheckInterval*4 + 20*time.Millisecond)

	// Metric should still be registered with value 1
	metrics, err := registry.Gather()
	require.NoError(t, err)
	var mf *dto.MetricFamily
	for _, m := range metrics {
		if m.GetName() == "database_observability_connection_info" {
			mf = m
			break
		}
	}
	require.NotNil(t, mf, "metric should remain registered while pings succeed")
	require.Len(t, mf.Metric, 1)
	require.Equal(t, float64(1), mf.Metric[0].GetGauge().GetValue())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunConnectionInfoMonitor_CancelStopsGoroutine(t *testing.T) {
	defer goleak.VerifyNone(t)

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	registry := prometheus.NewRegistry()
	infoMetric := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "database_observability",
		Name:      "connection_info",
		Help:      "Information about the connection",
	}, []string{"provider_name", "provider_region", "provider_account", "db_instance_identifier", "engine", "engine_version"})
	require.NoError(t, registry.Register(infoMetric))

	labelValues := []string{"a", "b", "c", "d", "e", "f"}
	infoMetric.WithLabelValues(labelValues...).Set(1)

	mock.ExpectPing() // at most one ping before we cancel

	ctx := context.Background()
	stopped := make(chan struct{})
	onStopped := func() { close(stopped) }
	config := &ConnectionInfoMonitorConfig{
		CheckInterval:   testCheckInterval,
		ChecksThreshold: testThreshold,
	}
	cancel := RunConnectionInfoMonitor(ctx, db, registry, infoMetric, labelValues, onStopped, config)

	// Cancel immediately; onStopped should be called when the goroutine exits
	cancel()
	select {
	case <-stopped:
		// goroutine exited
	case <-time.After(2 * time.Second):
		t.Fatal("onStopped was not called after cancel")
	}
}

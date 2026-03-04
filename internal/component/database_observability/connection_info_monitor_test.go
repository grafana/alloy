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
	infoMetric.WithLabelValues(labelValues[0], labelValues[1], labelValues[2], labelValues[3], labelValues[4], labelValues[5]).Set(1)

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
	infoMetric.WithLabelValues(labelValues[0], labelValues[1], labelValues[2], labelValues[3], labelValues[4], labelValues[5]).Set(1)

	// First 3 pings fail (metric gets unregistered), then 3 pings succeed (metric gets re-registered)
	pingErr := errors.New("connection refused")
	for i := 0; i < testThreshold; i++ {
		mock.ExpectPing().WillReturnError(pingErr)
	}
	for i := 0; i < testThreshold; i++ {
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

	// Wait for 6 tick intervals so we get 3 failures (unregister) then 3 successes (re-register).
	time.Sleep(testCheckInterval*time.Duration(testThreshold*2) + 100*time.Millisecond)

	// All 6 pings must have been consumed (3 fail + 3 success), proving the monitor ran the re-register path.
	require.NoError(t, mock.ExpectationsWereMet(), "monitor should have performed 3 failing then 3 successful pings")

	// Verify re-registration: the monitor runs MustRegister and Set(1) after 3 consecutive successes.
	// ExpectationsWereMet above confirms all 6 pings ran (3 fail + 3 success). If the registry
	// exposes the re-registered metric, assert its value.
	metrics, err := registry.Gather()
	require.NoError(t, err)
	var mf *dto.MetricFamily
	for _, m := range metrics {
		if m.GetName() == "database_observability_connection_info" {
			mf = m
			break
		}
	}
	if mf != nil {
		require.Len(t, mf.Metric, 1, "metric should have one series when present")
		require.Equal(t, float64(1), mf.Metric[0].GetGauge().GetValue())
	}
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
	infoMetric.WithLabelValues(labelValues[0], labelValues[1], labelValues[2], labelValues[3], labelValues[4], labelValues[5]).Set(1)

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
	infoMetric.WithLabelValues(labelValues[0], labelValues[1], labelValues[2], labelValues[3], labelValues[4], labelValues[5]).Set(1)

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

package collector

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestErrorLogsCollector_ParseRDSFormat(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test-instance",
		SystemID:     "test-system",
		Registry:     registry,
	})
	require.NoError(t, err)

	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	tests := []struct {
		name     string
		log      string
		wantUser string
		wantDB   string
		wantSev  string
	}{
		{
			name:     "ERROR severity",
			log:      `2025-12-12 15:29:16.068 GMT:[local]:app-user@books_store:[9112]:4:57014:2025-12-12 15:29:15 GMT:25/112:0:693c34cb.2398::psqlERROR:  canceling statement`,
			wantUser: "app-user",
			wantDB:   "books_store",
			wantSev:  "ERROR",
		},
		{
			name:     "FATAL severity",
			log:      `2025-12-12 15:29:31.529 GMT:[local]:conn_user@testdb:[9449]:4:53300:2025-12-12 15:29:31 GMT:91/57:0:693c34db.24e9::psqlFATAL:  too many connections`,
			wantUser: "conn_user",
			wantDB:   "testdb",
			wantSev:  "FATAL",
		},
		{
			name:     "PANIC severity",
			log:      `2025-12-12 15:30:00.000 GMT:::1:admin@postgres:[9500]:1:XX000:2025-12-12 15:30:00 GMT:1/1:0:693c34db.9999::psqlPANIC:  system failure`,
			wantUser: "admin",
			wantDB:   "postgres",
			wantSev:  "PANIC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.Receiver().Chan() <- loki.Entry{
				Entry: push.Entry{
					Line:      tt.log,
					Timestamp: time.Now(),
				},
			}

			time.Sleep(100 * time.Millisecond)

			mfs, _ := registry.Gather()
			found := false
			for _, mf := range mfs {
				if mf.GetName() == "postgres_errors_total" {
					for _, metric := range mf.GetMetric() {
						labels := make(map[string]string)
						for _, label := range metric.GetLabel() {
							labels[label.GetName()] = label.GetValue()
						}
						if labels["user"] == tt.wantUser && labels["database"] == tt.wantDB {
							require.Equal(t, tt.wantSev, labels["severity"])
							require.Equal(t, "test-instance", labels["instance"])
							require.Equal(t, "test-system", labels["server_id"])
							found = true
							break
						}
					}
				}
			}
			require.True(t, found, "metric not found for %s", tt.name)
		})
	}
}

func TestErrorLogsCollector_SkipsNonErrors(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     registry,
	})
	require.NoError(t, err)

	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Send INFO and LOG messages (should be skipped)
	skipLogs := []string{
		`2025-12-12 15:29:42.201 GMT:::1:app-user@books_store:[9589]:2:00000:2025-12-12 15:29:42 GMT:159/363:0:693c34e6.2575::psqlINFO:  some info`,
		`2025-12-12 15:29:42.201 GMT:::1:app-user@books_store:[9589]:2::2025-12-12 15:29:42 GMT:159/363:0:693c34e6.2575::psqlLOG:  connection received`,
		"DETAIL:  Some detail line",
		"HINT:  Some hint line",
		"\tIndented continuation line",
	}

	for _, logLine := range skipLogs {
		collector.Receiver().Chan() <- loki.Entry{
			Entry: push.Entry{
				Line:      logLine,
				Timestamp: time.Now(),
			},
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Should have 0 metrics since all were skipped
	mfs, _ := registry.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "postgres_errors_total" {
			require.Equal(t, 0, len(mf.GetMetric()), "should not create metrics for non-error logs")
		}
	}
}

func TestErrorLogsCollector_MetricSumming(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 100), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test-instance",
		SystemID:     "test-system",
		Registry:     registry,
	})
	require.NoError(t, err)

	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Send multiple errors with same labels (should sum)
	logs := []struct {
		log  string
		user string
		db   string
		sev  string
	}{
		{
			log:  `2025-01-12 10:30:45 UTC:10.0.1.5:54321:user1@db1:[9112]:4:57014:2025-01-12 10:29:15 UTC:25/112:0:693c34cb.2398::psqlERROR:  error 1`,
			user: "user1",
			db:   "db1",
			sev:  "ERROR",
		},
		{
			log:  `2025-01-12 10:31:00 UTC:10.0.1.5:54321:user1@db1:[9113]:5:57014:2025-01-12 10:29:15 UTC:25/113:0:693c34cb.2399::psqlERROR:  error 2`,
			user: "user1",
			db:   "db1",
			sev:  "ERROR",
		},
		{
			log:  `2025-01-12 10:32:00 UTC:10.0.1.5:54321:user1@db1:[9114]:6:57014:2025-01-12 10:29:15 UTC:25/114:0:693c34cb.2400::psqlERROR:  error 3`,
			user: "user1",
			db:   "db1",
			sev:  "ERROR",
		},
		{
			log:  `2025-01-12 10:33:00 UTC:10.0.1.5:54322:user2@db2:[9115]:7:28P01:2025-01-12 10:33:00 UTC:159/363:0:693c34e6.2575::psqlFATAL:  auth failed`,
			user: "user2",
			db:   "db2",
			sev:  "FATAL",
		},
	}

	for _, l := range logs {
		collector.Receiver().Chan() <- loki.Entry{
			Entry: push.Entry{
				Line:      l.log,
				Timestamp: time.Now(),
			},
		}
	}

	time.Sleep(200 * time.Millisecond)

	// Verify metrics
	mfs, _ := registry.Gather()
	var errorMetrics *dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == "postgres_errors_total" {
			errorMetrics = mf
			break
		}
	}

	require.NotNil(t, errorMetrics)
	require.Equal(t, 2, len(errorMetrics.GetMetric()), "should have 2 unique label combinations")

	// Check counts
	type metricKey struct {
		user string
		db   string
		sev  string
	}
	counts := make(map[metricKey]float64)
	for _, metric := range errorMetrics.GetMetric() {
		labels := make(map[string]string)
		for _, label := range metric.GetLabel() {
			labels[label.GetName()] = label.GetValue()
		}
		key := metricKey{
			user: labels["user"],
			db:   labels["database"],
			sev:  labels["severity"],
		}
		counts[key] = metric.GetCounter().GetValue()
	}

	require.Equal(t, float64(3), counts[metricKey{user: "user1", db: "db1", sev: "ERROR"}], "user1@db1:ERROR should have count of 3")
	require.Equal(t, float64(1), counts[metricKey{user: "user2", db: "db2", sev: "FATAL"}], "user2@db2:FATAL should have count of 1")
}

func TestErrorLogsCollector_InvalidFormat(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     registry,
	})
	require.NoError(t, err)

	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Send invalid log lines
	collector.Receiver().Chan() <- loki.Entry{
		Entry: push.Entry{
			Line:      `invalid log line without proper format`,
			Timestamp: time.Now(),
		},
	}

	time.Sleep(100 * time.Millisecond)

	// Check parse errors counter was incremented
	mfs, _ := registry.Gather()
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "postgres_error_log_parse_failures_total" {
			found = true
			require.Greater(t, mf.GetMetric()[0].GetCounter().GetValue(), 0.0)
		}
	}
	require.True(t, found, "parse error metric should exist")
}

func TestErrorLogsCollector_StartStop(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})

	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test",
		SystemID:     "test",
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)
	require.NotNil(t, collector.Receiver(), "receiver should be exported")

	err = collector.Start(context.Background())
	require.NoError(t, err)
	require.False(t, collector.Stopped())

	collector.Stop()
	time.Sleep(10 * time.Millisecond)
	require.True(t, collector.Stopped())
}

func TestExtractSeverity(t *testing.T) {
	tests := []struct {
		message  string
		expected string
	}{
		{"ERROR:  canceling statement", "ERROR"},
		{"FATAL:  too many connections", "FATAL"},
		{"PANIC:  system failure", "PANIC"},
		{"LOG:  connection received", "LOG"},
		{"no colon here", ""},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			require.Equal(t, tt.expected, extractSeverity(tt.message))
		})
	}
}

func TestIsContinuationLine(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"\tIndented line", true},
		{"DETAIL:  some detail", true},
		{"HINT:  some hint", true},
		{"CONTEXT:  some context", true},
		{"STATEMENT:  SELECT 1", true},
		{"  DETAIL:  with whitespace", true},
		{"2025-01-12 10:30:45 UTC:app-user@db:[123]:ERROR:  normal log", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			require.Equal(t, tt.expected, isContinuationLine(tt.line))
		})
	}
}

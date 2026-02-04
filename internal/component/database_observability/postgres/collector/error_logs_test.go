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
		name          string
		log           string
		wantUser      string
		wantDB        string
		wantSev       string
		wantSQLState  string
	}{
		{
			name:         "ERROR severity",
			log:          `2025-12-12 15:29:16.068 GMT:[local]:app-user@books_store:[9112]:4:57014:2025-12-12 15:29:15 GMT:25/112:0:693c34cb.2398::psqlERROR:  canceling statement`,
			wantUser:     "app-user",
			wantDB:       "books_store",
			wantSev:      "ERROR",
			wantSQLState: "57014",
		},
		{
			name:         "FATAL severity",
			log:          `2025-12-12 15:29:31.529 GMT:[local]:conn_user@testdb:[9449]:4:53300:2025-12-12 15:29:31 GMT:91/57:0:693c34db.24e9::psqlFATAL:  too many connections`,
			wantUser:     "conn_user",
			wantDB:       "testdb",
			wantSev:      "FATAL",
			wantSQLState: "53300",
		},
		{
			name:         "PANIC severity",
			log:          `2025-12-12 15:30:00.000 GMT:10.0.1.10(5432):admin@postgres:[9500]:1:XX000:2025-12-12 15:30:00 GMT:1/1:0:693c34db.9999::psqlPANIC:  system failure`,
			wantUser:     "admin",
			wantDB:       "postgres",
			wantSev:      "PANIC",
			wantSQLState: "XX000",
		},
		{
			name:         "UTC timezone",
			log:          `2025-12-12 15:29:16.068 UTC:10.0.1.5(12345):app-user@books_store:[9112]:4:40001:2025-12-12 15:29:15 UTC:25/112:0:693c34cb.2398::psqlERROR:  could not serialize access`,
			wantUser:     "app-user",
			wantDB:       "books_store",
			wantSev:      "ERROR",
			wantSQLState: "40001",
		},
		{
			name:         "EST timezone",
			log:          `2025-12-12 15:29:16.068 EST:10.0.1.5(12345):app-user@books_store:[9112]:4:40001:2025-12-12 15:29:15 EST:25/112:0:693c34cb.2398::psqlERROR:  could not serialize access`,
			wantUser:     "app-user",
			wantDB:       "books_store",
			wantSev:      "ERROR",
			wantSQLState: "40001",
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
							require.Equal(t, tt.wantSQLState, labels["sqlstate"])
							require.Equal(t, tt.wantSQLState[:2], labels["sqlstate_class"])
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

	// Send invalid log line (has ERROR: but wrong format - missing required fields)
	collector.Receiver().Chan() <- loki.Entry{
		Entry: push.Entry{
			Line:      `ERROR: this line has ERROR but invalid RDS format`,
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

func TestErrorLogsCollector_SQLStateExtraction(t *testing.T) {
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
		name              string
		log               string
		wantSQLState      string
		wantSQLStateClass string
		wantSeverity      string
	}{
		{
			name:              "Serialization failure (40001)",
			log:               `2026-01-25 20:00:00.702 UTC:10.24.193.106(33090):mybooks-app@books_store:[25599]:1:40001:2026-01-25 19:58:36 UTC:172/48089:85097235:697675ec.63ff:[unknown]:ERROR:  could not serialize access due to concurrent update`,
			wantSQLState:      "40001",
			wantSQLStateClass: "40",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Deadlock detected (40P01)",
			log:               `2026-01-25 20:01:30 UTC:10.32.115.73(34710):mybooks-app-2@books_store_2:[2170]:1:40P01:2026-01-25 20:00:00 UTC:100/200:85097240:69767600.1000:[unknown]:ERROR:  deadlock detected`,
			wantSQLState:      "40P01",
			wantSQLStateClass: "40",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Unique violation (23505)",
			log:               `2026-01-25 20:02:00 UTC:10.24.193.106(44148):app-user@testdb:[25296]:2:23505:2026-01-25 20:00:00 UTC:121/51119:85097236:6976755e.62d0:[unknown]:ERROR:  duplicate key value violates unique constraint`,
			wantSQLState:      "23505",
			wantSQLStateClass: "23",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Query canceled (57014)",
			log:               `2025-12-12 15:29:16.068 GMT:[local]:app-user@books_store:[9112]:4:57014:2025-12-12 15:29:15 GMT:25/112:0:693c34cb.2398::psqlERROR:  canceling statement`,
			wantSQLState:      "57014",
			wantSQLStateClass: "57",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Too many connections (53300)",
			log:               `2025-12-12 15:29:31.529 GMT:[local]:conn_user@testdb:[9449]:4:53300:2025-12-12 15:29:31 GMT:91/57:0:693c34db.24e9::psqlFATAL:  too many connections`,
			wantSQLState:      "53300",
			wantSQLStateClass: "53",
			wantSeverity:      "FATAL",
		},
		{
			name:              "Auth failed (28P01)",
			log:               `2025-12-12 10:33:00 UTC:10.0.1.5:54322:user2@db2:[9115]:7:28P01:2025-12-12 10:33:00 UTC:159/363:0:693c34e6.2575::psqlFATAL:  password authentication failed`,
			wantSQLState:      "28P01",
			wantSQLStateClass: "28",
			wantSeverity:      "FATAL",
		},
		{
			name:              "Internal error (XX000)",
			log:               `2025-12-12 15:30:00.000 GMT:10.0.1.10(5432):admin@postgres:[9500]:1:XX000:2025-12-12 15:30:00 GMT:1/1:0:693c34db.9999::psqlPANIC:  unexpected internal error`,
			wantSQLState:      "XX000",
			wantSQLStateClass: "XX",
			wantSeverity:      "PANIC",
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
						if labels["sqlstate"] == tt.wantSQLState {
							require.Equal(t, tt.wantSQLStateClass, labels["sqlstate_class"], "sqlstate_class should match")
							require.Equal(t, tt.wantSeverity, labels["severity"], "severity should match")
							require.Equal(t, "test-instance", labels["instance"])
							require.Equal(t, "test-system", labels["server_id"])
							found = true
							break
						}
					}
				}
			}
			require.True(t, found, "metric with sqlstate=%s not found for %s", tt.wantSQLState, tt.name)
		})
	}
}

func TestErrorLogsCollector_UpdateSystemID(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	// Create collector with initial SystemID
	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		InstanceKey:  "test-instance",
		SystemID:     "initial-system-id",
		Registry:     registry,
	})
	require.NoError(t, err)

	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Process a log entry with initial SystemID (using GMT timezone)
	logLine := `2025-12-12 15:29:16.068 GMT:10.0.1.5(12345):app-user@books_store:[9112]:4:40001:2025-12-12 15:29:15 GMT:25/112:0:693c34cb.2398::psqlERROR:  could not serialize access`
	
	entry := loki.Entry{
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      logLine,
		},
	}
	
	collector.Receiver().Chan() <- entry
	time.Sleep(100 * time.Millisecond)

	// Verify metric has initial SystemID
	mfs, _ := registry.Gather()
	found := false
	for _, mf := range mfs {
		if mf.GetName() == "postgres_errors_total" {
			for _, metric := range mf.GetMetric() {
				labels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				if labels["sqlstate"] == "40001" {
					require.Equal(t, "initial-system-id", labels["server_id"], "should have initial system ID")
					found = true
					break
				}
			}
		}
	}
	require.True(t, found, "metric with initial system ID not found")

	// Update SystemID
	collector.UpdateSystemID("new-system-id")

	// Process another log entry
	collector.Receiver().Chan() <- entry
	time.Sleep(100 * time.Millisecond)

	// Verify metric now has new SystemID
	mfs, _ = registry.Gather()
	foundNew := false
	for _, mf := range mfs {
		if mf.GetName() == "postgres_errors_total" {
			for _, metric := range mf.GetMetric() {
				labels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				if labels["sqlstate"] == "40001" && labels["server_id"] == "new-system-id" {
					foundNew = true
					break
				}
			}
		}
	}
	require.True(t, foundNew, "metric with new system ID not found")

	// Test concurrent updates (thread safety)
	t.Run("concurrent_updates", func(t *testing.T) {
		done := make(chan bool, 10)
		
		// Launch 10 goroutines updating SystemID
		for i := 0; i < 10; i++ {
			go func(id int) {
				for j := 0; j < 100; j++ {
					collector.UpdateSystemID("concurrent-id-" + string(rune('0'+id)))
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// If we get here without panic/deadlock, thread safety is working
		t.Log("Concurrent updates completed successfully")
	})
}

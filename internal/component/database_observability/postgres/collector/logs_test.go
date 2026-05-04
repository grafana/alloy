package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
)

func TestLogsCollector_ParseRDSFormat(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Build log lines with timestamps after collector start (like SkipsHistoricalLogs)
	ts := startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	tests := []struct {
		name         string
		log          string
		wantUser     string
		wantDB       string
		wantSev      string
		wantSQLState string
	}{
		{
			name:         "ERROR severity",
			log:          ts1 + ":[local]:app-user@books_store:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement",
			wantUser:     "app-user",
			wantDB:       "books_store",
			wantSev:      "ERROR",
			wantSQLState: "57014",
		},
		{
			name:         "FATAL severity",
			log:          ts1 + ":[local]:conn_user@testdb:[9449]:4:53300:" + ts2 + ":91/57:0:693c34db.24e9::psqlFATAL:  too many connections",
			wantUser:     "conn_user",
			wantDB:       "testdb",
			wantSev:      "FATAL",
			wantSQLState: "53300",
		},
		{
			name:         "PANIC severity",
			log:          ts1 + ":10.0.1.10(5432):admin@postgres:[9500]:1:XX000:" + ts2 + ":1/1:0:693c34db.9999::psqlPANIC:  system failure",
			wantUser:     "admin",
			wantDB:       "postgres",
			wantSev:      "PANIC",
			wantSQLState: "XX000",
		},
		{
			name:         "UTC timezone",
			log:          ts1 + ":10.0.1.5(12345):app-user@books_store:[9112]:4:40001:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  could not serialize access",
			wantUser:     "app-user",
			wantDB:       "books_store",
			wantSev:      "ERROR",
			wantSQLState: "40001",
		},
		{
			name:         "EST timezone",
			log:          strings.ReplaceAll(ts1, " UTC", " EST") + ":10.0.1.5(12345):app-user@books_store:[9112]:4:40001:" + strings.ReplaceAll(ts2, " UTC", " EST") + ":25/112:0:693c34cb.2398::psqlERROR:  could not serialize access",
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
				if mf.GetName() == "database_observability_pg_errors_total" {
					for _, metric := range mf.GetMetric() {
						labels := make(map[string]string)
						for _, label := range metric.GetLabel() {
							labels[label.GetName()] = label.GetValue()
						}
						if labels["user"] == tt.wantUser && labels["datname"] == tt.wantDB && labels["severity"] == tt.wantSev && labels["sqlstate"] == tt.wantSQLState {
							require.Equal(t, tt.wantSev, labels["severity"])
							require.Equal(t, tt.wantSQLState, labels["sqlstate"])
							require.Equal(t, tt.wantSQLState[:2], labels["sqlstate_class"])
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

func TestLogsCollector_SkipsNonErrors(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Build INFO and LOG lines with timestamps AFTER collector start, so they would pass the
	// historical filter if they reached it. They are skipped for severity (not ERROR/FATAL/PANIC).
	ts := startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	skipLogs := []string{
		ts1 + ":::1:app-user@books_store:[9589]:2:00000:" + ts2 + ":159/363:0:693c34e6.2575::psqlINFO:  some info",
		ts1 + ":::1:app-user@books_store:[9589]:2:00000:" + ts2 + ":159/363:0:693c34e6.2575::psqlLOG:  connection received",
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
		if mf.GetName() == "database_observability_pg_errors_total" {
			require.Equal(t, 0, len(mf.GetMetric()), "should not create metrics for non-error logs")
		}
	}
}

func TestLogsCollector_MetricSumming(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 100), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	ts := startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	// Send multiple errors with same labels (should sum)
	logs := []struct {
		log  string
		user string
		db   string
		sev  string
	}{
		{
			log:  ts1 + ":10.0.1.5:54321:user1@db1:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  error 1",
			user: "user1",
			db:   "db1",
			sev:  "ERROR",
		},
		{
			log:  ts1 + ":10.0.1.5:54321:user1@db1:[9113]:5:57014:" + ts2 + ":25/113:0:693c34cb.2399::psqlERROR:  error 2",
			user: "user1",
			db:   "db1",
			sev:  "ERROR",
		},
		{
			log:  ts1 + ":10.0.1.5:54321:user1@db1:[9114]:6:57014:" + ts2 + ":25/114:0:693c34cb.2400::psqlERROR:  error 3",
			user: "user1",
			db:   "db1",
			sev:  "ERROR",
		},
		{
			log:  ts1 + ":10.0.1.5:54322:user2@db2:[9115]:7:28P01:" + ts2 + ":159/363:0:693c34e6.2575::psqlFATAL:  auth failed",
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
		if mf.GetName() == "database_observability_pg_errors_total" {
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
			db:   labels["datname"],
			sev:  labels["severity"],
		}
		counts[key] = metric.GetCounter().GetValue()
	}

	require.Equal(t, float64(3), counts[metricKey{user: "user1", db: "db1", sev: "ERROR"}], "user1@db1:ERROR should have count of 3")
	require.Equal(t, float64(1), counts[metricKey{user: "user2", db: "db2", sev: "FATAL"}], "user2@db2:FATAL should have count of 1")
}

func TestLogsCollector_InvalidFormat(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
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
		if mf.GetName() == "database_observability_pg_error_log_parse_failures_total" {
			found = true
			require.Greater(t, mf.GetMetric()[0].GetCounter().GetValue(), 0.0)
		}
	}
	require.True(t, found, "parse error metric should exist")
}

func TestLogsCollector_EmptyUserAndDatabase(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Build log with timestamps after collector start (empty user/database = background worker)
	ts := startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")
	logLine := fmt.Sprintf("%s::@:[26350]:1:57P01:%s:828/162213:0:6982f7c4.66ee:FATAL:  terminating background worker \"parallel worker\" due to administrator command", ts1, ts2)

	collector.Receiver().Chan() <- loki.Entry{
		Entry: push.Entry{
			Line:      logLine,
			Timestamp: time.Now(),
		},
	}

	time.Sleep(200 * time.Millisecond)

	// Verify metric was created with empty user and database labels
	mfs, _ := registry.Gather()
	var errorMetrics *dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_errors_total" {
			errorMetrics = mf
			break
		}
	}

	require.NotNil(t, errorMetrics)
	require.Equal(t, 1, len(errorMetrics.GetMetric()), "should have 1 metric entry")

	metric := errorMetrics.GetMetric()[0]
	labels := make(map[string]string)
	for _, lp := range metric.GetLabel() {
		labels[lp.GetName()] = lp.GetValue()
	}

	require.Equal(t, "", labels["datname"], "database should be empty")
	require.Equal(t, "", labels["user"], "user should be empty")
	require.Equal(t, "FATAL", labels["severity"])
	require.Equal(t, "57P01", labels["sqlstate"])
	require.Equal(t, "57", labels["sqlstate_class"])
	require.Equal(t, 1.0, metric.GetCounter().GetValue())

	// Verify no parse errors
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_error_log_parse_failures_total" {
			require.Equal(t, 0.0, mf.GetMetric()[0].GetCounter().GetValue(), "should have no parse errors")
		}
	}
}

func TestLogsCollector_StartStop(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)
	require.NotNil(t, collector.Receiver(), "receiver should be exported")

	err = collector.Start(context.Background())
	require.NoError(t, err)
	require.False(t, collector.Stopped())

	collector.Stop()
	require.Eventually(t, func() bool {
		return collector.Stopped()
	}, 5*time.Second, 100*time.Millisecond)
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

func TestLogsCollector_SQLStateExtraction(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	ts := startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	tests := []struct {
		name              string
		log               string
		wantSQLState      string
		wantSQLStateClass string
		wantSeverity      string
	}{
		{
			name:              "Serialization failure (40001)",
			log:               ts1 + ":10.24.193.106(33090):mybooks-app@books_store:[25599]:1:40001:" + ts2 + ":172/48089:85097235:697675ec.63ff:[unknown]:ERROR:  could not serialize access due to concurrent update",
			wantSQLState:      "40001",
			wantSQLStateClass: "40",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Deadlock detected (40P01)",
			log:               ts1 + ":10.32.115.73(34710):mybooks-app-2@books_store_2:[2170]:1:40P01:" + ts2 + ":100/200:85097240:69767600.1000:[unknown]:ERROR:  deadlock detected",
			wantSQLState:      "40P01",
			wantSQLStateClass: "40",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Unique violation (23505)",
			log:               ts1 + ":10.24.193.106(44148):app-user@testdb:[25296]:2:23505:" + ts2 + ":121/51119:85097236:6976755e.62d0:[unknown]:ERROR:  duplicate key value violates unique constraint",
			wantSQLState:      "23505",
			wantSQLStateClass: "23",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Query canceled (57014)",
			log:               ts1 + ":[local]:app-user@books_store:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement",
			wantSQLState:      "57014",
			wantSQLStateClass: "57",
			wantSeverity:      "ERROR",
		},
		{
			name:              "Too many connections (53300)",
			log:               ts1 + ":[local]:conn_user@testdb:[9449]:4:53300:" + ts2 + ":91/57:0:693c34db.24e9::psqlFATAL:  too many connections",
			wantSQLState:      "53300",
			wantSQLStateClass: "53",
			wantSeverity:      "FATAL",
		},
		{
			name:              "Auth failed (28P01)",
			log:               ts1 + ":10.0.1.5:54322:user2@db2:[9115]:7:28P01:" + ts2 + ":159/363:0:693c34e6.2575::psqlFATAL:  password authentication failed",
			wantSQLState:      "28P01",
			wantSQLStateClass: "28",
			wantSeverity:      "FATAL",
		},
		{
			name:              "Internal error (XX000)",
			log:               ts1 + ":10.0.1.10(5432):admin@postgres:[9500]:1:XX000:" + ts2 + ":1/1:0:693c34db.9999::psqlPANIC:  unexpected internal error",
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
				if mf.GetName() == "database_observability_pg_errors_total" {
					for _, metric := range mf.GetMetric() {
						labels := make(map[string]string)
						for _, label := range metric.GetLabel() {
							labels[label.GetName()] = label.GetValue()
						}
						if labels["sqlstate"] == tt.wantSQLState {
							require.Equal(t, tt.wantSQLStateClass, labels["sqlstate_class"], "sqlstate_class should match")
							require.Equal(t, tt.wantSeverity, labels["severity"], "severity should match")
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

func TestLogsCollector_SkipsHistoricalLogs(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime

	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Create timestamps relative to start time
	historicalTime := startTime.Add(-1 * time.Hour)
	recentTime := startTime.Add(10 * time.Second)

	// Send historical log (1 hour before start) with timestamp in log line
	historicalLine := fmt.Sprintf("%s:[local]:user@database:[1234]:1:28000:%s:1/1:0:000000.0::psqlERROR:  test historical error",
		historicalTime.Format("2006-01-02 15:04:05.000 MST"),
		historicalTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Historical line: %s", historicalLine)

	historicalEntry := loki.Entry{
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      historicalLine,
		},
	}

	// Send recent log (after start) with timestamp in log line
	recentLine := fmt.Sprintf("%s:[local]:user@database:[1234]:1:28000:%s:1/1:0:000000.0::psqlERROR:  test recent error",
		recentTime.Format("2006-01-02 15:04:05.000 MST"),
		recentTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Recent line: %s", recentLine)

	recentEntry := loki.Entry{
		Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      recentLine,
		},
	}

	// Process both
	collector.Receiver().Chan() <- historicalEntry
	collector.Receiver().Chan() <- recentEntry
	time.Sleep(200 * time.Millisecond)

	// Verify only recent log incremented counter
	mfs, err := registry.Gather()
	require.NoError(t, err)

	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_errors_total" {
			for _, metric := range mf.GetMetric() {
				totalCount += metric.GetCounter().GetValue()
			}
		}
	}

	t.Logf("Total count: %f", totalCount)
	require.Equal(t, float64(1), totalCount, "only recent log should be counted")
}

func TestLogsCollector_SkipsOnlyHistoricalLogs(t *testing.T) {
	// Explicitly validates that logs with timestamps before collector start produce 0 metrics
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	// Send ONLY historical logs (valid ERROR format, but timestamp before collector start)
	historicalTime := startTime.Add(-1 * time.Hour)
	historicalLine := fmt.Sprintf("%s:[local]:user@database:[1234]:1:28000:%s:1/1:0:000000.0::psqlERROR:  test historical error",
		historicalTime.Format("2006-01-02 15:04:05.000 MST"),
		historicalTime.Format("2006-01-02 15:04:05 MST"))

	collector.Receiver().Chan() <- loki.Entry{
		Entry: push.Entry{
			Line:      historicalLine,
			Timestamp: time.Now(),
		},
	}
	time.Sleep(200 * time.Millisecond)

	// Verify 0 metrics - historical logs must be skipped
	mfs, err := registry.Gather()
	require.NoError(t, err)

	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_errors_total" {
			for _, metric := range mf.GetMetric() {
				totalCount += metric.GetCounter().GetValue()
			}
		}
	}
	require.Equal(t, float64(0), totalCount, "historical logs must not produce metrics")
}

func TestLogsCollector_ExcludeDatabases(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:         loki.NewLogsReceiver(),
		EntryHandler:     entryHandler,
		Logger:           log.NewNopLogger(),
		Registry:         registry,
		ExcludeDatabases: []string{"excluded_db"},
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	ts := startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	excludedLog := ts1 + ":10.0.1.5(12345):app-user@excluded_db:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"
	allowedLog := ts1 + ":10.0.1.5(12345):app-user@allowed_db:[9113]:5:57014:" + ts2 + ":25/113:0:693c34cb.2399::psqlERROR:  canceling statement"

	collector.Receiver().Chan() <- loki.Entry{Entry: push.Entry{Line: excludedLog, Timestamp: time.Now()}}
	collector.Receiver().Chan() <- loki.Entry{Entry: push.Entry{Line: allowedLog, Timestamp: time.Now()}}

	time.Sleep(200 * time.Millisecond)

	mfs, _ := registry.Gather()
	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_errors_total" {
			for _, metric := range mf.GetMetric() {
				labels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				totalCount += metric.GetCounter().GetValue()
				require.Equal(t, "allowed_db", labels["datname"], "only allowed_db should produce metrics")
			}
		}
	}
	require.Equal(t, float64(1), totalCount, "only the non-excluded database log should be counted")
}

func TestLogsCollector_AttachesQueryFingerprintToError(t *testing.T) {
	cases := []struct {
		name                  string
		disableQueryRedaction bool
	}{
		{name: "redaction_on_default", disableQueryRedaction: false},
		{name: "redaction_off", disableQueryRedaction: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entryHandler := loki.NewCollectingHandler()
			defer entryHandler.Stop()
			registry := prometheus.NewRegistry()

			receiver := loki.NewLogsReceiver()
			c, err := NewLogs(LogsArguments{
				Receiver:              receiver,
				EntryHandler:          entryHandler,
				Logger:                log.NewNopLogger(),
				Registry:              registry,
				DisableQueryRedaction: tc.disableQueryRedaction,
			})
			require.NoError(t, err)
			require.NoError(t, c.Start(context.Background()))
			t.Cleanup(c.Stop)

			ts := c.startTime.Add(10 * time.Second).UTC()
			ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
			ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")
			pid := "12345"

			// ERROR line — capture its timestamp so we can assert it is preserved on the emitted entry.
			errorEntryTs := time.Now()
			receiver.Chan() <- loki.Entry{Entry: push.Entry{
				Timestamp: errorEntryTs,
				Line:      ts1 + ":127.0.0.1:5432:user@books_store:[" + pid + "]:1:42P01:" + ts2 + ":1/0:0:c1::psqlERROR:  relation \"missing\" does not exist",
			}}
			// STATEMENT continuation
			receiver.Chan() <- loki.Entry{Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "STATEMENT:  SELECT * FROM missing WHERE id = $1",
			}}

			require.Eventually(t, func() bool {
				for _, e := range entryHandler.Received() {
					if string(e.Labels["op"]) == "pg_error" {
						return true
					}
				}
				return false
			}, 2*time.Second, 50*time.Millisecond, "expected a pg_error entry")

			var errEntry loki.Entry
			for _, e := range entryHandler.Received() {
				if string(e.Labels["op"]) == "pg_error" {
					errEntry = e
					break
				}
			}

			// Assert structured metadata carries the fingerprint regardless of redaction.
			var gotFP string
			for _, m := range errEntry.Entry.StructuredMetadata {
				if m.Name == "query_fingerprint" {
					gotFP = m.Value
				}
			}
			require.NotEmpty(t, gotFP, "fingerprint should be set when STATEMENT is present")

			// Timestamp must match the ERROR entry's timestamp, not the time the entry was emitted.
			require.True(t, errEntry.Entry.Timestamp.Equal(errorEntryTs), "pg_error entry should preserve the source timestamp")

			// And the line carries the structured fields
			require.Contains(t, errEntry.Entry.Line, `severity="ERROR"`)
			require.Contains(t, errEntry.Entry.Line, `sqlstate="42P01"`)
			require.Contains(t, errEntry.Entry.Line, `datname="books_store"`)
			require.Contains(t, errEntry.Entry.Line, `user="user"`)

			if tc.disableQueryRedaction {
				require.Contains(t, errEntry.Entry.Line, `statement_preview="SELECT * FROM missing WHERE id = $1"`)
			} else {
				require.NotContains(t, errEntry.Entry.Line, `statement_preview=`,
					"statement_preview must be omitted when query redaction is on")
			}
		})
	}
}

func TestLogsCollector_EmitsErrorWithEmptyFingerprintAfterTimeout(t *testing.T) {
	entryHandler := loki.NewCollectingHandler()
	defer entryHandler.Stop()
	registry := prometheus.NewRegistry()

	receiver := loki.NewLogsReceiver()
	c, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)
	c.pendingErrorTimeout = 100 * time.Millisecond // tighten for the test

	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := c.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	// ERROR line with no following STATEMENT
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      ts1 + ":127.0.0.1:5432:user@books_store:[99999]:1:53300:" + ts2 + ":1/0:0:c1::psqlFATAL:  too many connections",
	}}

	require.Eventually(t, func() bool {
		for _, e := range entryHandler.Received() {
			if string(e.Labels["op"]) == "pg_error" {
				return true
			}
		}
		return false
	}, 2*time.Second, 50*time.Millisecond, "timeout-emitted entry should appear")

	var errEntry loki.Entry
	for _, e := range entryHandler.Received() {
		if string(e.Labels["op"]) == "pg_error" {
			errEntry = e
			break
		}
	}
	for _, m := range errEntry.Entry.StructuredMetadata {
		if m.Name == "query_fingerprint" {
			require.Equal(t, "", m.Value, "fingerprint should be empty when no STATEMENT arrived")
		}
	}
	require.Contains(t, errEntry.Entry.Line, `severity="FATAL"`)
	// statement_preview is gated behind disable_query_redaction; with the
	// default (redaction on) the field is omitted entirely.
	require.NotContains(t, errEntry.Entry.Line, `statement_preview=`)
}

func TestLogsCollector_ExcludeUsers(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
		ExcludeUsers: []string{"excluded_user"},
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	ts := startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	excludedLog := ts1 + ":10.0.1.5(12345):excluded_user@testdb:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"
	allowedLog := ts1 + ":10.0.1.5(12345):allowed_user@testdb:[9113]:5:57014:" + ts2 + ":25/113:0:693c34cb.2399::psqlERROR:  canceling statement"

	collector.Receiver().Chan() <- loki.Entry{Entry: push.Entry{Line: excludedLog, Timestamp: time.Now()}}
	collector.Receiver().Chan() <- loki.Entry{Entry: push.Entry{Line: allowedLog, Timestamp: time.Now()}}

	time.Sleep(200 * time.Millisecond)

	mfs, _ := registry.Gather()
	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_errors_total" {
			for _, metric := range mf.GetMetric() {
				labels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				totalCount += metric.GetCounter().GetValue()
				require.Equal(t, "allowed_user", labels["user"], "only allowed_user should produce metrics")
			}
		}
	}
	require.Equal(t, float64(1), totalCount, "only the non-excluded user log should be counted")
}

func TestLogsCollector_EmitsSlowQueryWithFingerprint(t *testing.T) {
	cases := []struct {
		name                  string
		disableQueryRedaction bool
	}{
		{name: "redaction_on_default", disableQueryRedaction: false},
		{name: "redaction_off", disableQueryRedaction: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entryHandler := loki.NewCollectingHandler()
			defer entryHandler.Stop()
			registry := prometheus.NewRegistry()

			receiver := loki.NewLogsReceiver()
			c, err := NewLogs(LogsArguments{
				Receiver:              receiver,
				EntryHandler:          entryHandler,
				Logger:                log.NewNopLogger(),
				Registry:              registry,
				DisableQueryRedaction: tc.disableQueryRedaction,
			})
			require.NoError(t, err)
			require.NoError(t, c.Start(context.Background()))
			t.Cleanup(c.Stop)

			ts := c.startTime.Add(10 * time.Second).UTC()
			ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
			ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

			const sqlText = "SELECT pg_sleep(1)"
			// Capture the inbound timestamp so we can assert it is preserved on the emitted entry.
			inboundTs := time.Now()
			receiver.Chan() <- loki.Entry{Entry: push.Entry{
				Timestamp: inboundTs,
				Line:      ts1 + ":127.0.0.1:5432:user@books_store:[12345]:1:00000:" + ts2 + ":1/0:0:c1::psqlLOG:  duration: 1234.567 ms  statement: " + sqlText,
			}}

			require.Eventually(t, func() bool {
				for _, e := range entryHandler.Received() {
					if string(e.Labels["op"]) == "pg_slow_query" {
						return true
					}
				}
				return false
			}, 2*time.Second, 50*time.Millisecond, "expected a pg_slow_query entry")

			var slowEntry loki.Entry
			for _, e := range entryHandler.Received() {
				if string(e.Labels["op"]) == "pg_slow_query" {
					slowEntry = e
					break
				}
			}

			expectedFP, _, fpErr := fingerprint.Fingerprint(sqlText, fingerprint.SourceLog, 0)
			require.NoError(t, fpErr)
			require.NotEmpty(t, expectedFP)

			var gotFP string
			for _, m := range slowEntry.Entry.StructuredMetadata {
				if m.Name == "query_fingerprint" {
					gotFP = m.Value
				}
			}
			require.Equal(t, expectedFP, gotFP)

			// Timestamp must match the inbound entry's timestamp, not the time the entry was emitted.
			require.True(t, slowEntry.Entry.Timestamp.Equal(inboundTs), "pg_slow_query entry should preserve the source timestamp")

			require.Contains(t, slowEntry.Entry.Line, `datname="books_store"`)
			require.Contains(t, slowEntry.Entry.Line, `user="user"`)
			require.Contains(t, slowEntry.Entry.Line, `duration_ms="1234.567"`)

			if tc.disableQueryRedaction {
				require.Contains(t, slowEntry.Entry.Line, `statement_preview="SELECT pg_sleep(1)"`)
			} else {
				require.NotContains(t, slowEntry.Entry.Line, `statement_preview=`,
					"statement_preview must be omitted when query redaction is on")
			}
		})
	}
}

// TestLogsCollector_DisplacedPendingErrorIsEmittedNotDropped asserts that when
// a PID issues a new ERROR before its predecessor's STATEMENT continuation
// arrives, the predecessor is emitted (with empty fingerprint) rather than
// being silently overwritten in the pendingErrors map.
func TestLogsCollector_DisplacedPendingErrorIsEmittedNotDropped(t *testing.T) {
	entryHandler := loki.NewCollectingHandler()
	defer entryHandler.Stop()
	registry := prometheus.NewRegistry()

	receiver := loki.NewLogsReceiver()
	c, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)
	c.pendingErrorTimeout = 100 * time.Millisecond // tighten so the second pending entry flushes within the test
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := c.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")
	pid := "12345"

	// Two ERRORs from the same PID with no STATEMENT between them. The first
	// is displaced when the second arrives; the second flushes via timeout.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      ts1 + ":127.0.0.1:5432:user@d:[" + pid + "]:1:42P01:" + ts2 + ":1/0:0:c1::psqlERROR:  first",
	}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{
		Timestamp: time.Now(),
		Line:      ts1 + ":127.0.0.1:5432:user@d:[" + pid + "]:1:42P02:" + ts2 + ":1/0:0:c1::psqlERROR:  second",
	}}

	// Both should produce a pg_error Loki entry; the first must not be silently dropped.
	require.Eventually(t, func() bool {
		count := 0
		for _, e := range entryHandler.Received() {
			if string(e.Labels["op"]) == "pg_error" {
				count++
			}
		}
		return count == 2
	}, 2*time.Second, 50*time.Millisecond, "expected both pg_error entries to be emitted")
}

// TestLogsCollector_DoesNotDeadlockWhenEmittingPgError is a regression test for
// the production deadlock where EntryHandler was wired to the input receiver's
// channel. Production now uses a separate fanout channel for emitted entries;
// this test asserts the collector tolerates a modest backlog without
// deadlocking.
func TestLogsCollector_DoesNotDeadlockWhenEmittingPgError(t *testing.T) {
	receiver := loki.NewLogsReceiver()
	// EntryHandler wired to a buffered channel separate from receiver, mirroring
	// how production wires it to the component's fanout.
	out := make(chan loki.Entry, 4)
	entryHandler := loki.NewEntryHandler(out, func() {})
	registry := prometheus.NewRegistry()

	c, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: entryHandler,
		Logger:       log.NewNopLogger(),
		Registry:     registry,
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := c.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	// Send 3 ERROR + STATEMENT pairs back-to-back. Without the fix, the first
	// emission would block because EntryHandler.Chan() == receiver.Chan().
	for i := 0; i < 3; i++ {
		receiver.Chan() <- loki.Entry{Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      ts1 + ":127.0.0.1:5432:user@d:[" + fmt.Sprintf("%d", 1000+i) + "]:1:42P01:" + ts2 + ":1/0:0:c1::psqlERROR:  err",
		}}
		receiver.Chan() <- loki.Entry{Entry: push.Entry{
			Timestamp: time.Now(),
			Line:      "STATEMENT:  SELECT " + fmt.Sprintf("%d", i),
		}}
	}

	// 3 pg_error entries should be readable from the output channel.
	for i := 0; i < 3; i++ {
		select {
		case e := <-out:
			require.Equal(t, "pg_error", string(e.Labels["op"]))
		case <-time.After(2 * time.Second):
			t.Fatalf("collector deadlocked at entry %d", i+1)
		}
	}
}

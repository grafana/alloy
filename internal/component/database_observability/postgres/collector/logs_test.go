package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logfmt/logfmt"
	"github.com/grafana/loki/pkg/push"
	"github.com/hoophq/alcatraz"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestLogsCollector_ParseRDSFormat(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
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

			require.Eventuallyf(t, func() bool {
				mfs, _ := registry.Gather()
				for _, mf := range mfs {
					if mf.GetName() == "database_observability_pg_non_query_errors_total" {
						for _, metric := range mf.GetMetric() {
							labels := make(map[string]string)
							for _, label := range metric.GetLabel() {
								labels[label.GetName()] = label.GetValue()
							}
							if labels["user"] == tt.wantUser && labels["datname"] == tt.wantDB && labels["severity"] == tt.wantSev && labels["sqlstate"] == tt.wantSQLState {
								return labels["sqlstate_class"] == tt.wantSQLState[:2]
							}
						}
					}
				}
				return false
			}, 2*time.Second, 5*time.Millisecond, "metric not found for %s", tt.name)
		})
	}
}

func TestLogsCollector_SkipsNonErrors(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)

	// Build INFO and LOG lines with timestamps AFTER collector start, so they would pass the
	// historical filter if they reached it. They are skipped for severity (not ERROR/FATAL/PANIC).
	ts := collector.startTime.Add(10 * time.Second).UTC()
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
		require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: logLine}}))
	}

	// Should have 0 metrics since all were skipped
	mfs, _ := registry.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
			require.Equal(t, 0, len(mf.GetMetric()), "should not create metrics for non-error logs")
		}
	}
}

// TestLogsCollector_DoesNotCountEmbeddedSeverityKeyword pins the robustness of
// severity detection: a non-error line whose text embeds an "ERROR:" keyword
// (here a LOG-level logged statement, as emitted with log_statement=all) must
// not be counted as an error. The line's real leading label (LOG) shadows the
// embedded keyword.
func TestLogsCollector_DoesNotCountEmbeddedSeverityKeyword(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)

	ts := collector.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	// A LOG-level statement line whose SQL text embeds an "ERROR:" keyword.
	// SQLSTATE is 00000 (successful completion).
	logLine := ts1 + ":10.0.1.5(12345):app-user@books_store:[9112]:4:00000:" + ts2 +
		":25/112:0:693c34cb.2398::psqlLOG:  statement: SELECT 'ERROR:' AS msg"

	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: logLine}}))

	mfs, _ := registry.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
			require.Equal(t, 0, len(mf.GetMetric()), "a LOG line with an embedded ERROR: keyword must not be counted")
		}
	}
}

func TestLogsCollector_MetricSumming(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 100), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
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

	type metricKey struct {
		user string
		db   string
		sev  string
	}

	gatherCounts := func() map[metricKey]float64 {
		mfs, _ := registry.Gather()
		counts := make(map[metricKey]float64)
		for _, mf := range mfs {
			if mf.GetName() == "database_observability_pg_non_query_errors_total" {
				for _, metric := range mf.GetMetric() {
					labels := make(map[string]string)
					for _, label := range metric.GetLabel() {
						labels[label.GetName()] = label.GetValue()
					}
					counts[metricKey{
						user: labels["user"],
						db:   labels["datname"],
						sev:  labels["severity"],
					}] = metric.GetCounter().GetValue()
				}
			}
		}
		return counts
	}

	require.Eventually(t, func() bool {
		counts := gatherCounts()
		return counts[metricKey{user: "user1", db: "db1", sev: "ERROR"}] == 3 &&
			counts[metricKey{user: "user2", db: "db2", sev: "FATAL"}] == 1
	}, 2*time.Second, 5*time.Millisecond, "expected counters did not reach target values")

	counts := gatherCounts()
	require.Len(t, counts, 2, "should have 2 unique label combinations")
	require.Equal(t, float64(3), counts[metricKey{user: "user1", db: "db1", sev: "ERROR"}], "user1@db1:ERROR should have count of 3")
	require.Equal(t, float64(1), counts[metricKey{user: "user2", db: "db2", sev: "FATAL"}], "user2@db2:FATAL should have count of 1")
}

func TestLogsCollector_InvalidFormat(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
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

	require.Eventually(t, func() bool {
		mfs, _ := registry.Gather()
		for _, mf := range mfs {
			if mf.GetName() == "database_observability_pg_error_log_parse_failures_total" {
				return mf.GetMetric()[0].GetCounter().GetValue() > 0
			}
		}
		return false
	}, 2*time.Second, 5*time.Millisecond, "parse error metric should be incremented")
}

func TestLogsCollector_EmptyUserAndDatabase(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
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

	require.Eventually(t, func() bool {
		mfs, _ := registry.Gather()
		for _, mf := range mfs {
			if mf.GetName() == "database_observability_pg_non_query_errors_total" && len(mf.GetMetric()) == 1 {
				return mf.GetMetric()[0].GetCounter().GetValue() == 1
			}
		}
		return false
	}, 2*time.Second, 5*time.Millisecond, "expected one FATAL metric with count 1")

	mfs, _ := registry.Gather()
	var errorMetrics *dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
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
		Logger:       logging.NewSlogNop(),
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

func TestLogsCollector_SQLStateExtraction(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
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

			require.Eventuallyf(t, func() bool {
				mfs, _ := registry.Gather()
				for _, mf := range mfs {
					if mf.GetName() == "database_observability_pg_non_query_errors_total" {
						for _, metric := range mf.GetMetric() {
							labels := make(map[string]string)
							for _, label := range metric.GetLabel() {
								labels[label.GetName()] = label.GetValue()
							}
							if labels["sqlstate"] == tt.wantSQLState {
								return labels["sqlstate_class"] == tt.wantSQLStateClass &&
									labels["severity"] == tt.wantSeverity
							}
						}
					}
				}
				return false
			}, 2*time.Second, 5*time.Millisecond, "metric with sqlstate=%s not found for %s", tt.wantSQLState, tt.name)
		})
	}
}

func TestLogsCollector_SkipsHistoricalLogs(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)

	historicalTime := collector.startTime.Add(-1 * time.Hour)
	recentTime := collector.startTime.Add(10 * time.Second)

	historicalLine := fmt.Sprintf("%s:[local]:user@database:[1234]:1:28000:%s:1/1:0:000000.0::psqlERROR:  test historical error",
		historicalTime.Format("2006-01-02 15:04:05.000 MST"),
		historicalTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Historical line: %s", historicalLine)

	recentLine := fmt.Sprintf("%s:[local]:user@database:[1234]:1:28000:%s:1/1:0:000000.0::psqlERROR:  test recent error",
		recentTime.Format("2006-01-02 15:04:05.000 MST"),
		recentTime.Format("2006-01-02 15:04:05 MST"))
	t.Logf("Recent line: %s", recentLine)

	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: historicalLine}}))
	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: recentLine}}))

	mfs, err := registry.Gather()
	require.NoError(t, err)

	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
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
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)

	historicalTime := collector.startTime.Add(-1 * time.Hour)
	historicalLine := fmt.Sprintf("%s:[local]:user@database:[1234]:1:28000:%s:1/1:0:000000.0::psqlERROR:  test historical error",
		historicalTime.Format("2006-01-02 15:04:05.000 MST"),
		historicalTime.Format("2006-01-02 15:04:05 MST"))

	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: historicalLine}}))

	mfs, err := registry.Gather()
	require.NoError(t, err)

	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
			for _, metric := range mf.GetMetric() {
				totalCount += metric.GetCounter().GetValue()
			}
		}
	}
	require.Equal(t, float64(0), totalCount, "historical logs must not produce metrics")
}

func TestLogsCollector_NonUTCLogTimezone(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	pstWall := startTime.Add(10 * time.Second).Add(-8 * time.Hour) // PST wall-clock for real UTC startTime+10s
	ts1 := pstWall.Format("2006-01-02 15:04:05.000") + " PST"
	ts2 := pstWall.Add(-1*time.Second).Format("2006-01-02 15:04:05") + " PST"

	logLine := ts1 + ":[local]:app-user@books_store:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"

	collector.Receiver().Chan() <- loki.Entry{
		Entry: push.Entry{
			Line:      logLine,
			Timestamp: time.Now(),
		},
	}

	require.Eventually(t, func() bool {
		mfs, _ := registry.Gather()
		var totalCount float64
		for _, mf := range mfs {
			if mf.GetName() == "database_observability_pg_non_query_errors_total" {
				for _, metric := range mf.GetMetric() {
					totalCount += metric.GetCounter().GetValue()
				}
			}
		}
		return totalCount == 1
	}, 2*time.Second, 5*time.Millisecond, "log with non-UTC abbreviation timezone must be counted")
}

func TestLogsCollector_LogTimezoneCountsRecentNonUTC(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)

	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)
	collector.logTimezone.Store(loc)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	abs := startTime.Add(10 * time.Second)
	inLoc := abs.In(loc)
	abbrev, _ := inLoc.Zone()
	ts1 := inLoc.Format("2006-01-02 15:04:05.000") + " " + abbrev
	ts2 := inLoc.Add(-1*time.Second).Format("2006-01-02 15:04:05") + " " + abbrev

	logLine := ts1 + ":[local]:app-user@books_store:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"
	collector.Receiver().Chan() <- loki.Entry{Entry: push.Entry{Line: logLine, Timestamp: time.Now()}}

	require.Eventually(t, func() bool {
		mfs, _ := registry.Gather()
		var totalCount float64
		for _, mf := range mfs {
			if mf.GetName() == "database_observability_pg_non_query_errors_total" {
				for _, metric := range mf.GetMetric() {
					totalCount += metric.GetCounter().GetValue()
				}
			}
		}
		return totalCount == 1
	}, 2*time.Second, 5*time.Millisecond, "recent non-UTC log must be counted when log_timezone Location is supplied")
}

func TestLogsCollector_LogTimezoneFiltersHistoricalNonUTC(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)

	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)
	collector.logTimezone.Store(loc)

	abs := collector.startTime.Add(-1 * time.Hour)
	inLoc := abs.In(loc)
	abbrev, _ := inLoc.Zone()
	ts1 := inLoc.Format("2006-01-02 15:04:05.000") + " " + abbrev
	ts2 := inLoc.Add(-1*time.Second).Format("2006-01-02 15:04:05") + " " + abbrev

	logLine := ts1 + ":[local]:app-user@books_store:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"
	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: logLine}}))

	mfs, _ := registry.Gather()
	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
			for _, metric := range mf.GetMetric() {
				totalCount += metric.GetCounter().GetValue()
			}
		}
	}
	require.Equal(t, float64(0), totalCount, "historical non-UTC log must be dropped when log_timezone Location is supplied")
}

func TestLogsCollector_LogTimezoneAbbrevMismatchFallsBack(t *testing.T) {
	// Europe/London emits GMT/BST — neither matches the PST in the log line.
	loc, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)

	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)
	collector.logTimezone.Store(loc)

	startTime := collector.startTime
	err = collector.Start(context.Background())
	require.NoError(t, err)
	defer collector.Stop()

	pstWall := startTime.Add(-2 * time.Hour)
	ts1 := pstWall.Format("2006-01-02 15:04:05.000") + " PST"
	ts2 := pstWall.Add(-1*time.Second).Format("2006-01-02 15:04:05") + " PST"

	logLine := ts1 + ":[local]:app-user@books_store:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"
	collector.Receiver().Chan() <- loki.Entry{Entry: push.Entry{Line: logLine, Timestamp: time.Now()}}

	require.Eventually(t, func() bool {
		mfs, _ := registry.Gather()
		var totalCount float64
		for _, mf := range mfs {
			if mf.GetName() == "database_observability_pg_non_query_errors_total" {
				for _, metric := range mf.GetMetric() {
					totalCount += metric.GetCounter().GetValue()
				}
			}
		}
		return totalCount == 1
	}, 2*time.Second, 5*time.Millisecond, "stale/mismatched log_timezone must fall back to counting the log, not silently drop it")
}

func TestLogsCollector_ExcludeDatabases(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:         loki.NewLogsReceiver(),
		EntryHandler:     entryHandler,
		Logger:           logging.NewSlogNop(),
		Registry:         registry,
		ExcludeDatabases: []string{"excluded_db"},
	})
	require.NoError(t, err)

	ts := collector.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	excludedLog := ts1 + ":10.0.1.5(12345):app-user@excluded_db:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"
	allowedLog := ts1 + ":10.0.1.5(12345):app-user@allowed_db:[9113]:5:57014:" + ts2 + ":25/113:0:693c34cb.2399::psqlERROR:  canceling statement"

	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: excludedLog}}))
	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: allowedLog}}))

	mfs, _ := registry.Gather()
	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
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

func TestLogsCollector_ExcludeUsers(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
	registry := prometheus.NewRegistry()

	collector, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: entryHandler,
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
		ExcludeUsers: []string{"excluded_user"},
	})
	require.NoError(t, err)

	ts := collector.startTime.Add(10 * time.Second).UTC()
	ts1 := ts.Format("2006-01-02 15:04:05.000 MST")
	ts2 := ts.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	excludedLog := ts1 + ":10.0.1.5(12345):excluded_user@testdb:[9112]:4:57014:" + ts2 + ":25/112:0:693c34cb.2398::psqlERROR:  canceling statement"
	allowedLog := ts1 + ":10.0.1.5(12345):allowed_user@testdb:[9113]:5:57014:" + ts2 + ":25/113:0:693c34cb.2399::psqlERROR:  canceling statement"

	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: excludedLog}}))
	require.NoError(t, collector.parseTextLog(loki.Entry{Entry: push.Entry{Line: allowedLog}}))

	mfs, _ := registry.Gather()
	var totalCount float64
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_non_query_errors_total" {
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

// newServerLogCollector builds a Logs collector without enable_error_logs_processing
// (the server-log categories need no SQL fingerprinting, so they work on a
// cgo-less build too) and returns it with its entry channel.
func newServerLogCollector(t *testing.T) (*Logs, chan loki.Entry) {
	t.Helper()
	entryCh := make(chan loki.Entry, 8)
	c, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: loki.NewEntryHandler(entryCh, func() {}),
		Logger:       logging.NewSlogNop(),
		Registry:     prometheus.NewRegistry(),
	})
	require.NoError(t, err)
	return c, entryCh
}

// serverLogLine builds a prefixed LOG-severity line for msg (which may embed
// literal newlines, e.g. autovacuum's multi-line ereport text).
func serverLogLine(c *Logs, pid, msg string) string {
	return logTS(c) + "::user@books_store:[" + pid + "]:1:00000:LOG:  " + msg
}

// preAuthServerLogLine builds a prefixed LOG-severity line shaped like a
// pre-authentication event (SSL handshake failure, client I/O error before
// login succeeds): PostgreSQL logs %u@%d as the literal "[unknown]@[unknown]"
// in this case, and %v (vxid) is empty since no backend transaction has been
// assigned yet.
func preAuthServerLogLine(c *Logs, pid, sqlstate, msg string) string {
	return logTS(c) + ":10.0.0.5(54321):[unknown]@[unknown]:[" + pid + "]:1:" + sqlstate + ":" + logTS(c) + "::0:6abe0000.0001:[unknown]:LOG:  " + msg
}

// fullPrefixServerLogLine builds a prefixed LOG-severity line with a
// complete %s:%v:%x:%c:%q%a run, so sessionMetaRegex matches and
// session_start_time/vxid/xid/session_id/application_name are all populated
// from the log_line_prefix meta -- unlike serverLogLine, which omits that
// run entirely.
func fullPrefixServerLogLine(c *Logs, pid, vxid, xid, msg string) string {
	ts := logTS(c)
	return ts + ":10.0.0.5(54321):user@books_store:[" + pid + "]:1:00000:" + ts + ":" + vxid + ":" + xid + ":6abe0000.0001:myapp:LOG:  " + msg
}

func TestLogsCollector_ServerLog_Checkpoint(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "checkpoint complete: wrote 344 buffers (2.1%); 0 WAL file(s) added, 0 removed, 0 recycled; write=0.202 s, sync=0.005 s, total=0.215 s; sync files=6, longest=0.003 s, average=0.001 s; distance=1234 kB, estimate=1234 kB"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "100", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "server_log", string(got[0].Labels["op"]))
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "checkpoint", fields["category"])
	require.Equal(t, "344", fields["buffers_written"])
	require.Equal(t, "0", fields["wal_added"])
	require.Equal(t, "0", fields["wal_removed"])
	require.Equal(t, "0", fields["wal_recycled"])
	require.Equal(t, "0.202", fields["write_seconds"])
	require.Equal(t, "0.005", fields["sync_seconds"])
	require.Equal(t, "0.215", fields["total_seconds"])

	_, hasSqlstate := fields["sqlstate"]
	require.False(t, hasSqlstate, `sqlstate="00000" is the case for every non-error log line and adds nothing`)
	_, hasSqlstateName := fields["sqlstate_name"]
	require.False(t, hasSqlstateName)
}

func TestLogsCollector_ServerLog_AutovacuumVacuum(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "automatic vacuum of table \"books_store.public.books\": index scans: 1\n" +
		"pages: 0 removed, 443 remain, 0 skipped due to pins, 0 skipped frozen\n" +
		"tuples: 10000 removed, 50000 remain, 0 are dead but not yet removable\n" +
		"system usage: CPU: user: 0.01 s, system: 0.00 s, elapsed: 0.05 s"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "101", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "autovacuum", fields["category"])
	require.Equal(t, "vacuum", fields["operation"])
	require.Equal(t, "books_store.public.books", fields["table"])
	require.Equal(t, "0.05", fields["elapsed_seconds"])
}

func TestLogsCollector_ServerLog_AutovacuumAnalyze(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "automatic analyze of table \"books_store.public.books\"\n" +
		"system usage: CPU: user: 0.01 s, system: 0.00 s, elapsed: 0.03 s"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "102", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "autovacuum", fields["category"])
	require.Equal(t, "analyze", fields["operation"])
	require.Equal(t, "books_store.public.books", fields["table"])
	require.Equal(t, "0.03", fields["elapsed_seconds"])
}

func TestLogsCollector_ServerLog_ConnectionAuthorized(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "connection authorized: user=app_user database=books_store application_name=myapp"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "103", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "connection", fields["category"])
	// user/datname come from the general log_line_prefix meta (the test
	// helper's "user"/"books_store"), not the message's own "app_user" --
	// the meta field is preferred whenever it's available, and the
	// pattern's own "database" capture is suppressed entirely rather than
	// emitted redundantly under a second key.
	require.Equal(t, "user", fields["user"])
	require.Equal(t, "books_store", fields["datname"])
	_, hasDatabase := fields["database"]
	require.False(t, hasDatabase, "pattern's own database capture is suppressed in favor of the meta datname field")
	require.Equal(t, "myapp", fields["application_name"])
	_, hasHost := fields["host"]
	require.False(t, hasHost, "client_addr stays deferred for connections")
}

func TestLogsCollector_ServerLog_ConnectionAuthorized_NoApplicationName(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "connection authorized: user=app_user database=books_store"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "104", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "user", fields["user"])
	require.Equal(t, "books_store", fields["datname"])
	_, hasAppName := fields["application_name"]
	require.False(t, hasAppName, "absent application_name must not appear as an empty field")
}

// TestLogsCollector_ServerLog_ConnectionReceivedIsDropped pins that
// "connection received:" (host/port only, no user/database) is never
// captured: client_addr stays deferred, and that line carries nothing else
// useful.
func TestLogsCollector_ServerLog_ConnectionReceivedIsDropped(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "connection received: host=10.0.1.5 port=54321"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "105", msg)}}))

	got := drainEntries(t, entryCh, 1, 300*time.Millisecond)
	require.Len(t, got, 0, `"connection received:" must never be captured`)
}

func TestLogsCollector_ServerLog_Disconnection(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "disconnection: session time: 1:02:03.456 user=app_user database=books_store host=127.0.0.1 port=54321"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "106", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "disconnection", fields["category"])
	require.Equal(t, "1:02:03.456", fields["session_time"])
	require.Equal(t, "user", fields["user"])
	require.Equal(t, "books_store", fields["datname"])
	_, hasDatabase := fields["database"]
	require.False(t, hasDatabase, "pattern's own database capture is suppressed in favor of the meta datname field")
	_, hasHost := fields["host"]
	require.False(t, hasHost, "client_addr stays deferred for disconnections")
}

func TestLogsCollector_ServerLog_LockWait_StillWaiting(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "process 12345 still waiting for ShareLock on transaction 67890 after 1000.000 ms"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "107", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "lock_wait", fields["category"])
	// pid comes from the general log_line_prefix meta (the test helper's
	// "107"), not the message's own "process 12345" -- the same dedup as
	// connection/disconnection's user/datname, since this is always the
	// same backend reporting on itself.
	require.Equal(t, "107", fields["pid"])
	// PostgreSQL's own wording is "still waiting for"; normalized to the
	// shorter "waiting" here.
	require.Equal(t, "waiting", fields["phase"])
	require.Equal(t, "ShareLock", fields["lock_type"])
	require.Equal(t, "67890", fields["xid"], "a transaction-shaped target is split out, not left as the opaque target string")
	require.Equal(t, "1000.000", fields["wait_ms"])
	_, hasTarget := fields["target"]
	require.False(t, hasTarget)
}

// TestLogsCollector_ServerLog_LockWait_XidPrefersPatternOverMeta pins that
// lock_wait's own xid (the transaction actually being waited on, parsed
// from the message text) wins over the general log_line_prefix meta's own
// xid (this backend's own transaction, often 0 for a read-only waiter, and
// unrelated to what it's blocked on) when both are present.
func TestLogsCollector_ServerLog_LockWait_XidPrefersPatternOverMeta(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "process 12345 still waiting for ShareLock on transaction 67890 after 1000.000 ms"
	line := fullPrefixServerLogLine(c, "119", "22/9", "555", msg)
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "67890", fields["xid"], "the waited-on transaction wins over this backend's own (555)")
	require.Equal(t, "22/9", fields["vxid"], "vxid has no pattern-level counterpart, so the meta value still comes through")
}

func TestLogsCollector_ServerLog_LockWait_Acquired(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "process 12345 acquired ShareLock on transaction 67890 after 1234.567 ms"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "108", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "lock_wait", fields["category"])
	require.Equal(t, "acquired", fields["phase"])
	require.Equal(t, "1234.567", fields["wait_ms"])
}

// TestLogsCollector_ServerLog_LockWait_FailedToAcquire pins PostgreSQL's
// third log_lock_waits phase: lock_timeout expiring instead of the lock
// eventually being granted. This was previously unrecognized entirely
// (only "still waiting for"/"acquired" matched), so the line was silently
// dropped.
func TestLogsCollector_ServerLog_LockWait_FailedToAcquire(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "process 12345 failed to acquire ShareLock on transaction 67890 after 5000.123 ms"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "113", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "lock_wait", fields["category"])
	require.Equal(t, "failed to acquire", fields["phase"])
	require.Equal(t, "67890", fields["xid"])
}

// TestLogsCollector_ServerLog_LockWait_RelationTarget pins the other common
// lock_wait target shape: a relation OID within a database OID. PostgreSQL
// never resolves this to a table name in a plain log_lock_waits message
// (unlike a deadlock's DETAIL+CONTEXT), so only the OIDs are available.
func TestLogsCollector_ServerLog_LockWait_RelationTarget(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "process 12345 still waiting for AccessExclusiveLock on relation 16394 of database 16388 after 1000.000 ms"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "114", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "16394", fields["relation_oid"])
	require.Equal(t, "16388", fields["database_oid"])
	_, hasTarget := fields["target"]
	require.False(t, hasTarget)
	_, hasXid := fields["xid"]
	require.False(t, hasXid)
}

// TestLogsCollector_ServerLog_LockWait_TupleTarget pins the tuple-level lock
// shape: two sessions contending for the same row (e.g. both running
// SELECT ... FOR UPDATE on it) log a wait on a specific tuple (page, offset)
// within a relation/database, confirmed live against a real instance -- a
// common case, not a rare one like the doc comment on the regexes once
// assumed.
func TestLogsCollector_ServerLog_LockWait_TupleTarget(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "process 7026 still waiting for AccessExclusiveLock on tuple (1677,98) of relation 20944 of database 20582 after 1000.312 ms"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "116", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "1677,98", fields["tuple"])
	require.Equal(t, "20944", fields["relation_oid"])
	require.Equal(t, "20582", fields["database_oid"])
	_, hasTarget := fields["target"]
	require.False(t, hasTarget)
}

// TestLogsCollector_ServerLog_LockWait_AdvisoryTargetStaysOpaque pins that a
// target shape splitLockWaitTarget doesn't recognize (e.g. an advisory
// lock's own 4-tuple format) is left as the raw target string, rather than
// dropped or mis-split.
func TestLogsCollector_ServerLog_LockWait_AdvisoryTargetStaysOpaque(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "process 12345 acquired ExclusiveLock on advisory lock [5,0,999888,1] after 2012.793 ms"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "115", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "advisory lock [5,0,999888,1]", fields["target"])
	_, hasRelationOID := fields["relation_oid"]
	require.False(t, hasRelationOID)
	_, hasXid := fields["xid"]
	require.False(t, hasXid)
}

func TestLogsCollector_ServerLog_TempFile(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := `temporary file: path "base/pgsql_tmp/pgsql_tmp12345.0", size 1048576`
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "109", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "temp_file", fields["category"])
	require.Equal(t, "base/pgsql_tmp/pgsql_tmp12345.0", fields["path"])
	require.Equal(t, "1048576", fields["size_bytes"])
}

func TestLogsCollector_ServerLog_ReplicationCommand(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := `received replication command: START_REPLICATION SLOT "sub1" LOGICAL 0/1634FF8`
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "110", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "replication_command", fields["category"])
	require.Equal(t, "START_REPLICATION", fields["command"])
	require.Equal(t, `SLOT "sub1" LOGICAL 0/1634FF8`, fields["args"])
}

func TestLogsCollector_ServerLog_ReplicationCommand_NoArgs(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "received replication command: IDENTIFY_SYSTEM"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "111", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "IDENTIFY_SYSTEM", fields["command"])
	_, hasArgs := fields["args"]
	require.False(t, hasArgs)
}

// TestLogsCollector_ServerLog_UnrecognizedLogLineDroppedSilently pins the
// registry's closed-world policy: a LOG-severity line that matches none of
// the registered categories (nor op="slow_query"/op="explain_plan_output",
// which are checked before the registry) is dropped without error, never
// captured as an unstructured blob.
func TestLogsCollector_ServerLog_UnrecognizedLogLineDroppedSilently(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "some made-up LOG message that matches no known category"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "112", msg)}}))

	got := drainEntries(t, entryCh, 1, 300*time.Millisecond)
	require.Len(t, got, 0, "an unrecognized LOG line must never be captured")
}

// TestLogsCollector_ServerLog_WarningAndNoticeSeveritiesDispatch pins that
// WARNING and NOTICE (not just LOG) also reach the server-log registry.
func TestLogsCollector_ServerLog_WarningAndNoticeSeveritiesDispatch(t *testing.T) {
	c, entryCh := newServerLogCollector(t)
	ts := logTS(c)

	warnLine := ts + "::user@books_store:[113]:1:00000:WARNING:  temporary file: path \"base/pgsql_tmp/pgsql_tmp1.0\", size 2048"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: warnLine}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))
	require.Equal(t, "temp_file", fields["category"])
}

func TestLogsCollector_ServerLog_ConnectionAuthenticated(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := `connection authenticated: identity="postgres" method=scram-sha-256 (/rdsdbdata/config/pg_hba.conf:1)`
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "116", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "connection_authenticated", fields["category"])
	// user comes from the general log_line_prefix meta (the test helper's
	// "user"), not the message's own identity="postgres" -- same dedup as
	// connection/disconnection, since %u is already set to the authenticated
	// identity by the time this line is logged.
	require.Equal(t, "user", fields["user"])
	_, hasIdentity := fields["identity"]
	require.False(t, hasIdentity, "the authenticated identity is named user, not identity")
	require.Equal(t, "scram-sha-256", fields["method"])
	require.Equal(t, "1", fields["pg_hba_line"])
}

func TestLogsCollector_ServerLog_ClientIOError(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "could not receive data from client: Connection reset by peer"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "117", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "client_io_error", fields["category"])
	require.Equal(t, "Connection reset by peer", fields["reason"])
}

func TestLogsCollector_ServerLog_TLSHandshakeFailure(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "could not accept SSL connection: EOF detected"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "118", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "tls_handshake_failure", fields["category"])
	require.Equal(t, "EOF detected", fields["reason"])
}

// TestLogsCollector_ServerLog_ClientIOError_PreAuth and
// TestLogsCollector_ServerLog_TLSHandshakeFailure_PreAuth pin a real crash:
// client_io_error and tls_handshake_failure happen almost exclusively on
// pre-authentication connections, where PostgreSQL logs %u@%d as the literal
// "[unknown]@[unknown]". parseTextLog located the PID's closing "]" with
// strings.Index(afterAt, "]") searched from the start of afterAt, which
// matched the "]" closing "[unknown]" instead of the PID's own "]" --
// producing a negative slice range and panicking on every real occurrence of
// these two categories.
func TestLogsCollector_ServerLog_ClientIOError_PreAuth(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "could not receive data from client: Connection reset by peer"
	line := preAuthServerLogLine(c, "21696", "08006", msg)
	require.NotPanics(t, func() {
		require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))
	})

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "client_io_error", fields["category"])
	require.Equal(t, "Connection reset by peer", fields["reason"])
	_, hasSqlstateName := fields["sqlstate_name"]
	require.False(t, hasSqlstateName, "sqlstate is never included on server_log entries")
}

func TestLogsCollector_ServerLog_TLSHandshakeFailure_PreAuth(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "could not accept SSL connection: EOF detected"
	line := preAuthServerLogLine(c, "22847", "08P01", msg)
	require.NotPanics(t, func() {
		require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))
	})

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "tls_handshake_failure", fields["category"])
	require.Equal(t, "EOF detected", fields["reason"])
	_, hasSqlstateName := fields["sqlstate_name"]
	require.False(t, hasSqlstateName, "sqlstate is never included on server_log entries")
}

// TestLogsCollector_ServerLog_ConfigReload_Sighup through
// TestLogsCollector_ServerLog_ConfigReload_InvalidParameter pin
// config_reload's 4 sub-shapes, distinguished by the "kind" field since they
// all share one category name.
func TestLogsCollector_ServerLog_ConfigReload_Sighup(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := "received SIGHUP, reloading configuration files"
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "119", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "config_reload", fields["category"])
	require.Equal(t, "sighup", fields["kind"])
}

func TestLogsCollector_ServerLog_ConfigReload_ParameterChanged(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := `parameter "log_lock_waits" changed to "on"`
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "120", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "config_reload", fields["category"])
	require.Equal(t, "parameter_changed", fields["kind"])
	require.Equal(t, "log_lock_waits", fields["param_name"])
	require.Equal(t, "on", fields["param_value"])
}

func TestLogsCollector_ServerLog_ConfigReload_FileError(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := `configuration file "/rdsdbdata/config/postgresql.conf" contains errors; unaffected changes were applied`
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "121", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "config_reload", fields["category"])
	require.Equal(t, "file_error", fields["kind"])
	require.Equal(t, "/rdsdbdata/config/postgresql.conf", fields["config_file"])
}

func TestLogsCollector_ServerLog_ConfigReload_InvalidParameter(t *testing.T) {
	c, entryCh := newServerLogCollector(t)

	msg := `invalid configuration parameter name "foo_bar"`
	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: serverLogLine(c, "122", msg)}}))

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "config_reload", fields["category"])
	require.Equal(t, "invalid_parameter", fields["kind"])
	require.Equal(t, "foo_bar", fields["param_name"])
}

// drainEntries reads up to want entries from the handler's channel within
// timeout. Returns whatever arrived in order.
func drainEntries(t *testing.T, ch chan loki.Entry, want int, timeout time.Duration) []loki.Entry {
	t.Helper()
	out := make([]loki.Entry, 0, want)
	deadline := time.Now().Add(timeout)
	for len(out) < want && time.Now().Before(deadline) {
		select {
		case e := <-ch:
			out = append(out, e)
		case <-time.After(25 * time.Millisecond):
		}
	}
	return out
}

// parseLogfmt decodes a logfmt entry body into a map using the same logfmt
// library production consumers use, so encoder bugs can't hide behind a
// matching bespoke test parser.
func parseLogfmt(t *testing.T, s string) map[string]string {
	t.Helper()
	out := map[string]string{}
	decoder := logfmt.NewDecoder(strings.NewReader(s))
	for decoder.ScanRecord() {
		for decoder.ScanKeyval() {
			out[string(decoder.Key())] = string(decoder.Value())
		}
	}
	require.NoError(t, decoder.Err(), "entry body must be valid logfmt")
	return out
}

// requireOnlyFields asserts the parsed logfmt entry contains exactly the given
// keys and no others, so a field silently added or dropped from the
// op="error_message" body fails the test that exercises it.
func requireOnlyFields(t *testing.T, fields map[string]string, allowed ...string) {
	t.Helper()
	allow := make(map[string]struct{}, len(allowed))
	for _, k := range allowed {
		allow[k] = struct{}{}
		require.Containsf(t, fields, k, "expected field %q to be present", k)
	}
	for k := range fields {
		_, ok := allow[k]
		require.Truef(t, ok, "unexpected field %q in minimal op=error_message entry", k)
	}
}

// startErrorLogs builds and starts a logs collector with op="error_message" emission
// enabled, returning it with its receiver and entry channel. A non-zero timeout
// overrides pendingErrorTimeout before Start (the run loop reads it once at
// startup, so the timeout/race tests must set it here).
func startErrorLogs(t *testing.T, timeout time.Duration) (*Logs, loki.LogsReceiver, chan loki.Entry) {
	t.Helper()
	if !fingerprint.Supported() {
		t.Skip("op=error_message emission requires SQL fingerprinting, which needs a cgo build")
	}
	receiver := loki.NewLogsReceiver()
	entryCh := make(chan loki.Entry, 8)
	c, err := NewLogs(LogsArguments{
		Receiver:                  receiver,
		EntryHandler:              loki.NewEntryHandler(entryCh, func() {}),
		Logger:                    logging.NewSlogNop(),
		Registry:                  prometheus.NewRegistry(),
		EnableErrorLogsProcessing: true,
	})
	require.NoError(t, err)
	if timeout > 0 {
		c.pendingErrorTimeout = timeout
	}
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)
	return c, receiver, entryCh
}

// logTS returns a log-line timestamp after the collector's start time (so it
// passes the historical-log filter), formatted as PostgreSQL emits it.
func logTS(c *Logs) string {
	return c.startTime.Add(10 * time.Second).UTC().Format("2006-01-02 15:04:05.000 MST")
}

func TestLogsCollector_EmitsErrorEntry_OnErrorPlusStatement(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "12345"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42P01:STATEMENT:  SELECT * FROM missing WHERE id = $1"}}
	// The next prefix line flushes the buffered STATEMENT deterministically.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "error_message", string(got[0].Labels["op"]))

	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))
	expectedFP, fpErr := fingerprint.Fingerprint("SELECT * FROM missing WHERE id = $1")
	require.NoError(t, fpErr)

	require.Equal(t, "ERROR", fields["severity"])
	require.Equal(t, "books_store", fields["datname"])
	require.Equal(t, expectedFP, fields["query_fingerprint"])
	require.Equal(t, "user", fields["user"])
	require.Equal(t, pid, fields["pid"])
	require.Equal(t, "42P01", fields["sqlstate"])
	require.Equal(t, "42", fields["sqlstate_class"])

	// client_addr and the SQL text itself are deferred; this fixture's prefix
	// omits the %s:%v:%x:%c run, so those fields are absent too.
	requireOnlyFields(t, fields, "severity", "datname", "query_fingerprint", "user", "pid", "sqlstate", "sqlstate_class", "sqlstate_name", "sqlstate_class_name", "message", "line_number")
}

// TestLogsCollector_EmitsErrorEntry_IncludesMessageField pins that the ERROR
// line's own message text (the text after the severity label, before any
// TestLogsCollector_EmitsErrorEntry_IncludesSessionMetaFields pins that the
// session-metadata fields already present in the required log_line_prefix
// (%l line number, %s session start time, %v vxid, %x transaction id, %c
// session id) are captured and emitted, none of them requiring redaction
// (all server-generated, non-PII).
func TestLogsCollector_EmitsErrorEntry_IncludesSessionMetaFields(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70012"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:7:42P01:2026-09-25 20:15:41 UTC:435/321499:168673161:6ab6d66d.6901:psql:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:8:42P01:2026-09-25 20:15:41 UTC:435/321499:168673161:6ab6d66d.6901:psql:STATEMENT:  SELECT * FROM missing"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:9:00000:2026-09-25 20:15:41 UTC:435/321499:168673161:6ab6d66d.6901:psql:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "7", fields["line_number"])
	require.Equal(t, "2026-09-25 20:15:41 UTC", fields["session_start_time"])
	require.Equal(t, "435/321499", fields["vxid"])
	require.Equal(t, "168673161", fields["xid"])
	require.Equal(t, "6ab6d66d.6901", fields["session_id"])
}

// TestLogsCollector_EmitsErrorEntry_RedactsPIIInApplicationName pins that
// application_name (%a) is captured, and — since it's client-controlled, a
// client could set it to anything — redacted the same way message/detail are.
func TestLogsCollector_EmitsErrorEntry_RedactsPIIInApplicationName(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70013"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:2026-09-25 20:15:41 UTC:435/321499:0:6ab6d66d.6901:myapp for john@example.com:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42P01:2026-09-25 20:15:41 UTC:435/321499:0:6ab6d66d.6901:myapp for john@example.com:STATEMENT:  SELECT * FROM missing"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:2026-09-25 20:15:41 UTC:435/321499:0:6ab6d66d.6901:myapp for john@example.com:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.NotContains(t, fields["application_name"], "john@example.com")
	require.Contains(t, fields["application_name"], "myapp")
}

// TestLogsCollector_SessionIDMismatch_RejectsStatementDespitePidMatch pins
// that %c (session id) is preferred over pid for pairing when both lines have
// one: a backend pid can be reused by a new session over a long-lived
// instance's uptime, but session id cannot. A STATEMENT that shares the
// pending's pid but carries a different session id must be rejected exactly
// as if it belonged to a different backend.
func TestLogsCollector_SessionIDMismatch_RejectsStatementDespitePidMatch(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 100*time.Millisecond)
	ts := logTS(c)
	pid := "70014"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:2026-09-25 20:15:41 UTC:435/321499:0:6ab6d66d.6901:psql:ERROR:  relation \"missing\" does not exist"}}
	// Same pid, but a different session id: a reused pid from a new backend,
	// not a continuation of the pending error above.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:2026-09-25 20:15:41 UTC:999/111111:0:ffffffff.ffff:psql:STATEMENT:  SELECT * FROM missing"}}

	// The rejected STATEMENT leaves the original pending without one, so it
	// still surfaces as op="error_message" once the timeout fires, just
	// without query_fingerprint, rather than a (wrongly) paired one.
	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "error_message", string(got[0].Labels["op"]))
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))
	_, hasFP := fields["query_fingerprint"]
	require.False(t, hasFP, "the rejected STATEMENT must never be fingerprinted into this entry")
}

// TestLogsCollector_FallsBackToPid_WhenSessionIDUnavailable pins that pairing
// still works by pid when the %s:%v:%x:%c run can't be parsed from a line —
// e.g. a future log_line_prefix variant that omits %c. Here the STATEMENT
// line goes straight from SQLSTATE to the label (no session-metadata run at
// all), unlike the ERROR line, which has the full run.
func TestLogsCollector_FallsBackToPid_WhenSessionIDUnavailable(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70015"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:2026-09-25 20:15:41 UTC:435/321499:0:6ab6d66d.6901:psql:ERROR:  relation \"missing\" does not exist"}}
	// No %s:%v:%x:%c run here — as if a different log_line_prefix were in use.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42P01:STATEMENT:  SELECT * FROM missing"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	expectedFP, fpErr := fingerprint.Fingerprint("SELECT * FROM missing")
	require.NoError(t, fpErr)
	require.Equal(t, expectedFP, fields["query_fingerprint"], "pid fallback must still pair the STATEMENT")
}

// STATEMENT/DETAIL continuation) is captured and emitted as the "message"
// field.
func TestLogsCollector_EmitsErrorEntry_IncludesMessageField(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70001"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42P01:STATEMENT:  SELECT * FROM missing"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, `relation "missing" does not exist`, fields["message"])
}

// TestLogsCollector_EmitsErrorEntry_RedactsPIIInMessage pins that a literal
// value embedded directly in the ERROR message (e.g. a truncated value on a
// varchar length violation) is redacted before emission, not passed through
// verbatim.
// TestLogs_Redact exercises the redact() helper directly, bypassing the log
// parsing pipeline, so PII coverage doesn't depend on constructing a full
// ERROR+STATEMENT sequence for every case.
func TestLogs_Redact(t *testing.T) {
	l := &Logs{piiEngine: alcatraz.NewEngine()}

	tests := []struct {
		name          string
		input         string
		wantUnchanged bool
		wantAbsent    []string
		wantPresent   []string
	}{
		{
			name:          "safe diagnostic text is left unchanged",
			input:         `duplicate key value violates unique constraint "users_email_key"`,
			wantUnchanged: true,
		},
		{
			name:          "a column named email is not itself PII",
			input:         `column "email" does not exist`,
			wantUnchanged: true,
		},
		{
			name:        "an email address is redacted",
			input:       "Key (email)=(john@example.com) already exists.",
			wantAbsent:  []string{"john@example.com"},
			wantPresent: []string{"already exists"},
		},
		{
			name:       "a Luhn-valid credit card number is redacted",
			input:      "Key (card_number)=(4111 1111 1111 1111) already exists.",
			wantAbsent: []string{"4111 1111 1111 1111"},
		},
		{
			name:       "multiple PII values in the same string are all redacted",
			input:      "contact john@example.com or card 4111 1111 1111 1111",
			wantAbsent: []string{"john@example.com", "4111 1111 1111 1111"},
		},
		{
			name:          "empty string is left unchanged",
			input:         "",
			wantUnchanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := l.redact(tt.input)
			if tt.wantUnchanged {
				require.Equal(t, tt.input, got)
			}
			for _, s := range tt.wantAbsent {
				require.NotContains(t, got, s)
			}
			for _, s := range tt.wantPresent {
				require.Contains(t, got, s)
			}
		})
	}
}

func TestLogsCollector_EmitsErrorEntry_RedactsPIIInMessage(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70002"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:22001:ERROR:  value too long for type character varying(50): \"john.doe@example.com is way too long a value\""}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:22001:STATEMENT:  INSERT INTO t (a) VALUES ($1)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.NotContains(t, fields["message"], "john.doe@example.com")
}

// TestLogsCollector_EmitsErrorEntry_IncludesDetailField_Redacted pins the
// motivating case for this whole feature: a unique-constraint violation's
// DETAIL line embeds the literal offending value, which must be redacted
// before it leaves the process.
func TestLogsCollector_EmitsErrorEntry_IncludesDetailField_Redacted(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70003"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23505:ERROR:  duplicate key value violates unique constraint \"users_email_key\""}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23505:DETAIL:  Key (email)=(john@example.com) already exists."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:23505:STATEMENT:  INSERT INTO users (email) VALUES ($1)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.NotContains(t, fields["detail"], "john@example.com")
	require.Contains(t, fields["detail"], "already exists")
}

// TestLogsCollector_EmitsErrorEntry_IncludesHintAndContextFields pins that
// HINT and CONTEXT continuation lines are captured the same way DETAIL is.
func TestLogsCollector_EmitsErrorEntry_IncludesHintAndContextFields(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70004"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42703:ERROR:  column \"emial\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42703:HINT:  Perhaps you meant to reference the column \"users.email\"."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:42703:CONTEXT:  SQL statement \"SELECT emial FROM users\""}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:42703:STATEMENT:  SELECT emial FROM users"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:5:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Contains(t, fields["hint"], `Perhaps you meant to reference the column "users.email"`)
	require.Contains(t, fields["context"], `SQL statement "SELECT emial FROM users"`)
}

// TestLogsCollector_EmitsErrorEntry_ExtractsPlpgsqlContext and
// TestLogsCollector_EmitsErrorEntry_ExtractsPlpgsqlContext_NoLine pin that a
// PL/pgSQL call-site CONTEXT is surfaced as structured plpgsql_function/
// plpgsql_line/plpgsql_statement fields instead of the generic redacted
// context field -- function name and line number are never PII, the same
// class of safe structured data as a deadlock's relation name.
func TestLogsCollector_EmitsErrorEntry_ExtractsPlpgsqlContext(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70012"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:P0001:ERROR:  boom from plpgsql"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:P0001:CONTEXT:  PL/pgSQL function inline_code_block line 3 at RAISE"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:P0001:STATEMENT:  DO $$ BEGIN RAISE EXCEPTION 'boom from plpgsql'; END $$;"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "inline_code_block", fields["plpgsql_function"])
	require.Equal(t, "3", fields["plpgsql_line"])
	require.Equal(t, "RAISE", fields["plpgsql_statement"])
	_, hasContext := fields["context"]
	require.False(t, hasContext, "a matched PL/pgSQL context is replaced by its structured fields, not duplicated")
}

func TestLogsCollector_EmitsErrorEntry_ExtractsPlpgsqlContext_NoLine(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70013"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:2F005:ERROR:  control reached end of function without RETURN"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:2F005:CONTEXT:  PL/pgSQL function bad_func()"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:2F005:STATEMENT:  SELECT bad_func()"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "bad_func()", fields["plpgsql_function"])
	_, hasLine := fields["plpgsql_line"]
	require.False(t, hasLine, "no line to blame for this error shape")
}

// TestLogsCollector_EmitsErrorEntry_RedactsPIIInHintAndContext closes a
// coverage gap: the HINT/CONTEXT capture test above only ever exercised safe
// text, so redaction was never actually proven to run on those two fields.
func TestLogsCollector_EmitsErrorEntry_RedactsPIIInHintAndContext(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70011"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23514:ERROR:  new row violates row-level security policy"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23514:HINT:  Contact john@example.com if you believe this is an error."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:23514:CONTEXT:  SQL statement \"SELECT notify('jane@example.com')\""}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:23514:STATEMENT:  INSERT INTO accounts (owner) VALUES ($1)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:5:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.NotContains(t, fields["hint"], "john@example.com")
	require.Contains(t, fields["hint"], "Contact")
	require.NotContains(t, fields["context"], "jane@example.com")
	require.Contains(t, fields["context"], "SQL statement")
}

// TestLogsCollector_MultiLineDetailThenStatement_RoutesContinuationsCorrectly
// pins that a bare TAB-continuation line always extends whichever field was
// opened most recently: a DETAIL spanning two lines, followed by a STATEMENT
// spanning two lines, must not bleed into each other.
func TestLogsCollector_MultiLineDetailThenStatement_RoutesContinuationsCorrectly(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70005"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23514:ERROR:  new row violates check constraint"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23514:DETAIL:  Failing row contains (1, active,"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tsuspended)."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:23514:STATEMENT:  UPDATE accounts SET status ="}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\t'suspended' WHERE id = $1"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "Failing row contains (1, active,\nsuspended).", fields["detail"])

	expectedSQL := "UPDATE accounts SET status =\n'suspended' WHERE id = $1"
	expectedFP, fpErr := fingerprint.Fingerprint(expectedSQL)
	require.NoError(t, fpErr)
	require.Equal(t, expectedFP, fields["query_fingerprint"])
}

// TestLogsCollector_LostStatement_EmitsErrorEntryWithoutFingerprint pins that when the
// STATEMENT line never arrives (e.g. dropped by the log forwarder, or
// PostgreSQL simply never logs one for this error shape -- confirmed live,
// e.g. a jsonpath syntax error), the captured message/DETAIL still reaches
// Loki once the timeout fires, as op="error_message" without
// query_fingerprint -- never silently dropped.
func TestLogsCollector_LostStatement_EmitsErrorEntryWithoutFingerprint(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 100*time.Millisecond)
	ts := logTS(c)
	pid := "70006"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23505:ERROR:  duplicate key value violates unique constraint \"users_email_key\""}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23505:DETAIL:  Key (email)=(john@example.com) already exists."}}
	// STATEMENT line lost in transit — never arrives.

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "error_message", string(got[0].Labels["op"]))
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))

	require.Equal(t, "ERROR", fields["severity"])
	require.Equal(t, "23505", fields["sqlstate"])
	require.Equal(t, `duplicate key value violates unique constraint "users_email_key"`, fields["message"])
	require.NotContains(t, fields["detail"], "john@example.com", "PII in detail must still be redacted on this path")
	_, hasFP := fields["query_fingerprint"]
	require.False(t, hasFP, "there's no query to fingerprint without a STATEMENT")
}

// TestLogsCollector_LostDetail_EntryStillEmittedWithoutDetailField pins that
// when the DETAIL line never arrives (lost in transit) but STATEMENT does,
// the entry still emits normally — it just has no "detail" field, rather
// than blocking or corrupting emission.
func TestLogsCollector_LostDetail_EntryStillEmittedWithoutDetailField(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70007"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23505:ERROR:  duplicate key value violates unique constraint \"users_email_key\""}}
	// DETAIL line lost in transit — never arrives.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23505:STATEMENT:  INSERT INTO users (email) VALUES ($1)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	_, hasDetail := fields["detail"]
	require.False(t, hasDetail, "no DETAIL line arrived; the field must be absent, not empty or corrupted")
	require.NotEmpty(t, fields["query_fingerprint"], "the STATEMENT that did arrive is still paired normally")
}

// TestLogsCollector_OrphanedContinuation_DroppedWhenNoFieldOpen pins that a
// bare TAB-continuation line arriving before any labeled field has been
// opened (e.g. its own label line was lost in transit) is safely dropped —
// it must not panic, and must not be misattached to any field once a real
// one (STATEMENT) does open.
func TestLogsCollector_OrphanedContinuation_DroppedWhenNoFieldOpen(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70008"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:ERROR:  relation \"missing\" does not exist"}}
	// Orphaned continuation: its label line (e.g. a DETAIL header) was lost.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\torphaned continuation text"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42P01:STATEMENT:  SELECT * FROM missing"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	expectedFP, fpErr := fingerprint.Fingerprint("SELECT * FROM missing")
	require.NoError(t, fpErr)
	require.Equal(t, expectedFP, fields["query_fingerprint"], "orphaned text must not have leaked into the STATEMENT")
	_, hasDetail := fields["detail"]
	require.False(t, hasDetail, "orphaned text must not have opened a DETAIL field either")
}

// TestLogsCollector_LostLabelLine_RedactionStillCoversMisattributedText
// documents a known limitation of line-by-line continuation tracking: if a
// field's own label line is lost (here, HINT's header) but its bare
// TAB-continuation still arrives, that text is misattributed onto whichever
// field was still open (DETAIL) rather than dropped or kept separate — this
// parser has no way to distinguish "DETAIL's legitimate second line" from
// "an orphaned continuation for a field whose header never arrived". What
// must still hold despite that: redaction is applied to the field's full
// accumulated text at emission time, so PII is not bypassed by this failure
// mode even though the fields themselves get mixed up.
func TestLogsCollector_LostLabelLine_RedactionStillCoversMisattributedText(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70009"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23505:ERROR:  duplicate key value violates unique constraint \"users_email_key\""}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23505:DETAIL:  Key (email)=(john@example.com) already exists."}}
	// HINT header line lost in transit — only its bare continuation arrives,
	// so it lands on the still-open DETAIL field instead.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tConsider adding a unique index on (email, tenant_id)."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:23505:STATEMENT:  INSERT INTO users (email) VALUES ($1)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.NotContains(t, fields["detail"], "john@example.com", "PII must stay redacted even when a lost label line merges unrelated text into the field")
	require.Contains(t, fields["detail"], "already exists")
	require.Contains(t, fields["detail"], "Consider adding a unique index", "documents the misattribution: the orphaned HINT text landed on DETAIL")
	_, hasHint := fields["hint"]
	require.False(t, hasHint, "HINT's own header never arrived, so no hint field opens")
}

// TestLogsCollector_LostErrorLine_OrphanedContinuationsSafelyIgnored pins
// that when the ERROR line itself is lost (so no pendingError was ever
// created) but its STATEMENT/DETAIL continuations arrive anyway, they are
// dropped without panicking or fabricating an entry.
func TestLogsCollector_LostErrorLine_OrphanedContinuationsSafelyIgnored(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70010"

	// ERROR line lost in transit — never arrives; no pendingError exists.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23505:DETAIL:  Key (email)=(john@example.com) already exists."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23505:STATEMENT:  INSERT INTO users (email) VALUES ($1)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 500*time.Millisecond)
	require.Len(t, got, 0, "no ERROR line ever opened a pendingError, so its orphaned continuations must not fabricate one")
}

func TestLogsCollector_TimedOutPendingEmitsErrorEntryWithoutFingerprint(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 100*time.Millisecond)
	ts := logTS(c)

	// FATAL with no following STATEMENT.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[99999]:1:53300:FATAL:  too many connections"}}

	// op="error_message" arrives once the timeout fires -- no STATEMENT ever
	// came, so there's no query_fingerprint, but it's not dropped either.
	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "error_message", string(got[0].Labels["op"]))
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))
	require.Equal(t, "FATAL", fields["severity"])
	require.Equal(t, "too many connections", fields["message"])

	// pg_non_query_errors_total increments once the timeout flushes it,
	// since this error never got attributed to a query; pg_query_errors_total
	// never does for this one.
	require.Equal(t, float64(1), testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("FATAL", "53300", "53", "too_many_connections", "insufficient_resources", "books_store", "user")))
	require.Equal(t, float64(0), testutil.ToFloat64(c.queryErrors.WithLabelValues("FATAL", "53300", "53", "too_many_connections", "insufficient_resources", "books_store", "user")))
}

// TestLogsCollector_MatchedError_DoesNotIncrementErrorsWithoutQuery pins
// that an ERROR+STATEMENT pair that resolves normally -- it got attributed
// to a query -- never increments pg_non_query_errors_total; only the
// no-STATEMENT path (see TestLogsCollector_TimedOutPendingEmitsErrorEntryWithoutFingerprint) does.
func TestLogsCollector_MatchedError_DoesNotIncrementErrorsWithoutQuery(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "70020"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42P01:STATEMENT:  SELECT * FROM missing"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))
	_, hasFP := fields["query_fingerprint"]
	require.True(t, hasFP, "this error was matched to a query")

	require.Equal(t, float64(0), testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("ERROR", "42P01", "42", "undefined_table", "syntax_error_or_access_rule_violation", "books_store", "user")))
}

// TestLogsCollector_DisplacedPendingEmitsErrorEntryThenNormalEntry pins
// that ERROR #1, displaced before its STATEMENT ever arrived, is still
// emitted as op="error_message" (just without query_fingerprint) right when
// ERROR #2 displaces it -- never fabricated into a mispaired entry, and
// never silently dropped. ERROR #2 still completes normally, with its own
// query_fingerprint, once its own STATEMENT arrives.
func TestLogsCollector_DisplacedPendingEmitsErrorEntryThenNormalEntry(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "55555"

	// ERROR #1 displaced by ERROR #2 from the same backend.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:42P01:ERROR:  err one"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:42P02:ERROR:  err two"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:42P02:STATEMENT:  SELECT 2"}}
	// The next prefix line flushes the buffered STATEMENT.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 2, 1*time.Second)
	require.Len(t, got, 2, "err one (unmatched) and err two (matched) both reach Loki")

	unmatchedFields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))
	require.Equal(t, "error_message", string(got[0].Labels["op"]))
	require.Equal(t, "err one", unmatchedFields["message"])
	require.Equal(t, "42P01", unmatchedFields["sqlstate"])
	_, hasFP := unmatchedFields["query_fingerprint"]
	require.False(t, hasFP, "err one never got a STATEMENT, so it must have no query_fingerprint")

	fields := parseLogfmt(t, strings.TrimPrefix(got[1].Line, `level="error" `))
	require.Equal(t, "error_message", string(got[1].Labels["op"]))
	expectedFP, fpErr := fingerprint.Fingerprint("SELECT 2")
	require.NoError(t, fpErr)
	require.Equal(t, "ERROR", fields["severity"])
	require.Equal(t, expectedFP, fields["query_fingerprint"], "the second error's STATEMENT is the one matched")
	require.Equal(t, pid, fields["pid"])
	require.Equal(t, "42P02", fields["sqlstate"], "the second error's SQLSTATE is the one matched")
	requireOnlyFields(t, fields, "severity", "datname", "query_fingerprint", "user", "pid", "sqlstate", "sqlstate_class", "sqlstate_name", "sqlstate_class_name", "message", "line_number")
}

// TestLogsCollector_EmitsErrorEntry_PrefixedMultiLineStatement exercises the
// production shape: STATEMENT keyword line carries the prefix and is followed
// by TAB-prefixed continuations. The next prefix line flushes the buffer.
func TestLogsCollector_EmitsErrorEntry_PrefixedMultiLineStatement(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "38"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::app-user@books_store:[" + pid + "]:4:40001:ERROR:  could not serialize access due to concurrent update"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::app-user@books_store:[" + pid + "]:5:40001:STATEMENT:  WITH target_books AS ("}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tSELECT id FROM books WHERE id = $1"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\t)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tUPDATE books SET sold = true FROM target_books WHERE books.id = target_books.id"}}
	// The next prefix line flushes the buffered STATEMENT.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::app-user@books_store:[" + pid + "]:6:00000:LOG:  duration: 1.234 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))

	expectedSQL := "WITH target_books AS (\nSELECT id FROM books WHERE id = $1\n)\nUPDATE books SET sold = true FROM target_books WHERE books.id = target_books.id"
	expectedFP, fpErr := fingerprint.Fingerprint(expectedSQL)
	require.NoError(t, fpErr)

	require.Equal(t, "ERROR", fields["severity"])
	require.Equal(t, expectedFP, fields["query_fingerprint"])
	require.Equal(t, "app-user", fields["user"])
	require.Equal(t, pid, fields["pid"])
	// The multi-line STATEMENT is the behavior under test; fields stay minimal.
	requireOnlyFields(t, fields, "severity", "datname", "query_fingerprint", "user", "pid", "sqlstate", "sqlstate_class", "sqlstate_name", "sqlstate_class_name", "message", "line_number")
}

// TestLogsCollector_StatementSurvivesTimeoutFlush_EmitsEntry pins that an
// ERROR+STATEMENT pair with no following log line still emits when the pending
// expires: flushExpiredPending emits a pending that has its STATEMENT rather
// than dropping it. The 500ms timeout is generous against goroutine starvation
// under parallel-test contention.
func TestLogsCollector_StatementSurvivesTimeoutFlush_EmitsEntry(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 500*time.Millisecond)
	ts := logTS(c)
	pid := "310"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::app-user@books_store:[" + pid + "]:4:40001:ERROR:  could not serialize access due to concurrent update"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::app-user@books_store:[" + pid + "]:5:40001:STATEMENT:  INSERT INTO t (a) VALUES ($1)"}}

	got := drainEntries(t, entryCh, 1, 3*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))
	expectedFP, _ := fingerprint.Fingerprint("INSERT INTO t (a) VALUES ($1)")
	require.Equal(t, expectedFP, fields["query_fingerprint"])
}

// TestLogsCollector_DoesNotEmitErrorEntryWhenFingerprintDisabled confirms that
// with EnableErrorLogsProcessing explicitly false the component still increments
// pg_query_errors_total but never forwards an op="error_message" Loki entry.
func TestLogsCollector_DoesNotEmitErrorEntryWhenFingerprintDisabled(t *testing.T) {
	receiver := loki.NewLogsReceiver()
	entryCh := make(chan loki.Entry, 8)

	c, err := NewLogs(LogsArguments{
		Receiver:                  receiver,
		EntryHandler:              loki.NewEntryHandler(entryCh, func() {}),
		Logger:                    logging.NewSlogNop(),
		Registry:                  prometheus.NewRegistry(),
		EnableErrorLogsProcessing: false, // explicitly off
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := logTS(c)

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[12345]:1:42P01:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[12345]:2:42P01:STATEMENT:  SELECT * FROM missing WHERE id = $1"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[12345]:3:00000:LOG:  duration: 0.001 ms"}}

	// Wait for the buffering / flush logic to settle. pg_non_query_errors_total
	// should have incremented immediately: with processing off, no STATEMENT
	// is ever looked for, so this can never be attributed to one.
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("ERROR", "42P01", "42", "undefined_table", "syntax_error_or_access_rule_violation", "books_store", "user")) >= 1
	}, 2*time.Second, 50*time.Millisecond)

	// No Loki entries should have flowed.
	select {
	case e := <-entryCh:
		t.Fatalf("expected no op=error_message entry; got one: %s", e.Line)
	case <-time.After(300 * time.Millisecond):
		// good — silence
	}
}

// TestLogsCollector_EmitsErrorEntry_DefaultsToDisabled pins that omitting
// EnableErrorLogsProcessing from LogsArguments yields the disabled behavior:
// pg_non_query_errors_total still increments, but no op="error_message" Loki entry appears.
func TestLogsCollector_EmitsErrorEntry_DefaultsToDisabled(t *testing.T) {
	receiver := loki.NewLogsReceiver()
	entryCh := make(chan loki.Entry, 8)

	c, err := NewLogs(LogsArguments{
		Receiver:     receiver,
		EntryHandler: loki.NewEntryHandler(entryCh, func() {}),
		Logger:       logging.NewSlogNop(),
		Registry:     prometheus.NewRegistry(),
		// EnableErrorLogsProcessing intentionally omitted — defaults to false
	})
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	t.Cleanup(c.Stop)

	ts := logTS(c)

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[12345]:1:42P01:ERROR:  relation \"missing\" does not exist"}}

	// Counter should still increment.
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("ERROR", "42P01", "42", "undefined_table", "syntax_error_or_access_rule_violation", "books_store", "user")) >= 1
	}, 2*time.Second, 50*time.Millisecond)

	// And no Loki entry should appear.
	select {
	case e := <-entryCh:
		t.Fatalf("expected no op=error_message entry; got one: %s", e.Line)
	case <-time.After(300 * time.Millisecond):
		// good
	}
}

// TestLogsCollector_CountsErrorWithEmbeddedStatementKeyword pins that an
// ERROR line whose message text contains a STATEMENT keyword is classified
// by its leftmost real label (ERROR), not diverted to the statement-attach
// path -- it's still counted once it flushes (here, via timeout, since no
// real STATEMENT ever arrives for it), in pg_non_query_errors_total.
func TestLogsCollector_CountsErrorWithEmbeddedStatementKeyword(t *testing.T) {
	c, receiver, _ := startErrorLogs(t, 100*time.Millisecond)
	ts := logTS(c)
	line := ts + `::user@books_store:[123]:1:42601:ERROR:  syntax error at or near "STATEMENT:  SELECT"`

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: line}}

	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("ERROR", "42601", "42", "syntax_error", "syntax_error_or_access_rule_violation", "books_store", "user")) == 1
	}, 2*time.Second, 50*time.Millisecond, "an ERROR line with an embedded STATEMENT keyword must still be counted")
}

// TestLogsCollector_AppNameLabelDoesNotShadowSeverity pins that a label-like
// substring in the client-controlled application_name (%a sits between the
// SQLSTATE anchor and the real severity) cannot shadow the message's actual
// label: matching requires PostgreSQL's ":  " separator.
func TestLogsCollector_AppNameLabelDoesNotShadowSeverity(t *testing.T) {
	registry := prometheus.NewRegistry()
	c, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: loki.NewEntryHandler(make(chan loki.Entry, 1), func() {}),
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)

	tsBase := c.startTime.Add(10 * time.Second).UTC()
	ts1 := tsBase.Format("2006-01-02 15:04:05.000 MST")
	ts2 := tsBase.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")

	// application_name "etl-LOG:worker" contains "LOG:" before the real ERROR label.
	line := ts1 + ":10.0.1.5(12345):app-user@books_store:[9112]:4:57014:" + ts2 +
		":25/112:0:693c34cb.2398::etl-LOG:workerERROR:  canceling statement"

	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("ERROR", "57014", "57", "query_canceled", "operator_intervention", "books_store", "app-user"))
	require.Equal(t, float64(1), got, "a label-like application_name must not shadow the real severity")
}

// newLogsForClassify builds a Logs collector for severity-classification tests
// and returns it alongside the recent-line timestamp fields.
func newLogsForClassify(t *testing.T) (*Logs, *prometheus.Registry, string, string) {
	t.Helper()
	registry := prometheus.NewRegistry()
	c, err := NewLogs(LogsArguments{
		Receiver:     loki.NewLogsReceiver(),
		EntryHandler: loki.NewEntryHandler(make(chan loki.Entry, 1), func() {}),
		Logger:       logging.NewSlogNop(),
		Registry:     registry,
	})
	require.NoError(t, err)

	tsBase := c.startTime.Add(10 * time.Second).UTC()
	ts1 := tsBase.Format("2006-01-02 15:04:05.000 MST")
	ts2 := tsBase.Add(-1 * time.Second).Format("2006-01-02 15:04:05 MST")
	return c, registry, ts1, ts2
}

// TestLogsCollector_ForgedAppNameDoesNotHideError pins the hide direction: a
// client that sets application_name to exactly "LOG:  " (PostgreSQL's own
// separator, which pg_clean_ascii preserves) forges a benign label before the
// real ERROR. Walking to the last label in the run recovers the real severity,
// so the error is still counted rather than silently dropped.
func TestLogsCollector_ForgedAppNameDoesNotHideError(t *testing.T) {
	c, _, ts1, ts2 := newLogsForClassify(t)

	// application_name = "LOG:  " sits before the real ERROR label.
	line := ts1 + ":10.0.1.5(12345):app-user@books_store:[9112]:4:57014:" + ts2 +
		":25/112:0:693c34cb.2398::LOG:  ERROR:  canceling statement"

	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("ERROR", "57014", "57", "query_canceled", "operator_intervention", "books_store", "app-user"))
	require.Equal(t, float64(1), got, `a forged "LOG:  " application_name must not hide the real ERROR`)
}

// TestLogsCollector_ForgedAppNameDoesNotInflateError pins the inflate direction:
// a client that sets application_name to exactly "ERROR:  " forges an error
// label before the real LOG label on a benign statement line (SQLSTATE 00000).
// Walking to the last label recovers LOG, so nothing is counted.
func TestLogsCollector_ForgedAppNameDoesNotInflateError(t *testing.T) {
	c, registry, ts1, ts2 := newLogsForClassify(t)

	// application_name = "ERROR:  " sits before the real LOG label.
	line := ts1 + ":10.0.1.5(12345):app-user@books_store:[9112]:4:00000:" + ts2 +
		":25/112:0:693c34cb.2398::ERROR:  LOG:  statement: SELECT 1"

	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	mfs, _ := registry.Gather()
	for _, mf := range mfs {
		if mf.GetName() == "database_observability_pg_query_errors_total" {
			require.Equal(t, 0, len(mf.GetMetric()), `a forged "ERROR:  " application_name must not inflate the error count`)
		}
	}
}

// TestLogsCollector_MultipleForgedAppNameLabels pins that a run of several
// forged labels in application_name is fully consumed: the walk advances past
// every "<label>:  " token to the real severity PostgreSQL appends last.
func TestLogsCollector_MultipleForgedAppNameLabels(t *testing.T) {
	c, _, ts1, ts2 := newLogsForClassify(t)

	// application_name = "ERROR:  LOG:  " precedes the real FATAL label.
	line := ts1 + ":[local]:conn_user@testdb:[9449]:4:53300:" + ts2 +
		":91/57:0:693c34db.24e9::ERROR:  LOG:  FATAL:  too many connections"

	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := testutil.ToFloat64(c.nonQueryErrors.WithLabelValues("FATAL", "53300", "53", "too_many_connections", "insufficient_resources", "testdb", "conn_user"))
	require.Equal(t, float64(1), got, "multiple forged application_name labels must all be skipped to the real FATAL")
}

// TestLogsCollector_DeadlockDetail_FingerprintsCompetingQueries pins the
// motivating fix: a deadlock's DETAIL/CONTEXT get parsed into structured,
// query-text-free fields -- blocker pid, lock type, locked relation, and
// the blocker's query fingerprint (the victim's own fingerprint is just the
// existing top-level query_fingerprint, from STATEMENT) -- instead of a
// free-text detail/context pair that would otherwise carry raw SQL.
func TestLogsCollector_DeadlockDetail_FingerprintsCompetingQueries(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "18765"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:40P01:ERROR:  deadlock detected"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:40P01:DETAIL:  Process 18765 waits for ShareLock on transaction 596; blocked by process 18766."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 18766 waits for ShareLock on transaction 595; blocked by process 18765."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 18765: UPDATE accounts SET balance = balance - 100 WHERE id = 1;"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 18766: UPDATE accounts SET balance = balance - 100 WHERE id = 2;"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:40P01:CONTEXT:  while locking tuple (3,90) in relation \"accounts\""}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:40P01:STATEMENT:  UPDATE accounts SET balance = balance - 100 WHERE id = 1"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:5:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	victimFP, err := fingerprint.Fingerprint("UPDATE accounts SET balance = balance - 100 WHERE id = 1")
	require.NoError(t, err)
	blockerFP, err := fingerprint.Fingerprint("UPDATE accounts SET balance = balance - 100 WHERE id = 2;")
	require.NoError(t, err)

	require.Equal(t, "18766", fields["pid_blocker"])
	require.Equal(t, "ShareLock", fields["lock_type"])
	require.Equal(t, "accounts", fields["relation"])
	require.Equal(t, victimFP, fields["query_fingerprint"], "the victim's fingerprint is just the existing top-level field, from STATEMENT")
	require.Equal(t, blockerFP, fields["query_fingerprint_blocker"])
	for _, raw := range []string{"WHERE id = 1", "WHERE id = 2", "UPDATE accounts"} {
		require.NotContains(t, fields["query_fingerprint_blocker"], raw, "no raw query text may survive anywhere in the entry")
	}
	_, hasDetail := fields["detail"]
	require.False(t, hasDetail, "a handled deadlock must not also emit the generic, query-text-bearing detail field")
	_, hasContext := fields["context"]
	require.False(t, hasContext, "deadlock_relation already carries the one thing in CONTEXT worth keeping")
}

// TestLogsCollector_DeadlockDetail_BlockerFingerprintOmittedWhenEmpty pins
// that when the blocker's query text isn't available (here, an empty
// capture after its "Process N: " marker), the entry still emits with the
// structural fields (blocker pid, lock type) present, but
// query_fingerprint_blocker omitted -- never fabricated, and never falling
// back to raw/redacted query text.
func TestLogsCollector_DeadlockDetail_BlockerFingerprintOmittedWhenEmpty(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "18767"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:40P01:ERROR:  deadlock detected"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:40P01:DETAIL:  Process 18767 waits for ShareLock on transaction 1; blocked by process 18769."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 18769 waits for ShareLock on transaction 2; blocked by process 18767."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 18769:    "}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:40P01:STATEMENT:  SELECT 1"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1, "entry must still be emitted despite the blocker query being unavailable")
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.Equal(t, "18769", fields["pid_blocker"])
	require.Equal(t, "ShareLock", fields["lock_type"])
	_, hasFingerprint := fields["query_fingerprint_blocker"]
	require.False(t, hasFingerprint, "no query text was available for the blocker, so the fingerprint field must be omitted rather than fabricated")
	_, hasDetail := fields["detail"]
	require.False(t, hasDetail)
}

// TestLogsCollector_DeadlockDetail_OnlyAppliesToDeadlockSQLState pins that the
// fingerprint-based redaction only runs for sqlstate=40P01: a "Process N: ..."
// shaped DETAIL under any other SQLSTATE goes through the normal alcatraz
// redaction path unchanged (a false-positive match on unrelated text should
// not be fingerprinted as SQL).
func TestLogsCollector_DeadlockDetail_OnlyAppliesToDeadlockSQLState(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "18768"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:23505:ERROR:  duplicate key value violates unique constraint"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:23505:DETAIL:  Process 18768: contact john@example.com"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:23505:STATEMENT:  INSERT INTO users (email) VALUES ($1)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	require.NotContains(t, fields["detail"], "john@example.com", "non-deadlock DETAIL still goes through normal PII redaction")
	require.Contains(t, fields["detail"], "Process 18768:")
}

// TestLogsCollector_DeadlockDetail_MultiLineQuery_FingerprintsAcrossLines pins
// the real-world shape production traffic actually produces: a
// "Process <pid>: " marker with nothing after it on that physical line, and
// the competing query itself spread across the following continuation
// line(s) (pretty-printed, multi-line SQL is routine from real application
// code) -- including a traceparent SQL comment on both sides, which must
// come through as its own field rather than vanish with the discarded query
// text.
func TestLogsCollector_DeadlockDetail_MultiLineQuery_FingerprintsAcrossLines(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 0)
	ts := logTS(c)
	pid := "17236"

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:1:40P01:ERROR:  deadlock detected"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:2:40P01:DETAIL:  Process 17236 waits for ShareLock on transaction 1; blocked by process 16906."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 16906 waits for ShareLock on transaction 2; blocked by process 17236."}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 17236: "}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\t\tWITH candidates AS ("}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\t\t\tSELECT id FROM books WHERE stock > 10"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\t\t)"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\t\tSELECT * FROM candidates FOR UPDATE /*traceparent='00-aaaa-bbbb-01'*/"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\tProcess 16906: "}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(), Line: "\t\tSELECT * FROM books WHERE id = 1 FOR UPDATE /*traceparent='00-cccc-dddd-01'*/"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:3:40P01:STATEMENT:  WITH candidates AS (SELECT id FROM books WHERE stock > 10) SELECT * FROM candidates FOR UPDATE /*traceparent='00-aaaa-bbbb-01'*/"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[" + pid + "]:4:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	blockerFP, err := fingerprint.Fingerprint("SELECT * FROM books WHERE id = 1 FOR UPDATE /*traceparent='00-cccc-dddd-01'*/")
	require.NoError(t, err)

	require.Equal(t, "16906", fields["pid_blocker"])
	require.Equal(t, "ShareLock", fields["lock_type"])
	require.Equal(t, blockerFP, fields["query_fingerprint_blocker"])
	require.Equal(t, "00-aaaa-bbbb-01", fields["traceparent"])
	require.Equal(t, "00-cccc-dddd-01", fields["traceparent_blocker"])
	for _, raw := range []string{"FOR UPDATE", "stock > 10", "candidates"} {
		require.NotContains(t, fields["query_fingerprint_blocker"], raw, "no raw multi-line query text may survive anywhere in the entry")
	}
	_, hasDetail := fields["detail"]
	require.False(t, hasDetail)
}

// TestLogsCollector_StatementFromDifferentPidDoesNotAttach pins the PID guard:
// a STATEMENT line from another backend must not attach to the pending
// error, so interleaved streams cannot emit a mispaired op="error_message"
// entry -- it still surfaces as op="error_message" once the timeout fires,
// just without query_fingerprint.
func TestLogsCollector_StatementFromDifferentPidDoesNotAttach(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 100*time.Millisecond)
	ts := logTS(c)

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[111]:1:42P01:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[222]:1:42P02:STATEMENT:  SELECT * FROM other_backend"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[222]:2:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 2*time.Second)
	require.Len(t, got, 1)
	require.Equal(t, "error_message", string(got[0].Labels["op"]))
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="error" `))
	require.Equal(t, "111", fields["pid"], "a STATEMENT from a different PID must not pair with the pending error")
	_, hasFP := fields["query_fingerprint"]
	require.False(t, hasFP)
}

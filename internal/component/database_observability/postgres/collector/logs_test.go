package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logfmt/logfmt"
	"github.com/grafana/loki/pkg/push"
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
					if mf.GetName() == "database_observability_pg_errors_total" {
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
		if mf.GetName() == "database_observability_pg_errors_total" {
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
		if mf.GetName() == "database_observability_pg_errors_total" {
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
			if mf.GetName() == "database_observability_pg_errors_total" {
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
			if mf.GetName() == "database_observability_pg_errors_total" && len(mf.GetMetric()) == 1 {
				return mf.GetMetric()[0].GetCounter().GetValue() == 1
			}
		}
		return false
	}, 2*time.Second, 5*time.Millisecond, "expected one FATAL metric with count 1")

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
					if mf.GetName() == "database_observability_pg_errors_total" {
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
		if mf.GetName() == "database_observability_pg_errors_total" {
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
			if mf.GetName() == "database_observability_pg_errors_total" {
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
			if mf.GetName() == "database_observability_pg_errors_total" {
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
		if mf.GetName() == "database_observability_pg_errors_total" {
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
			if mf.GetName() == "database_observability_pg_errors_total" {
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
// keys and no others — pinning the minimal v1 op="error_message" field set so any
// re-added error-detail field fails the test until its own PR lands.
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

	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))
	expectedFP, fpErr := fingerprint.Fingerprint("SELECT * FROM missing WHERE id = $1")
	require.NoError(t, fpErr)

	require.Equal(t, "ERROR", fields["severity"])
	require.Equal(t, "books_store", fields["datname"])
	require.Equal(t, expectedFP, fields["query_fingerprint"])

	// v1 emits only the bare-minimum field set; error-detail fields (sqlstate,
	// pid, client/session, error_message, the SQL text, …) are deferred.
	requireOnlyFields(t, fields, "severity", "datname", "query_fingerprint")
}

func TestLogsCollector_TimedOutPendingDoesNotEmitErrorEntry(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 100*time.Millisecond)
	ts := logTS(c)

	// FATAL with no following STATEMENT.
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[99999]:1:53300:FATAL:  too many connections"}}

	// pg_errors_total increments immediately.
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("FATAL", "53300", "53", "books_store", "user")) == 1
	}, 1*time.Second, 50*time.Millisecond)

	// No entry should arrive even after the timeout window.
	got := drainEntries(t, entryCh, 1, 400*time.Millisecond)
	require.Len(t, got, 0, "no STATEMENT → no Loki entry")
}

func TestLogsCollector_DisplacedPendingEmitsExactlyOneEntry(t *testing.T) {
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
	require.Len(t, got, 1, "exactly one entry: only the second error's STATEMENT was matched")

	body := strings.TrimPrefix(got[0].Line, `level="info" `)
	fields := parseLogfmt(t, body)
	expectedFP, fpErr := fingerprint.Fingerprint("SELECT 2")
	require.NoError(t, fpErr)
	require.Equal(t, "ERROR", fields["severity"])
	require.Equal(t, expectedFP, fields["query_fingerprint"], "the second error's STATEMENT is the one matched")
	requireOnlyFields(t, fields, "severity", "datname", "query_fingerprint")
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
	fields := parseLogfmt(t, strings.TrimPrefix(got[0].Line, `level="info" `))

	expectedSQL := "WITH target_books AS (\nSELECT id FROM books WHERE id = $1\n)\nUPDATE books SET sold = true FROM target_books WHERE books.id = target_books.id"
	expectedFP, fpErr := fingerprint.Fingerprint(expectedSQL)
	require.NoError(t, fpErr)

	require.Equal(t, "ERROR", fields["severity"])
	require.Equal(t, expectedFP, fields["query_fingerprint"])
	// The multi-line STATEMENT is the behavior under test; fields stay minimal.
	requireOnlyFields(t, fields, "severity", "datname", "query_fingerprint")
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
// pg_errors_total but never forwards an op="error_message" Loki entry.
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

	// Wait for the buffering / flush logic to settle. pg_errors_total should
	// have incremented (gating doesn't touch it).
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("ERROR", "42P01", "42", "books_store", "user")) >= 1
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
// pg_errors_total still increments, but no op="error_message" Loki entry appears.
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
		return testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("ERROR", "42P01", "42", "books_store", "user")) >= 1
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
// ERROR line whose message text contains a STATEMENT keyword is still counted
// in pg_errors_total when enable_error_logs_processing is on: the line is classified by
// its leftmost real label (ERROR), not diverted to the statement-attach path.
func TestLogsCollector_CountsErrorWithEmbeddedStatementKeyword(t *testing.T) {
	if !fingerprint.Supported() {
		t.Skip("enable_error_logs_processing requires a cgo build")
	}
	registry := prometheus.NewRegistry()
	c, err := NewLogs(LogsArguments{
		Receiver:                  loki.NewLogsReceiver(),
		EntryHandler:              loki.NewEntryHandler(make(chan loki.Entry, 1), func() {}),
		Logger:                    logging.NewSlogNop(),
		Registry:                  registry,
		EnableErrorLogsProcessing: true,
	})
	require.NoError(t, err)

	ts := c.startTime.Add(10 * time.Second).UTC().Format("2006-01-02 15:04:05.000 MST")
	line := ts + `::user@books_store:[123]:1:42601:ERROR:  syntax error at or near "STATEMENT:  SELECT"`

	require.NoError(t, c.parseTextLog(loki.Entry{Entry: push.Entry{Line: line}}))

	got := testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("ERROR", "42601", "42", "books_store", "user"))
	require.Equal(t, float64(1), got, "an ERROR line with an embedded STATEMENT keyword must still be counted")
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

	got := testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("ERROR", "57014", "57", "books_store", "app-user"))
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

	got := testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("ERROR", "57014", "57", "books_store", "app-user"))
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
		if mf.GetName() == "database_observability_pg_errors_total" {
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

	got := testutil.ToFloat64(c.errorsBySQLState.WithLabelValues("FATAL", "53300", "53", "testdb", "conn_user"))
	require.Equal(t, float64(1), got, "multiple forged application_name labels must all be skipped to the real FATAL")
}

// TestLogsCollector_StatementFromDifferentPidDoesNotAttach pins the PID guard:
// a STATEMENT line from another backend must not attach to the pending error,
// so interleaved streams cannot emit a mispaired op="error_message" entry.
func TestLogsCollector_StatementFromDifferentPidDoesNotAttach(t *testing.T) {
	c, receiver, entryCh := startErrorLogs(t, 100*time.Millisecond)
	ts := logTS(c)

	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[111]:1:42P01:ERROR:  relation \"missing\" does not exist"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[222]:1:42P02:STATEMENT:  SELECT * FROM other_backend"}}
	receiver.Chan() <- loki.Entry{Entry: push.Entry{Timestamp: time.Now(),
		Line: ts + "::user@books_store:[222]:2:00000:LOG:  duration: 0.001 ms"}}

	got := drainEntries(t, entryCh, 1, 500*time.Millisecond)
	require.Len(t, got, 0, "a STATEMENT from a different PID must not pair with the pending error")
}

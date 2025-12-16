package collector

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestErrorLogsCollector_ParseJSON(t *testing.T) {
	tests := []struct {
		name          string
		jsonLog       string
		expectedError bool
		checkFields   func(*testing.T, *ParsedError)
	}{
		{
			name:          "real statement timeout from production",
			jsonLog:       `{"timestamp":"2025-12-12 15:29:16.068 GMT","user":"app-user","dbname":"books_store","pid":9112,"remote_host":"[local]","session_id":"693c34cb.2398","line_num":4,"ps":"SELECT","session_start":"2025-12-12 15:29:15 GMT","vxid":"25/112","txid":0,"error_severity":"ERROR","state_code":"57014","message":"canceling statement due to statement timeout","statement":"SELECT pg_sleep(5);","application_name":"psql","backend_type":"client backend","query_id":5457019535816659310}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "57014", p.SQLStateCode)
				require.Equal(t, "57", p.SQLStateClass)
				require.Equal(t, "Operator Intervention", p.ErrorCategory)
				require.Equal(t, "statement_timeout", p.TimeoutType)
			},
		},
		{
			name:          "real deadlock from production",
			jsonLog:       `{"timestamp":"2025-12-12 15:29:23.258 GMT","user":"app-user","dbname":"books_store","pid":9185,"remote_host":"[local]","session_id":"693c34cf.23e1","line_num":9,"ps":"UPDATE","session_start":"2025-12-12 15:29:19 GMT","vxid":"36/148","txid":837,"error_severity":"ERROR","state_code":"40P01","message":"deadlock detected","detail":"Process 9185 waits for ShareLock on transaction 836; blocked by process 9184.\nProcess 9184 waits for ShareLock on transaction 837; blocked by process 9185.\nProcess 9185: UPDATE books SET stock = stock WHERE id = 2;\nProcess 9184: UPDATE books SET stock = stock WHERE id = 1;","hint":"See server log for query details.","context":"while locking tuple (3,88) in relation \"books\"","statement":"UPDATE books SET stock = stock WHERE id = 2;","application_name":"psql","backend_type":"client backend","query_id":3188095831510673590}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "40P01", p.SQLStateCode)
				require.Equal(t, "40", p.SQLStateClass)
				require.Equal(t, "Transaction Rollback", p.ErrorCategory)
				require.Equal(t, "ShareLock", p.LockType)
				require.Equal(t, "3,88", p.TupleLocation)
				require.Equal(t, int32(9184), p.BlockerPID, "should extract blocker PID")
				require.Equal(t, "UPDATE books SET stock = stock WHERE id = 1;", p.BlockerQuery, "should extract blocker query")
				require.NotEmpty(t, p.Detail)
				require.NotEmpty(t, p.Hint)
				require.NotEmpty(t, p.Context)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})
			collector, err := NewErrorLogs(ErrorLogsArguments{
				Receiver:     loki.NewLogsReceiver(),
				EntryHandler: entryHandler,
				Logger:       log.NewNopLogger(),
				InstanceKey:  "test-instance",
				SystemID:     "test-system",
				Registry:     prometheus.NewRegistry(),
			})
			require.NoError(t, err)

			var jsonLog PostgreSQLJSONLog
			err = json.Unmarshal([]byte(tt.jsonLog), &jsonLog)
			require.NoError(t, err)

			parsed, err := collector.buildParsedError(&jsonLog)
			if tt.expectedError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, parsed)

			collector.extractInsights(parsed)

			if tt.checkFields != nil {
				tt.checkFields(t, parsed)
			}
		})
	}
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
	require.NotNil(t, collector)
	require.NotNil(t, collector.Receiver(), "receiver should be exported")

	err = collector.Start(context.Background())
	require.NoError(t, err)
	require.False(t, collector.Stopped())

	time.Sleep(10 * time.Millisecond)

	collector.Stop()

	time.Sleep(10 * time.Millisecond)
	require.True(t, collector.Stopped())
}

func TestErrorLogsCollector_MetricsIncremented(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 100), func() {})
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

	tests := []struct {
		name           string
		logLine        string
		expectedMetric string
		checkFunc      func(*testing.T, prometheus.Gatherer)
	}{
		{
			name:           "real resource limit error from production",
			logLine:        `{"timestamp":"2025-12-12 15:29:31.529 GMT","user":"conn_limited","dbname":"books_store","pid":9449,"remote_host":"[local]","session_id":"693c34db.24e9","line_num":4,"ps":"startup","session_start":"2025-12-12 15:29:31 GMT","vxid":"91/57","txid":0,"error_severity":"FATAL","state_code":"53300","message":"too many connections for role \"conn_limited\"","backend_type":"client backend","query_id":-6883023751393440299}`,
			expectedMetric: "postgres_errors_by_sqlstate_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_errors_by_sqlstate_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
						metric := mf.GetMetric()[0]
						labels := make(map[string]string)
						for _, label := range metric.GetLabel() {
							labels[label.GetName()] = label.GetValue()
						}
						require.Equal(t, "53", labels["sqlstate_class_code"], "sqlstate_class_code label should be 53")
						require.Equal(t, "53300", labels["sqlstate"], "sqlstate should be 53300")
					}
				}
				require.True(t, found, "errors_by_sqlstate metric should exist")
			},
		},
		{
			name:           "real authentication failure from production",
			logLine:        `{"timestamp":"2025-12-12 15:29:42.201 GMT","user":"app-user","dbname":"books_store","pid":9589,"remote_host":"::1","remote_port":52860,"session_id":"693c34e6.2575","line_num":2,"ps":"authentication","session_start":"2025-12-12 15:29:42 GMT","vxid":"159/363","txid":0,"error_severity":"FATAL","state_code":"28P01","message":"password authentication failed for user \"app-user\"","detail":"Connection matched file \"/etc/postgresql/pg_hba_cluster.conf\" line 4: \"host    all             all             ::1/128                md5\"","backend_type":"client backend","query_id":225649433808025698}`,
			expectedMetric: "postgres_auth_failures_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_auth_failures_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
						metric := mf.GetMetric()[0]
						labels := make(map[string]string)
						for _, label := range metric.GetLabel() {
							labels[label.GetName()] = label.GetValue()
						}
						require.Equal(t, "app-user", labels["user"], "user label should be set")
						// Auth method is extracted from message ("password authentication failed"), not from detail where "md5" appears
						require.Equal(t, "password", labels["auth_method"], "auth method should be extracted from message")
					}
				}
				require.True(t, found, "auth_failures metric should exist")
			},
		},
		{
			name:           "real statement timeout from production",
			logLine:        `{"timestamp":"2025-12-12 15:29:16.068 GMT","user":"app-user","dbname":"books_store","pid":9112,"remote_host":"[local]","session_id":"693c34cb.2398","line_num":4,"ps":"SELECT","session_start":"2025-12-12 15:29:15 GMT","vxid":"25/112","txid":0,"error_severity":"ERROR","state_code":"57014","message":"canceling statement due to statement timeout","statement":"SELECT pg_sleep(5);","application_name":"psql","backend_type":"client backend","query_id":5457019535816659310}`,
			expectedMetric: "postgres_errors_by_sqlstate_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				foundMetric := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_errors_by_sqlstate_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
						for _, metric := range mf.GetMetric() {
							labels := make(map[string]string)
							for _, label := range metric.GetLabel() {
								labels[label.GetName()] = label.GetValue()
							}
							if labels["sqlstate"] == "57014" {
								foundMetric = true
								require.Equal(t, "57", labels["sqlstate_class_code"], "sqlstate_class_code label should be 57")
								break
							}
						}
					}
				}
				require.True(t, found, "errors_by_sqlstate metric should exist")
				require.True(t, foundMetric, "metric with sqlstate=57014 should exist")
			},
		},
		{
			name: "connection error (synthetic - rare in production)",
			logLine: `{"timestamp":"2024-11-28 10:15:30.123 UTC","user":"myuser","dbname":"testdb","pid":12349,` +
				`"error_severity":"FATAL","state_code":"08006","message":"connection failure"}`,
			expectedMetric: "postgres_connection_errors_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_connection_errors_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
					}
				}
				require.True(t, found, "connection_errors metric should exist")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector.Receiver().Chan() <- loki.Entry{
				Entry: push.Entry{
					Line:      tt.logLine,
					Timestamp: time.Now(),
				},
			}

			time.Sleep(100 * time.Millisecond)

			tt.checkFunc(t, registry)
		})
	}
}

func TestErrorLogsCollector_StrconvQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple text",
			expected: `"simple text"`,
		},
		{
			name:     "escaped quotes",
			input:    `value with "quotes"`,
			expected: `"value with \"quotes\""`,
		},
		{
			name:     "escaped backslashes",
			input:    `path\to\file`,
			expected: `"path\\to\\file"`,
		},
		{
			name:     "escaped newlines",
			input:    "line1\nline2",
			expected: `"line1\nline2"`,
		},
		{
			name:     "escaped tabs",
			input:    "column1\tcolumn2",
			expected: `"column1\tcolumn2"`,
		},
		{
			name:     "escaped carriage returns",
			input:    "value\rwith\rcr",
			expected: `"value\rwith\rcr"`,
		},
		{
			name:     "complex SQL with quotes",
			input:    `SQL statement "SELECT 1 FROM ONLY "public"."books" x WHERE "id" OPERATOR(pg_catalog.=) $1 FOR KEY SHARE OF x"`,
			expected: `"SQL statement \"SELECT 1 FROM ONLY \"public\".\"books\" x WHERE \"id\" OPERATOR(pg_catalog.=) $1 FOR KEY SHARE OF x\""`,
		},
		{
			name:     "multiple special characters",
			input:    "line1\nwith \"quotes\"\tand\\backslash",
			expected: `"line1\nwith \"quotes\"\tand\\backslash"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strconv.Quote(tt.input)
			require.Equal(t, tt.expected, result, "strconv.Quote should properly escape special characters")
		})
	}
}

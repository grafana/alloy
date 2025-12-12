package collector

import (
	"context"
	"encoding/json"
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
			name:          "real unique constraint violation from production",
			jsonLog:       `{"timestamp":"2025-12-12 15:29:12.986 GMT","user":"app-user","dbname":"books_store","pid":9050,"remote_host":"[local]","session_id":"693c34c8.235a","line_num":4,"ps":"INSERT","session_start":"2025-12-12 15:29:12 GMT","vxid":"16/116","txid":833,"error_severity":"ERROR","state_code":"23505","message":"duplicate key value violates unique constraint \"uk_books_isbn\"","detail":"Key (isbn)=(9780123456781) already exists.","statement":"INSERT INTO books (title, isbn, publication_date, rental_price_per_day, stock)\n   VALUES ('Chaos Duplicate ISBN', '9780123456781', CURRENT_DATE, 10.00, 5);","application_name":"psql","backend_type":"client backend","query_id":-5382488324425698396}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "23505", p.SQLStateCode)
				require.Equal(t, "23", p.SQLStateClass)
				require.Equal(t, "Integrity Constraint Violation", p.ErrorCategory)
				require.Equal(t, "app-user", p.User)
				require.Equal(t, "books_store", p.DatabaseName)
				require.Equal(t, int32(9050), p.PID)
				require.Equal(t, int64(-5382488324425698396), p.QueryID)
				require.Equal(t, "uk_books_isbn", p.ConstraintName)
				require.Equal(t, "isbn", p.ColumnName)
			},
		},
		{
			name:          "real foreign key violation from production",
			jsonLog:       `{"timestamp":"2025-12-12 15:29:13.288 GMT","user":"app-user","dbname":"books_store","pid":9059,"remote_host":"[local]","session_id":"693c34c9.2363","line_num":4,"ps":"INSERT","session_start":"2025-12-12 15:29:13 GMT","vxid":"19/127","txid":834,"error_severity":"ERROR","state_code":"23503","message":"insert or update on table \"rentals\" violates foreign key constraint \"rentals_book_id_fkey\"","detail":"Key (book_id)=(99999999) is not present in table \"books\".","statement":"INSERT INTO rentals (book_id, customer_name, customer_email, rental_date, expected_return_date, daily_rate, status)\n   VALUES (99999999, 'Chaos User', 'chaos@example.com', CURRENT_TIMESTAMP, CURRENT_DATE + INTERVAL '3 days', 9.99, 'ACTIVE');","application_name":"psql","backend_type":"client backend","query_id":8532599624588683544}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "23503", p.SQLStateCode)
				require.Equal(t, "23", p.SQLStateClass)
				require.Equal(t, "Integrity Constraint Violation", p.ErrorCategory)
				require.Equal(t, "app-user", p.User)
				require.Equal(t, "books_store", p.DatabaseName)
				require.Equal(t, "rentals_book_id_fkey", p.ConstraintName)
				require.Equal(t, "book_id", p.ColumnName)
				require.Equal(t, "books", p.ReferencedTable)
			},
		},
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
				require.Equal(t, int32(9185), p.BlockedPID, "should extract blocked PID")
				require.Equal(t, int32(9184), p.BlockerPID, "should extract blocker PID")
				require.Equal(t, "UPDATE books SET stock = stock WHERE id = 2;", p.BlockedQuery, "should extract blocked query")
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
				Severities:   []string{"ERROR", "FATAL", "PANIC"},
				PassThrough:  false,
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
		Severities:   []string{"ERROR", "FATAL", "PANIC"},
		PassThrough:  false,
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
		Severities:   []string{"ERROR", "FATAL", "PANIC"},
		PassThrough:  false,
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
			name:           "real constraint violation from production",
			logLine:        `{"timestamp":"2025-12-12 15:29:12.986 GMT","user":"app-user","dbname":"books_store","pid":9050,"remote_host":"[local]","session_id":"693c34c8.235a","line_num":4,"ps":"INSERT","session_start":"2025-12-12 15:29:12 GMT","vxid":"16/116","txid":833,"error_severity":"ERROR","state_code":"23505","message":"duplicate key value violates unique constraint \"uk_books_isbn\"","detail":"Key (isbn)=(9780123456781) already exists.","statement":"INSERT INTO books (title, isbn, publication_date, rental_price_per_day, stock)\n   VALUES ('Chaos Duplicate ISBN', '9780123456781', CURRENT_DATE, 10.00, 5);","application_name":"psql","backend_type":"client backend","query_id":-5382488324425698396}`,
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
						require.Equal(t, "23", labels["sqlstate_class_code"], "sqlstate_class_code label should be 23")
						require.Equal(t, "23505", labels["sqlstate"], "sqlstate should be 23505")
					}
				}
				require.True(t, found, "errors_by_sqlstate metric should exist")
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

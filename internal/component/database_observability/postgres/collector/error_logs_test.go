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
	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestErrorLogsCollector_ParseJSON(t *testing.T) {
	tests := []struct {
		name          string
		jsonLog       string
		expectedError bool
		checkFields   func(*testing.T, *ParsedError)
	}{
		{
			name:          "statement timeout (57014)",
			jsonLog:       `{"timestamp":"2025-12-12 15:29:16.068 GMT","user":"app-user","dbname":"books_store","pid":9112,"remote_host":"[local]","session_id":"693c34cb.2398","line_num":4,"ps":"SELECT","session_start":"2025-12-12 15:29:15 GMT","vxid":"25/112","txid":0,"error_severity":"ERROR","state_code":"57014","message":"canceling statement due to statement timeout","statement":"SELECT pg_sleep(5);","application_name":"psql","backend_type":"client backend","query_id":5457019535816659310}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "57014", p.SQLState)
				require.Equal(t, "57", p.SQLStateClass)
				require.Equal(t, "Operator Intervention", p.ErrorCategory)
				require.Equal(t, "query_canceled", p.ErrorName, "should extract error name from SQLSTATE")
			},
		},
		{
			name:          "deadlock detected (40P01)",
			jsonLog:       `{"timestamp":"2025-12-12 15:29:23.258 GMT","user":"app-user","dbname":"books_store","pid":9185,"remote_host":"[local]","session_id":"693c34cf.23e1","line_num":9,"ps":"UPDATE","session_start":"2025-12-12 15:29:19 GMT","vxid":"36/148","txid":837,"error_severity":"ERROR","state_code":"40P01","message":"deadlock detected","detail":"Process 9185 waits for ShareLock on transaction 836; blocked by process 9184.\nProcess 9184 waits for ShareLock on transaction 837; blocked by process 9185.\nProcess 9185: UPDATE books SET stock = stock WHERE id = 2;\nProcess 9184: UPDATE books SET stock = stock WHERE id = 1;","hint":"See server log for query details.","context":"while locking tuple (3,88) in relation \"books\"","statement":"UPDATE books SET stock = stock WHERE id = 2;","application_name":"psql","backend_type":"client backend","query_id":3188095831510673590}`,
			expectedError: false,
			checkFields: func(t *testing.T, p *ParsedError) {
				require.Equal(t, "ERROR", p.ErrorSeverity)
				require.Equal(t, "40P01", p.SQLState)
				require.Equal(t, "40", p.SQLStateClass)
				require.Equal(t, "Transaction Rollback", p.ErrorCategory)
				require.Equal(t, "deadlock_detected", p.ErrorName, "should extract error name from SQLSTATE")
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
				Receiver:              loki.NewLogsReceiver(),
				EntryHandler:          entryHandler,
				Logger:                log.NewNopLogger(),
				InstanceKey:           "test-instance",
				SystemID:              "test-system",
				Registry:              prometheus.NewRegistry(),
				DisableQueryRedaction: true,
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

			if tt.checkFields != nil {
				tt.checkFields(t, parsed)
			}
		})
	}
}

func TestErrorLogsCollector_StartStop(t *testing.T) {
	entryHandler := loki.NewEntryHandler(make(chan loki.Entry, 10), func() {})

	collector, err := NewErrorLogs(ErrorLogsArguments{
		Receiver:              loki.NewLogsReceiver(),
		EntryHandler:          entryHandler,
		Logger:                log.NewNopLogger(),
		InstanceKey:           "test",
		SystemID:              "test",
		Registry:              prometheus.NewRegistry(),
		DisableQueryRedaction: true,
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
		Receiver:              loki.NewLogsReceiver(),
		EntryHandler:          entryHandler,
		Logger:                log.NewNopLogger(),
		InstanceKey:           "test",
		SystemID:              "test",
		Registry:              registry,
		DisableQueryRedaction: true,
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
			name:           "too many connections (53300)",
			logLine:        `{"timestamp":"2025-12-12 15:29:31.529 GMT","user":"conn_limited","dbname":"books_store","pid":9449,"remote_host":"[local]","session_id":"693c34db.24e9","line_num":4,"ps":"startup","session_start":"2025-12-12 15:29:31 GMT","vxid":"91/57","txid":0,"error_severity":"FATAL","state_code":"53300","message":"too many connections for role \"conn_limited\"","backend_type":"client backend","query_id":-6883023751393440299}`,
			expectedMetric: "postgres_errors_by_sqlstate_query_user_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_errors_by_sqlstate_query_user_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
						metric := mf.GetMetric()[0]
						labels := make(map[string]string)
						for _, label := range metric.GetLabel() {
							labels[label.GetName()] = label.GetValue()
						}
						require.Equal(t, "53", labels["sqlstate_class"], "sqlstate_class label should be 53")
						require.Equal(t, "53300", labels["sqlstate"], "sqlstate_code should be 53300")
						require.Equal(t, "too_many_connections", labels["error_name"], "error_name label should be too_many_connections")
						require.Equal(t, "conn_limited", labels["user"], "user label should be conn_limited")
					}
				}
				require.True(t, found, "errors_by_sqlstate metric should exist")
			},
		},
		{
			name:           "authentication failure (28P01)",
			logLine:        `{"timestamp":"2025-12-12 15:29:42.201 GMT","user":"app-user","dbname":"books_store","pid":9589,"remote_host":"::1","remote_port":52860,"session_id":"693c34e6.2575","line_num":2,"ps":"authentication","session_start":"2025-12-12 15:29:42 GMT","vxid":"159/363","txid":0,"error_severity":"FATAL","state_code":"28P01","message":"password authentication failed for user \"app-user\"","detail":"Connection matched file \"/etc/postgresql/pg_hba_cluster.conf\" line 4: \"host    all             all             ::1/128                md5\"","backend_type":"client backend","query_id":225649433808025698}`,
			expectedMetric: "postgres_errors_by_sqlstate_query_user_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_errors_by_sqlstate_query_user_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
						// Find the metric with sqlstate_class="28" (auth errors)
						for _, metric := range mf.GetMetric() {
							labels := make(map[string]string)
							for _, label := range metric.GetLabel() {
								labels[label.GetName()] = label.GetValue()
							}
							if labels["sqlstate_class"] == "28" {
								require.Equal(t, "app-user", labels["user"], "user label should be set")
								require.Equal(t, "invalid_password", labels["error_name"], "error_name should be invalid_password")
								break
							}
						}
					}
				}
				require.True(t, found, "errors_by_sqlstate metric should exist")
			},
		},
		{
			name:           "query canceled with queryid tracking (57014)",
			logLine:        `{"timestamp":"2025-12-12 15:29:16.068 GMT","user":"app-user","dbname":"books_store","pid":9112,"remote_host":"[local]","session_id":"693c34cb.2398","line_num":4,"ps":"SELECT","session_start":"2025-12-12 15:29:15 GMT","vxid":"25/112","txid":0,"error_severity":"ERROR","state_code":"57014","message":"canceling statement due to statement timeout","statement":"SELECT pg_sleep(5);","application_name":"psql","backend_type":"client backend","query_id":5457019535816659310}`,
			expectedMetric: "postgres_errors_by_sqlstate_query_user_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				foundMetric := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_errors_by_sqlstate_query_user_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
						for _, metric := range mf.GetMetric() {
							labels := make(map[string]string)
							for _, label := range metric.GetLabel() {
								labels[label.GetName()] = label.GetValue()
							}
							if labels["sqlstate"] == "57014" {
								foundMetric = true
								require.Equal(t, "57", labels["sqlstate_class"], "sqlstate_class label should be 57")
								require.Equal(t, "query_canceled", labels["error_name"], "error_name label should be query_canceled")
								require.Equal(t, "app-user", labels["user"], "user label should be app-user")
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
			name: "connection failure (08006)",
			logLine: `{"timestamp":"2024-11-28 10:15:30.123 UTC","user":"myuser","dbname":"testdb","pid":12349,` +
				`"error_severity":"FATAL","state_code":"08006","message":"connection failure"}`,
			expectedMetric: "postgres_errors_by_sqlstate_query_user_total",
			checkFunc: func(t *testing.T, g prometheus.Gatherer) {
				mfs, _ := g.Gather()
				found := false
				for _, mf := range mfs {
					if mf.GetName() == "postgres_errors_by_sqlstate_query_user_total" {
						found = true
						require.Greater(t, len(mf.GetMetric()), 0, "should have at least one metric")
						// Find the metric with sqlstate_class="08" (connection errors)
						for _, metric := range mf.GetMetric() {
							labels := make(map[string]string)
							for _, label := range metric.GetLabel() {
								labels[label.GetName()] = label.GetValue()
							}
							if labels["sqlstate_class"] == "08" {
								require.Equal(t, "myuser", labels["user"], "user label should be set")
								require.Equal(t, "connection_failure", labels["error_name"], "error_name should be connection_failure")
								break
							}
						}
					}
				}
				require.True(t, found, "errors_by_sqlstate metric should exist")
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

func TestErrorLogsCollector_StatementRedaction(t *testing.T) {
	tests := []struct {
		name                  string
		jsonLog               string
		disableQueryRedaction bool
		checkStatement        func(*testing.T, string)
	}{
		{
			name:                  "redaction enabled - smart redaction for mixed text",
			jsonLog:               `{"timestamp":"2025-12-12 15:29:16.068 GMT","user":"app-user","dbname":"books_store","pid":9112,"error_severity":"ERROR","state_code":"40P01","message":"deadlock detected","statement":"SELECT * FROM users WHERE id = 123 AND name = 'John'","internal_query":"UPDATE accounts SET balance = 500 WHERE id = 42","detail":"Process 9112 waits for ShareLock; blocked by process 9184.\nProcess 9112: UPDATE books SET stock = 200 WHERE id = 2;\nProcess 9184: DELETE FROM orders WHERE id = 99;","hint":"SELECT * FROM users FOR UPDATE","context":"while executing query: INSERT INTO logs (message) VALUES ('test')"}`,
			disableQueryRedaction: false,
			checkStatement: func(t *testing.T, logLine string) {
				// Statement should be obfuscated - literals replaced with ?
				require.Contains(t, logLine, `statement="SELECT * FROM users WHERE id = ? AND name = ?"`)
				require.NotContains(t, logLine, `statement="SELECT * FROM users WHERE id = 123`)

				// Internal query should be obfuscated
				require.Contains(t, logLine, `internal_query="UPDATE accounts SET balance = ? WHERE id = ?"`)
				require.NotContains(t, logLine, `internal_query="UPDATE accounts SET balance = 500`)

				// Detail: SQL should be redacted but process IDs preserved
				require.Contains(t, logLine, `detail=`)
				require.Contains(t, logLine, `Process 9112`)                            // Process ID preserved
				require.Contains(t, logLine, `Process 9184`)                            // Process ID preserved
				require.Contains(t, logLine, `UPDATE books SET stock = ? WHERE id = ?`) // SQL redacted
				require.Contains(t, logLine, `DELETE FROM orders WHERE id = ?`)         // SQL redacted
				require.NotContains(t, logLine, `stock = 200`)                          // Literal should be gone
				require.NotContains(t, logLine, `id = 99`)                              // Literal should be gone

				// Hint: Pure SQL example, fully redacted
				require.Contains(t, logLine, `hint="SELECT * FROM users FOR UPDATE"`)

				// Context: SQL extracted and redacted, then parentheses redacted
				require.Contains(t, logLine, `context=`)
				require.Contains(t, logLine, `INSERT INTO logs (?) VALUES (?)`) // SQL redacted, then parens redacted
				require.NotContains(t, logLine, `VALUES ('test')`)              // Literal should be gone
			},
		},
		{
			name:                  "redaction disabled - all fields unredacted",
			jsonLog:               `{"timestamp":"2025-12-12 15:29:16.068 GMT","user":"app-user","dbname":"books_store","pid":9112,"error_severity":"ERROR","state_code":"40P01","message":"deadlock detected","statement":"SELECT * FROM users WHERE id = 123 AND name = 'John'","internal_query":"UPDATE accounts SET balance = 500 WHERE id = 42","detail":"Process 9112: UPDATE books SET stock = 200 WHERE id = 2","hint":"SELECT * FROM users FOR UPDATE"}`,
			disableQueryRedaction: true,
			checkStatement: func(t *testing.T, logLine string) {
				// Statement should NOT be obfuscated - original values preserved
				require.Contains(t, logLine, `SELECT * FROM users WHERE id = 123`)
				require.Contains(t, logLine, `name = 'John'`)

				// Internal query should NOT be obfuscated
				require.Contains(t, logLine, `UPDATE accounts SET balance = 500 WHERE id = 42`)

				// Detail should preserve original SQL
				require.Contains(t, logLine, `Process 9112`)
				require.Contains(t, logLine, `UPDATE books SET stock = 200 WHERE id = 2`)

				// Hint should preserve original
				require.Contains(t, logLine, `SELECT * FROM users FOR UPDATE`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entryChan := make(chan loki.Entry, 10)
			entryHandler := loki.NewEntryHandler(entryChan, func() {})

			collector, err := NewErrorLogs(ErrorLogsArguments{
				Receiver:              loki.NewLogsReceiver(),
				EntryHandler:          entryHandler,
				Logger:                log.NewNopLogger(),
				InstanceKey:           "test",
				SystemID:              "test",
				Registry:              prometheus.NewRegistry(),
				DisableQueryRedaction: tt.disableQueryRedaction,
			})
			require.NoError(t, err)

			err = collector.Start(context.Background())
			require.NoError(t, err)
			defer collector.Stop()

			collector.Receiver().Chan() <- loki.Entry{
				Entry: push.Entry{
					Line:      tt.jsonLog,
					Timestamp: time.Now(),
				},
			}

			select {
			case entry := <-entryChan:
				tt.checkStatement(t, entry.Line)
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for log entry")
			}
		})
	}
}

func TestRedactParenthesizedValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no parentheses",
			input:    "Process 9112 waits for ShareLock; blocked by process 9184.",
			expected: "Process 9112 waits for ShareLock; blocked by process 9184.",
		},
		{
			name:     "PostgreSQL constraint error - Failing row contains",
			input:    "Failing row contains (k6-books, t, -1.000, 2026-01-06 21:44:44.431006, 7033, 2026-01-06 19:33:48.988596, 2026-01-06 21:44:45.123756).",
			expected: "Failing row contains (?).",
		},
		{
			name:     "PostgreSQL foreign key error - Key not present",
			input:    "Key (book_id)=(99999999) is not present in table \"books\".",
			expected: "Key (book_id)=(?) is not present in table \"books\".",
		},
		{
			name:     "PostgreSQL unique constraint error - Key already exists",
			input:    "Key (isbn)=(9780123456781) already exists.",
			expected: "Key (isbn)=(?) already exists.",
		},
		{
			name:     "Multiple Key patterns in one message",
			input:    "Key (book_id)=(99999999) is not present in table \"books\". Key (isbn)=(9780123456781) already exists.",
			expected: "Key (book_id)=(?) is not present in table \"books\". Key (isbn)=(?) already exists.",
		},
		{
			name:     "Composite key with multiple columns",
			input:    "Key (user_id, order_id)=(12345, 67890) already exists.",
			expected: "Key (user_id, order_id)=(?) already exists.",
		},
		{
			name:     "Failing row with mixed data types",
			input:    "Failing row contains (john@example.com, John Doe, 2025-01-01, true, 123.45).",
			expected: "Failing row contains (?).",
		},
		{
			name:     "Empty parentheses",
			input:    "Empty call: func().",
			expected: "Empty call: func(?).",
		},
		{
			name:     "Unmatched parentheses - returns original",
			input:    "Incomplete (value without closing",
			expected: "Incomplete (value without closing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := database_observability.RedactParenthesizedValues(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestRedactCombined(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PostgreSQL error with both parenthesized values and SQL",
			input:    "Key (book_id)=(99999999) is not present in table \"books\". Process 9112: UPDATE books SET stock = 200 WHERE id = 2;",
			expected: "Key (book_id)=(?) is not present in table \"books\". Process 9112: UPDATE books SET stock = ? WHERE id = ?;",
		},
		{
			name:     "Failing row with SQL in context",
			input:    "Failing row contains (john@example.com, John Doe, 2025-01-01, true, 123.45). Query: INSERT INTO users VALUES ('test', 123)",
			expected: "Failing row contains (?). Query: INSERT INTO users VALUES (?)",
		},
		{
			name:     "Multiple errors with mixed patterns",
			input:    "Key (isbn)=(9780123456781) already exists. Detail: SELECT * FROM books WHERE isbn = '9780123456781'",
			expected: "Key (isbn)=(?) already exists. Detail: SELECT * FROM books WHERE isbn = ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply both redaction functions in sequence, as done in emitToLoki
			// SQL redaction first, then parenthesized values
			result := database_observability.RedactSQLWithinMixedText(tt.input)
			result = database_observability.RedactParenthesizedValues(result)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestRedactMixedTextWithSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure text without SQL",
			input:    "Process 9112 waits for ShareLock; blocked by process 9184.",
			expected: "Process 9112 waits for ShareLock; blocked by process 9184.",
		},
		{
			name:     "text with single UPDATE statement",
			input:    "Process 9112: UPDATE books SET stock = 200 WHERE id = 2;",
			expected: "Process 9112: UPDATE books SET stock = ? WHERE id = ?;",
		},
		{
			name:     "text with multiple SQL statements",
			input:    "Process 9112: UPDATE books SET stock = 200 WHERE id = 2; Process 9184: DELETE FROM orders WHERE id = 99;",
			expected: "Process 9112: UPDATE books SET stock = ? WHERE id = ?; Process 9184: DELETE FROM orders WHERE id = ?;",
		},
		{
			name:     "deadlock detail with complex SQL",
			input:    "Process 9185 waits for ShareLock on transaction 836; blocked by process 9184.\nProcess 9185: UPDATE books SET stock = stock WHERE id = 2;\nProcess 9184: UPDATE books SET stock = stock WHERE id = 1;",
			expected: "Process 9185 waits for ShareLock on transaction 836; blocked by process 9184.\nProcess 9185: UPDATE books SET stock = stock WHERE id = ?;\nProcess 9184: UPDATE books SET stock = stock WHERE id = ?;",
		},
		{
			name:     "context with INSERT statement",
			input:    "while executing query: INSERT INTO logs (message, level) VALUES ('error occurred', 3)",
			expected: "while executing query: INSERT INTO logs (message, level) VALUES (?, ?)",
		},
		{
			name:     "SELECT statement in descriptive text",
			input:    "Error during: SELECT * FROM users WHERE age > 25 AND status = 'active'",
			expected: "Error during: SELECT * FROM users WHERE age > ? AND status = ?",
		},
		{
			name:     "multiple keywords with mixed case",
			input:    "Failed: select count(*) from orders where total > 100; insert into audit values (123)",
			expected: "Failed: select count(*) from orders where total > ?; insert into audit values (?)",
		},
		{
			name:     "administrative commands not redacted (no PII)",
			input:    "Error during: VACUUM ANALYZE users; REINDEX TABLE orders; CHECKPOINT;",
			expected: "Error during: VACUUM ANALYZE users; REINDEX TABLE orders; CHECKPOINT;",
		},
		{
			name:     "transaction control not redacted (no data)",
			input:    "Error during: BEGIN TRANSACTION ISOLATION LEVEL SERIALIZABLE; UPDATE accounts SET balance = 1000 WHERE id = 5; COMMIT;",
			expected: "Error during: BEGIN TRANSACTION ISOLATION LEVEL SERIALIZABLE; UPDATE accounts SET balance = ? WHERE id = ?; COMMIT;",
		},
		{
			name:     "DDL statements not redacted (no PII in schema)",
			input:    "Failed: CREATE TABLE users (id INT, name VARCHAR(50)); ALTER TABLE users ADD COLUMN email VARCHAR(100);",
			expected: "Failed: CREATE TABLE users (id INT, name VARCHAR(50)); ALTER TABLE users ADD COLUMN email VARCHAR(100);",
		},
		{
			name:     "prepared statements with data - redacted",
			input:    "Error in: PREPARE stmt AS SELECT * FROM users WHERE id = $1 AND name = 'John'",
			expected: "Error in: PREPARE stmt AS SELECT * FROM users WHERE id = $1 AND name = ?",
		},
		{
			name:     "COPY command with data paths - redacted",
			input:    "Failed: COPY users (name, email) FROM '/tmp/data.csv' WITH (FORMAT csv, DELIMITER ',');",
			expected: "Failed: COPY users (name, email) FROM ? WITH (FORMAT csv, DELIMITER ?);",
		},
		{
			name:     "WITH CTE with data - redacted",
			input:    "Error in: WITH active_users AS (SELECT * FROM users WHERE status = 'active') SELECT * FROM active_users",
			expected: "Error in: WITH active_users AS (SELECT * FROM users WHERE status = ?) SELECT * FROM active_users",
		},
		{
			name:     "MERGE statement with PII - redacted",
			input:    "Failed: MERGE INTO users USING new_users ON users.id = new_users.id WHEN MATCHED THEN UPDATE SET name = 'updated'",
			expected: "Failed: MERGE INTO users USING new_users ON users.id = new_users.id WHEN MATCHED THEN UPDATE SET name = ?",
		},
		{
			name:     "user/role creation with credentials - redacted",
			input:    "Error: CREATE USER john WITH PASSWORD 'secret123'; GRANT ALL ON database mydb TO john;",
			expected: "Error: CREATE USER john WITH PASSWORD ?; GRANT ALL ON database mydb TO john;",
		},
		{
			name:     "SET with sensitive config - redacted",
			input:    "Error during: SET search_path TO public, private; UPDATE users SET email = 'test@example.com';",
			expected: "Error during: SET search_path TO public, private; UPDATE users SET email = ?;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := database_observability.RedactSQLWithinMixedText(tt.input)
			require.Equal(t, tt.expected, result)
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

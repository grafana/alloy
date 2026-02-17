package database_observability

import (
	"testing"

	"github.com/DataDog/go-sqllexer"
	"github.com/stretchr/testify/require"
)

func TestPgSqlParser_Redact(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		want    string
		wantErr bool
	}{
		{
			name: "simple select with literals",
			sql:  "SELECT * FROM users WHERE id = 123 AND name = 'john'",
			want: "SELECT * FROM users WHERE id = ? AND name = ?",
		},
		{
			name: "insert with multiple values",
			sql:  "INSERT INTO users (name, age) VALUES ('john', 30), ('jane', 25)",
			want: "INSERT INTO users (name, age) VALUES (?, ?), (?, ?)",
		},
		{
			name: "update with where clause",
			sql:  "UPDATE users SET last_login = '2024-03-20 10:00:00' WHERE id = 456",
			want: "UPDATE users SET last_login = ? WHERE id = ?",
		},
		{
			name: "delete with complex condition",
			sql:  "DELETE FROM users WHERE age > 50 AND last_login < '2023-01-01'",
			want: "DELETE FROM users WHERE age > ? AND last_login < ?",
		},
		{
			name: "select with subquery",
			sql:  "SELECT * FROM orders WHERE user_id IN (SELECT id FROM users WHERE age > 21)",
			want: "SELECT * FROM orders WHERE user_id IN (SELECT id FROM users WHERE age > ?)",
		},
		{
			name: "simple WITH statement",
			sql:  "WITH active_users AS (SELECT * FROM users WHERE last_login > '2024-01-01') SELECT * FROM active_users WHERE age > 25",
			want: "WITH active_users AS (SELECT * FROM users WHERE last_login > ?) SELECT * FROM active_users WHERE age > ?",
		},
		{
			name: "complex WITH statement with multiple CTEs",
			sql: `WITH active_users AS (
					SELECT * FROM users WHERE last_login > '2024-01-01'
				), recent_orders AS (
					SELECT o.* FROM orders o
					JOIN active_users u ON u.id = o.user_id
					WHERE o.created_at > '2024-03-01'
				)
				SELECT au.name, COUNT(ro.id) as order_count
				FROM active_users au
				LEFT JOIN recent_orders ro ON ro.user_id = au.id
				GROUP BY au.name
				HAVING COUNT(ro.id) > 5`,
			want: `WITH active_users AS (
					SELECT * FROM users WHERE last_login > ?
				), recent_orders AS (
					SELECT o.* FROM orders o
					JOIN active_users u ON u.id = o.user_id
					WHERE o.created_at > ?
				)
				SELECT au.name, COUNT(ro.id) as order_count
				FROM active_users au
				LEFT JOIN recent_orders ro ON ro.user_id = au.id
				GROUP BY au.name
				HAVING COUNT(ro.id) > ?`,
		},
		{
			name: "WITH RECURSIVE statement",
			sql: `WITH RECURSIVE subordinates AS (
					SELECT * FROM employees WHERE manager_id = 123
					UNION ALL
					SELECT e.* FROM employees e
					INNER JOIN subordinates s ON s.id = e.manager_id
				)
				SELECT * FROM subordinates`,
			want: `WITH RECURSIVE subordinates AS (
					SELECT * FROM employees WHERE manager_id = ?
					UNION ALL
					SELECT e.* FROM employees e
					INNER JOIN subordinates s ON s.id = e.manager_id
				)
				SELECT * FROM subordinates`,
		},
		{
			name: "WITH statement with INSERT",
			sql: `WITH new_users AS (
					SELECT generate_series(1, 3) as id, 'user_' || generate_series(1, 3) as name
				)
				INSERT INTO users (id, name, created_at)
				SELECT id, name, '2024-03-20'::timestamp
				FROM new_users`,
			want: `WITH new_users AS (
					SELECT generate_series(?, ?) as id, ? || generate_series(?, ?) as name
				)
				INSERT INTO users (id, name, created_at)
				SELECT id, name, ?::timestamp
				FROM new_users`,
		},
		{
			name: "WITH statement with UPDATE",
			sql: `WITH inactive_users AS (
					SELECT id FROM users
					WHERE last_login < '2023-01-01' AND status = 'active'
				)
				UPDATE users SET status = 'inactive', updated_at = '2024-03-20'
				WHERE id IN (SELECT id FROM inactive_users)`,
			want: `WITH inactive_users AS (
					SELECT id FROM users
					WHERE last_login < ? AND status = ?
				)
				UPDATE users SET status = ?, updated_at = ?
				WHERE id IN (SELECT id FROM inactive_users)`,
		},
		{
			name: "WITH statement with DELETE",
			sql: `WITH old_orders AS (
					SELECT id FROM orders
					WHERE created_at < '2023-01-01' AND status = 'completed'
				)
				DELETE FROM order_items
				WHERE order_id IN (SELECT id FROM old_orders)`,
			want: `WITH old_orders AS (
					SELECT id FROM orders
					WHERE created_at < ? AND status = ?
				)
				DELETE FROM order_items
				WHERE order_id IN (SELECT id FROM old_orders)`,
		},
		{
			name: "IN clause with ANY array",
			sql:  "SELECT * FROM users WHERE id = ANY(ARRAY[1, 2, 3])",
			want: "SELECT * FROM users WHERE id = ANY(ARRAY[?, ?, ?])",
		},
		{
			name: "function call with variadic arguments",
			sql:  "SELECT concat_ws(',', VARIADIC ARRAY['a', 'b', 'c'])",
			want: "SELECT concat_ws(?, VARIADIC ARRAY[?, ?, ?])",
		},
		{
			name: "auth statement with password",
			sql:  "ALTER USER myuser WITH PASSWORD 'secret123'",
			want: "ALTER USER myuser WITH PASSWORD ?",
		},
		{
			name: "truncated query without comment",
			sql:  "SELECT * FROM users WHERE id = 123 AND name = 'john' AND ...",
			want: "SELECT * FROM users WHERE id = ? AND name = ? AND ...",
		},
		{
			name: "truncated query with complete comment",
			sql:  "SELECT * FROM users WHERE id = 123 /* some comment */ AND ...",
			want: "SELECT * FROM users WHERE id = ? /* some comment */ AND ...",
		},
		{
			name: "truncated query with incomplete comment",
			sql:  "SELECT * FROM users WHERE id = 123 /* some comment that gets truncated ...",
			want: "SELECT * FROM users WHERE id = ? /* some comment that gets truncated ...",
		},
		{
			name: "type cast",
			sql:  "SELECT id, name, '2024-03-20'::timestamp FROM users",
			want: "SELECT id, name, ?::timestamp FROM users",
		},
		{
			name: "table wildcard",
			sql:  "SELECT u.* FROM users u",
			want: "SELECT u.* FROM users u",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSql(tt.sql)
			if got != tt.want {
				t.Errorf("\nRedact()\nGOT:\n%s\nWANT:\n%s", got, tt.want)
			}
		})
	}
}

func TestContainsReservedKeywords(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		reservedWords map[string]ExplainReservedWordMetadata
		expected      bool
	}{
		{
			name:          "Simple single keyword at the beginning of the query",
			query:         "INSERT INTO users (name) VALUES ('John')",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with INSERT in string literal",
			query:         "SELECT 'INSERT INTO table' FROM users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SELECT with insert in column name",
			query:         "SELECT insert_date FROM users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SELECT with INSERT in comment",
			query:         "SELECT * FROM users -- INSERT comment",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SELECT with INSERT in block comment",
			query:         "SELECT * FROM users /* INSERT block comment */",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "CREATE TABLE statement",
			query:         "CREATE TABLE users (id INT, name VARCHAR(50))",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with CREATE in quoted identifier",
			query:         `SELECT "create_date" FROM users`,
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "UPDATE statement",
			query:         "UPDATE users SET name = 'John' WHERE id = 1",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with update in string",
			query:         "SELECT * FROM users WHERE status = 'update_pending'",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "DELETE statement",
			query:         "DELETE FROM users WHERE id = 1",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with delete in table name",
			query:         "SELECT * FROM delete_log",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "plain SELECT statement",
			query:         "SELECT * FROM users WHERE name = 'John'",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "complex SELECT with joins",
			query:         "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SELECT with reserved word in WHERE clause string",
			query:         "SELECT * FROM users WHERE description LIKE '%CREATE%'",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "DROP statement",
			query:         "DROP TABLE users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with drop in column alias",
			query:         "SELECT name AS drop_reason FROM users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "EXPLAIN statement should be detected",
			query:         "EXPLAIN SELECT * FROM users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with explain in string should not be detected",
			query:         "SELECT 'explain this' FROM users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "PREPARE statement should be detected",
			query:         "PREPARE stmt AS SELECT * FROM users WHERE id = $1",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with prepare in column name should not be detected",
			query:         "SELECT prepare_date FROM users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SET statement should be detected",
			query:         "SET search_path TO public",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "SELECT with set in string should not be detected",
			query:         "SELECT 'set this value' FROM users",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SELECT with reserved word in quoted identifier",
			query:         `SELECT "insert" FROM users`,
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SELECT with reserved word in table alias",
			query:         "SELECT * FROM users AS insert_table",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "SELECT with legacy LOCK IN SHARE MODE",
			query:         "SELECT name FROM users LOCK IN SHARE MODE",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "WITH statement that INSERTS",
			query:         "WITH cte AS (SELECT * FROM users) INSERT into users (name) VALUES ('John')",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "WITH statement that only SELECTs",
			query:         "WITH cte AS (SELECT * FROM users) SELECT * FROM cte",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		{
			name:          "FOR UPDATE precedes UPDATE",
			query:         "SELECT * FROM users FOR UPDATE UPDATE users SET name = 'John'",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		{
			name:          "START TRANSACTION should be detected",
			query:         "START TRANSACTION",
			reservedWords: ExplainReservedWordDenyList,
			expected:      true,
		},
		// Single prefix with default reserved word list
		{
			name:          "SELECT with FOR UPDATE",
			query:         "SELECT name FROM users FOR UPDATE",
			reservedWords: ExplainReservedWordDenyList,
			expected:      false,
		},
		// Multiple prefix
		{
			name:  "SELECT with multiple prefixes",
			query: "SELECT name FROM users LOCK IN SHARE MODE",
			reservedWords: map[string]ExplainReservedWordMetadata{
				"MODE": {
					ExemptionPrefixes: &[]string{"SHARE", "IN", "LOCK"},
				},
			},
			expected: false,
		},
		{
			name:  "multiple prefixes in deny list insufficient prefixes in query",
			query: "SELECT name FROM users LOCK IN SHARE MODE",
			reservedWords: map[string]ExplainReservedWordMetadata{
				"MODE": {
					ExemptionPrefixes: &[]string{"SHARE", "IN", "LOCK", "EXTRA"},
				},
			},
			expected: true,
		},
		// This is here to demonstrate a blindspot of the current implementation
		// If the query contains the key word, with all of the defined prefixes,
		// but also includes other keywords/commands/identifiers before the keyword
		// in the denylist, we have to assume it's a new command/keyword/identifier.

		// This means it is VERY important to create complete denylist exemption lists when appropriate.
		{
			name:  "one prefix in deny list additional prefixes in query",
			query: "SELECT name FROM users LOCK IN SHARE MODE",
			reservedWords: map[string]ExplainReservedWordMetadata{
				"MODE": {
					ExemptionPrefixes: &[]string{"SHARE"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run("MySQL: "+tt.name, func(t *testing.T) {
			result, err := ContainsReservedKeywords(tt.query, tt.reservedWords, sqllexer.DBMSMySQL)
			require.NoError(t, err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for query: %s", tt.expected, result, tt.query)
			}
		})
		t.Run("PostgreSQL: "+tt.name, func(t *testing.T) {
			result, err := ContainsReservedKeywords(tt.query, tt.reservedWords, sqllexer.DBMSPostgres)
			require.NoError(t, err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for query: %s", tt.expected, result, tt.query)
			}
		})
	}

	t.Run("lexer error", func(t *testing.T) {
		// TODO - consider if lexer should still error on "foo, see https://github.com/DataDog/go-sqllexer/pull/72
		_, err := ContainsReservedKeywords("SELECT `foo", ExplainReservedWordDenyList, sqllexer.DBMSMySQL)
		require.Error(t, err)
	})
}

package collector

import (
	"testing"
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
			got := redact(tt.sql)
			if got != tt.want {
				t.Errorf("\nRedact()\nGOT:\n%s\nWANT:\n%s", got, tt.want)
			}
		})
	}
}

func TestPgSqlParser_ExtractTableNames(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		want    []string
		wantErr bool
	}{
		{
			name: "simple select",
			sql:  "SELECT * FROM users",
			want: []string{"users"},
		},
		{
			name: "select with join",
			sql:  "SELECT * FROM users u JOIN orders o ON u.id = o.user_id",
			want: []string{"orders", "users"},
		},
		{
			name: "select with schema qualified tables",
			sql:  "SELECT * FROM public.users JOIN sales.orders ON users.id = orders.user_id",
			want: []string{"public.users", "sales.orders"},
		},
		{
			name: "insert statement",
			sql:  "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
			want: []string{"users"},
		},
		{
			name: "update statement",
			sql:  "UPDATE users SET last_login = NOW() WHERE id = 1",
			want: []string{"users"},
		},
		{
			name: "delete statement",
			sql:  "DELETE FROM users WHERE id = 1",
			want: []string{"users"},
		},
		{
			name: "with clause",
			sql: `WITH active_users AS (
				SELECT * FROM users WHERE status = 'active'
			)
			SELECT * FROM active_users au
			JOIN orders o ON o.user_id = au.id`,
			want: []string{"orders", "users"},
		},
		{
			name: "subquery in where clause",
			sql: `SELECT * FROM orders 
				WHERE user_id IN (SELECT id FROM users WHERE status = 'active')`,
			want: []string{"orders", "users"},
		},
		{
			name: "multiple schema qualified tables with aliases",
			sql: `SELECT u.name, o.total, p.status 
				FROM public.users u 
				JOIN sales.orders o ON u.id = o.user_id
				LEFT JOIN shipping.packages p ON o.id = p.order_id`,
			want: []string{"public.users", "sales.orders", "shipping.packages"},
		},
		{
			name: "truncated query with ...",
			sql:  "SELECT * FROM users JOIN orders ON users.id = orders.user_id AND...",
			want: []string{"users", "orders"},
		},
		{
			name: "truncated query with incomplete comment",
			sql:  "SELECT * FROM users JOIN orders ON users.id = orders.user_id /* some comment that gets truncated...",
			want: []string{"users", "orders"},
		},
		{
			name: "truncated query mid-table name",
			sql:  "SELECT * FROM users JOIN ord...",
			want: []string{"users", "ord..."},
		},
		{
			name: "truncated query with schema qualified tables",
			sql:  "SELECT * FROM public.users JOIN sales.orders ON users.id = orders.user_id AND...",
			want: []string{"public.users", "sales.orders"},
		},
		{
			name: "query with table.* expression",
			sql:  "SELECT u.*, o.* FROM users u JOIN orders o ON u.id = o.user_id",
			want: []string{"users", "orders"},
		},
		{
			name: "query with type cast",
			sql:  "SELECT u.id, '2024-03-20'::timestamp FROM users u",
			want: []string{"users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractTableNames(tt.sql)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractTableNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ExtractTableNames()\nGOT = %v\nWANT = %v", got, tt.want)
					return
				}
				// Compare slices ignoring order since table names might come in different order
				gotMap := make(map[string]bool)
				wantMap := make(map[string]bool)
				for _, table := range got {
					gotMap[table] = true
				}
				for _, table := range tt.want {
					wantMap[table] = true
				}
				for table := range gotMap {
					if !wantMap[table] {
						t.Errorf("ExtractTableNames() got unexpected table = %v", table)
					}
				}
				for table := range wantMap {
					if !gotMap[table] {
						t.Errorf("ExtractTableNames() missing expected table = %v", table)
					}
				}
			}
		})
	}
}

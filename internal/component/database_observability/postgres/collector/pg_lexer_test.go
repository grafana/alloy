package collector

import (
	"testing"
)

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
			got, err := ExtractTableNames(tt.sql)
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

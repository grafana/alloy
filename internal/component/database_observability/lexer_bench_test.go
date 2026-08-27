package database_observability

import (
	"strings"
	"testing"

	"github.com/DataDog/go-sqllexer"
)

// multiPrefixDenyList exercises the multi-token exemption lookback (LOCK IN
// SHARE MODE) with a custom list. Used to benchmark the deeper prefix walk.
var multiPrefixDenyList = map[string]ExplainReservedWordMetadata{
	"MODE": {
		ExemptionPrefixes: &[]string{"SHARE", "IN", "LOCK"},
	},
}

var benchInputs = []struct {
	name          string
	reservedWords map[string]ExplainReservedWordMetadata
	query         string
}{
	{
		name:          "select",
		reservedWords: ExplainReservedWordDenyList,
		query:         "SELECT id FROM users WHERE id = 1",
	},
	{
		name:          "select_with_joins",
		reservedWords: ExplainReservedWordDenyList,
		query:         "SELECT u.id, u.name, p.title, c.body FROM users u JOIN posts p ON u.id = p.user_id LEFT JOIN comments c ON c.post_id = p.id WHERE u.active = true AND p.created_at > '2024-01-01' ORDER BY p.created_at DESC LIMIT 100",
	},
	{
		name:          "long_cte",
		reservedWords: ExplainReservedWordDenyList,
		query: strings.Repeat(`WITH active_users AS (
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
				HAVING COUNT(ro.id) > 5
				UNION ALL
				`, 20) + "SELECT 1",
	},
	{
		name:          "insert",
		reservedWords: ExplainReservedWordDenyList,
		query:         "INSERT INTO users (name, age) VALUES ('john', 30)",
	},
	{
		name:          "for_update_exemption",
		reservedWords: ExplainReservedWordDenyList,
		query:         "SELECT name FROM users WHERE id = 1 FOR UPDATE",
	},
	{
		name:          "for_update_exemption_deep_lookback",
		reservedWords: ExplainReservedWordDenyList,
		query:         "SELECT u.id, u.name, u.email, u.created_at, p.title, p.body, c.id, c.body, a.street, a.city FROM users u JOIN posts p ON u.id = p.user_id JOIN comments c ON c.post_id = p.id JOIN addresses a ON a.user_id = u.id WHERE u.active = true AND p.created_at > '2024-01-01' ORDER BY p.created_at DESC LIMIT 100 FOR UPDATE",
	},
	{
		name:          "update_concatenated_no_exemption",
		reservedWords: ExplainReservedWordDenyList,
		query:         "SELECT u.id, u.name, u.email, p.title, c.body FROM users u JOIN posts p ON u.id = p.user_id JOIN comments c ON c.post_id = p.id WHERE u.active = true ORDER BY p.created_at DESC; UPDATE users SET name = 'john' WHERE id = 1",
	},
	{
		name:          "repeated_for_update",
		reservedWords: ExplainReservedWordDenyList,
		query:         strings.Repeat("SELECT id FROM users WHERE id = 1 FOR UPDATE; ", 20) + "SELECT 1 FOR UPDATE",
	},
	{
		name:          "multi_prefix_lock",
		reservedWords: multiPrefixDenyList,
		query:         "SELECT name FROM users WHERE id = 1 LOCK IN SHARE MODE",
	},
	{
		name:          "multi_prefix_lock_deep_lookback",
		reservedWords: multiPrefixDenyList,
		query:         "SELECT u.id, u.name, u.email, u.created_at, p.title, p.body, c.id, c.body FROM users u JOIN posts p ON u.id = p.user_id JOIN comments c ON c.post_id = p.id WHERE u.active = true ORDER BY p.created_at DESC LIMIT 100 LOCK IN SHARE MODE",
	},
	{
		name:          "multi_prefix_incomplete",
		reservedWords: multiPrefixDenyList,
		query:         "SELECT name FROM users WHERE id = 1 LOCK SHARE MODE",
	},
}

var benchDBMS = []struct {
	name string
	dbms sqllexer.DBMSType
}{
	{name: "postgres", dbms: sqllexer.DBMSPostgres},
	{name: "mysql", dbms: sqllexer.DBMSMySQL},
}

func BenchmarkContainsReservedKeywords(b *testing.B) {
	for _, dbms := range benchDBMS {
		for _, in := range benchInputs {
			b.Run(dbms.name+"/"+in.name, func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := ContainsReservedKeywords(in.query, in.reservedWords, dbms.dbms); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

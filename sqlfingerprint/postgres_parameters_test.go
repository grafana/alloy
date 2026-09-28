package sqlfingerprint_test

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestPostgresJavaParameters matches JDBC question-mark placeholders to native
// PostgreSQL parameters without requiring an obfuscated trailing comment.
func TestPostgresJavaParameters(t *testing.T) {
	spanQuery := `select r1_0.id,r1_0.address,r1_0.created_at,r1_0.description,r1_0.food_type,r1_0.name,r1_0.neighborhood,r1_0.price_range,r1_0.updated_at from restaurants r1_0 where ?=? order by r1_0.name`
	databaseQuery := `select r1_0.id,r1_0.address,r1_0.created_at,r1_0.description,r1_0.food_type,r1_0.name,r1_0.neighborhood,r1_0.price_range,r1_0.updated_at from restaurants r1_0 where $1=$2 order by r1_0.name`
	for _, version := range []int{16, 17, 18} {
		t.Run(fmt.Sprintf("postgres%d", version), func(t *testing.T) {
			f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
			want := one(t, f, sqlfingerprint.PostgreSQL, databaseQuery)
			for _, suffix := range []string{"", ";", " /*application='TapasFinder'*/", " ?"} {
				if got := one(t, f, sqlfingerprint.PostgreSQL, spanQuery+suffix); got != want {
					t.Errorf("Java span with suffix %q and pg_stat_statements differ:\ngot  %s\nwant %s", suffix, got, want)
				}
			}
		})
	}
}

// TestPostgresParameterContexts exercises operand positions without a comment
// suffix and keeps JSON operators distinct from adjacent parameter placeholders.
func TestPostgresParameterContexts(t *testing.T) {
	for _, version := range []int{16, 17, 18} {
		t.Run(fmt.Sprintf("postgres%d", version), func(t *testing.T) {
			f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
			for _, pair := range []struct{ query, normalized string }{
				{`SELECT * FROM restaurants WHERE ? = ? ORDER BY name`, `SELECT * FROM restaurants WHERE $1 = $2 ORDER BY name`},
				{`SELECT ?, COALESCE(?, name) FROM restaurants`, `SELECT $1, COALESCE($2, name) FROM restaurants`},
				{`SELECT * FROM restaurants WHERE id IN (?, ?)`, `SELECT * FROM restaurants WHERE id IN ($1, $2)`},
				{`SELECT ?::integer FROM restaurants`, `SELECT $1::integer FROM restaurants`},
				{`SELECT data ? ? FROM reviews`, `SELECT data ? $1 FROM reviews`},
				{`SELECT ? ? ? FROM reviews`, `SELECT $1 ? $2 FROM reviews`},
				{`SELECT data ?| ARRAY[?] FROM reviews`, `SELECT data ?| ARRAY[$1] FROM reviews`},
				{`SELECT data ?& ARRAY[?] FROM reviews`, `SELECT data ?& ARRAY[$1] FROM reviews`},
				{`SELECT "?", '?' FROM reviews WHERE id=? ORDER BY id`, `SELECT "?", $1 FROM reviews WHERE id=$2 ORDER BY id`},
			} {
				if one(t, f, sqlfingerprint.PostgreSQL, pair.query) != one(t, f, sqlfingerprint.PostgreSQL, pair.normalized) {
					t.Errorf("parameter normalization differs for %q", pair.query)
				}
			}
			jsonOperator := one(t, f, sqlfingerprint.PostgreSQL, `SELECT data ? ? FROM reviews`)
			equality := one(t, f, sqlfingerprint.PostgreSQL, `SELECT data = $1 FROM reviews`)
			if jsonOperator == equality {
				t.Fatal("JSON existence and equality operators must remain distinct")
			}
		})
	}
}

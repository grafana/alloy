package sqlfingerprint_test

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestPostgresRubyOTelOrderComment matches an obfuscated trailing Rails comment
// after ORDER BY, including both directions and PostgreSQL's NULLS modifiers.
func TestPostgresRubyOTelOrderComment(t *testing.T) {
	query := `SELECT "restaurants".* FROM "restaurants" ORDER BY "restaurants"."name"`
	comment := `/*action='index',application='TapasFinder',controller='restaurants'*/`
	for _, version := range []int{16, 17, 18} {
		for _, order := range []string{"ASC", "desc", "ASC NULLS LAST", "DESC NULLS FIRST", "NULLS FIRST", "nulls last"} {
			t.Run(fmt.Sprintf("postgres%d/%s", version, order), func(t *testing.T) {
				f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
				statement := query + " " + order
				want := one(t, f, sqlfingerprint.PostgreSQL, statement+" "+comment)
				for _, suffix := range []string{" ?", " ?;"} {
					if got := one(t, f, sqlfingerprint.PostgreSQL, statement+suffix); got != want {
						t.Fatalf("Ruby span and pg_stat_statements differ:\n%s\n%s", got, want)
					}
				}
			})
		}
	}
}

// TestPostgresRubyOTelTrailingComment covers comments after different clauses
// and values, so the compatibility rule cannot regress to an ASC-only special case.
func TestPostgresRubyOTelTrailingComment(t *testing.T) {
	pairs := []struct{ span, database string }{
		{`SELECT * FROM restaurants ?`, `SELECT * FROM restaurants`},
		{`SELECT data ?`, `SELECT data`},
		{`SELECT * FROM restaurants ORDER BY name ?`, `SELECT * FROM restaurants ORDER BY name`},
		{`SELECT * FROM restaurants ORDER BY "ASC" ?`, `SELECT * FROM restaurants ORDER BY "ASC"`},
		{`SELECT * FROM restaurants WHERE id = $1 ?`, `SELECT * FROM restaurants WHERE id = $1`},
		{`SELECT * FROM restaurants LIMIT 10 ?`, `SELECT * FROM restaurants LIMIT $1`},
		{`SELECT COUNT(*) FROM restaurants ?`, `SELECT COUNT(*) FROM restaurants`},
		{`SELECT ? ?`, `SELECT $1`},
		{`SELECT * FROM restaurants WHERE id = ? AND name = ? ?`, `SELECT * FROM restaurants WHERE id = $1 AND name = $2`},
		{`SELECT data ? ? ?`, `SELECT data ? $1`},
		{`SELECT data ?| ARRAY[?] ?`, `SELECT data ?| ARRAY[$1]`},
		{`SELECT data ?& ARRAY[?] ?`, `SELECT data ?& ARRAY[$1]`},
	}
	for _, version := range []int{16, 17, 18} {
		f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
		for _, pair := range pairs {
			want := one(t, f, sqlfingerprint.PostgreSQL, pair.database+" /* Rails tags */")
			if got := one(t, f, sqlfingerprint.PostgreSQL, pair.span); got != want {
				t.Errorf("PostgreSQL %d: %q differs from %q", version, pair.span, pair.database)
			}
		}
	}
}

// TestPostgresRubyOTelOrderCommentPreservesQuery ensures the comment marker
// doesn't remove meaningful operators, quoted text, or sort directions.
func TestPostgresRubyOTelOrderCommentPreservesQuery(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	query := `SELECT data ? 'name' FROM restaurants ORDER BY name`
	ascending := one(t, f, sqlfingerprint.PostgreSQL, query+" ASC ?")
	if ascending != one(t, f, sqlfingerprint.PostgreSQL, query+" ASC") {
		t.Fatal("comment marker changed the JSON operator or ordering")
	}
	if ascending == one(t, f, sqlfingerprint.PostgreSQL, query+" DESC ?") {
		t.Fatal("ascending and descending queries must remain distinct")
	}
	for _, query := range []string{`SELECT "?"`, `SELECT '?'`} {
		if one(t, f, sqlfingerprint.PostgreSQL, query) != one(t, f, sqlfingerprint.PostgreSQL, query+" ?") {
			t.Errorf("changed quoted question mark in %q", query)
		}
	}
	for _, query := range []string{`?`, `SELECT ?`, `SELECT * FROM restaurants WHERE id = ?`, `SELECT data ?|`, `SELECT data ?&`} {
		result := f.Fingerprint(sqlfingerprint.PostgreSQL, query)
		if len(result.Fingerprints) != 0 || len(result.Failures) != 1 {
			t.Errorf("accepted incomplete statement %q: %+v", query, result)
		}
	}
}

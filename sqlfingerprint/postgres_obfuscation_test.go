package sqlfingerprint_test

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestPostgresRubyOTelComment matches a Ruby span's obfuscated value and trailing
// Rails comment to the representative query returned by pg_stat_statements.
func TestPostgresRubyOTelComment(t *testing.T) {
	spanQuery := `SELECT AVG("reviews"."rating") FROM "reviews" WHERE "reviews"."restaurant_id" = ? ?`
	databaseQuery := `SELECT AVG("reviews"."rating") FROM "reviews" WHERE "reviews"."restaurant_id" = $1 /*action='index',application='TapasFinder',controller='restaurants'*/`
	for _, version := range []int{16, 17, 18} {
		t.Run(fmt.Sprintf("postgres%d", version), func(t *testing.T) {
			f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
			want := one(t, f, sqlfingerprint.PostgreSQL, databaseQuery)
			got := one(t, f, sqlfingerprint.PostgreSQL, spanQuery)
			if got != want {
				t.Fatalf("Ruby span and pg_stat_statements differ:\n%s\n%s", got, want)
			}
			// The statement splitter must apply the same rule before a semicolon.
			if got := one(t, f, sqlfingerprint.PostgreSQL, spanQuery+";"); got != want {
				t.Fatalf("semicolon changed fingerprint: got %s, want %s", got, want)
			}
		})
	}
}

// TestPostgresRubyOTelPreservesOperators prevents the compatibility rule from
// removing question marks used as PostgreSQL JSON operators or quoted names.
func TestPostgresRubyOTelPreservesOperators(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, pair := range []struct{ query, normalized string }{
		{`SELECT data ? 'rating' FROM reviews`, `SELECT data ? $1 FROM reviews`},
		{`SELECT data ?| ARRAY['rating'] FROM reviews`, `SELECT data ?| ARRAY[$1] FROM reviews`},
		{`SELECT data ?& ARRAY['rating'] FROM reviews`, `SELECT data ?& ARRAY[$1] FROM reviews`},
		{`SELECT "? ?" FROM reviews WHERE id = 1`, `SELECT "? ?" FROM reviews WHERE id = $1`},
	} {
		if one(t, f, sqlfingerprint.PostgreSQL, pair.query) != one(t, f, sqlfingerprint.PostgreSQL, pair.normalized) {
			t.Errorf("changed operator or identifier in %q", pair.query)
		}
	}
	if one(t, f, sqlfingerprint.PostgreSQL, `SELECT data ? 'rating' FROM reviews`) == one(t, f, sqlfingerprint.PostgreSQL, `SELECT data FROM reviews`) {
		t.Fatal("JSON existence operator disappeared")
	}
	// Removing the comment marker must still leave a complete expression.
	for _, query := range []string{`SELECT data ? ?`, `SELECT * FROM reviews WHERE id = ?`} {
		result := f.Fingerprint(sqlfingerprint.PostgreSQL, query)
		if len(result.Fingerprints) != 0 || len(result.Failures) != 1 {
			t.Errorf("accepted ambiguous trailing operator in %q: %+v", query, result)
		}
	}
}

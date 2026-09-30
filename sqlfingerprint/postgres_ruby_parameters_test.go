package sqlfingerprint_test

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// Obfuscating $1 removes its digits but leaves the dollar sign. The complete
// marker must survive trailing-comment removal, including at statement end.
func TestPostgresRubyDollarParameters(t *testing.T) {
	pairs := []struct{ span, database string }{
		{`SELECT "restaurants".* FROM "restaurants" WHERE (name ILIKE $? OR neighborhood ILIKE $? OR food_type ILIKE $?) ORDER BY "restaurants"."name" ASC`, `SELECT "restaurants".* FROM "restaurants" WHERE (name ILIKE $1 OR neighborhood ILIKE $2 OR food_type ILIKE $3) ORDER BY "restaurants"."name" ASC`},
		{`SELECT * FROM restaurants WHERE name NOT ilike $?`, `SELECT * FROM restaurants WHERE name NOT ilike $1`},
		{`SELECT AVG("reviews"."rating") FROM "reviews" WHERE "reviews"."restaurant_id" = $?`, `SELECT AVG("reviews"."rating") FROM "reviews" WHERE "reviews"."restaurant_id" = $1`},
		{`SELECT $?`, `SELECT $1`},
		{`SELECT $?::integer`, `SELECT $1::integer`},
		{`SELECT COALESCE($?, $?)`, `SELECT COALESCE($1, $2)`},
		{`SELECT * FROM reviews WHERE id IN ($?, $?)`, `SELECT * FROM reviews WHERE id IN ($1, $2)`},
		{`SELECT data ? $? FROM reviews`, `SELECT data ? $1 FROM reviews`},
		{`SELECT data ?| ARRAY[$?] FROM reviews`, `SELECT data ?| ARRAY[$1] FROM reviews`},
		{`INSERT INTO reviews (restaurant_id, rating) VALUES ($?, $?)`, `INSERT INTO reviews (restaurant_id, rating) VALUES ($1, $2)`},
		{`UPDATE reviews SET rating = $? WHERE id = $?`, `UPDATE reviews SET rating = $1 WHERE id = $2`},
	}
	for _, version := range []int{16, 17, 18} {
		t.Run(fmt.Sprintf("postgres%d", version), func(t *testing.T) {
			f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
			for _, pair := range pairs {
				want := one(t, f, sqlfingerprint.PostgreSQL, pair.database)
				for _, suffix := range []string{"", ";", " ?", " ?;"} {
					if got := one(t, f, sqlfingerprint.PostgreSQL, pair.span+suffix); got != want {
						t.Errorf("%q: got %s, want %s", pair.span+suffix, got, want)
					}
				}
			}
		})
	}
}

func TestPostgresRubyDollarParametersPreserveSyntax(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, query := range []string{`SELECT "$?" FROM reviews`, `SELECT '$?'`, `SELECT $$contains $?$$`, `SELECT $tag$contains $? and ;$tag$`} {
		if one(t, f, sqlfingerprint.PostgreSQL, query) != one(t, f, sqlfingerprint.PostgreSQL, query+" /* $? */") {
			t.Errorf("changed quoted SQL: %q", query)
		}
	}
	if one(t, f, sqlfingerprint.PostgreSQL, `SELECT "$?"`) == one(t, f, sqlfingerprint.PostgreSQL, `SELECT $?`) {
		t.Fatal("quoted identifier treated as parameter")
	}
	for _, query := range []string{`SELECT * FROM $?`, `SELECT $ ?`, `SELECT $?abc`, `SELECT $tag$unterminated $?`} {
		if result := f.Fingerprint(sqlfingerprint.PostgreSQL, query); len(result.Fingerprints) != 0 || len(result.Failures) == 0 {
			t.Errorf("accepted invalid SQL %q: %+v", query, result)
		}
	}
}

package sqlfingerprint_test

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestPostgresSQLAlchemyParameters compares the supplied fifteen-parameter
// SQLAlchemy query with PostgreSQL text while retaining each version's IN rules.
func TestPostgresSQLAlchemyParameters(t *testing.T) {
	spanQuery := `SELECT reviews.restaurant_id, reviews.id, reviews.author, reviews.rating, reviews.comment, reviews.created_at, reviews.updated_at FROM reviews WHERE reviews.restaurant_id IN (%(primary_keys_1)s::INTEGER, %(primary_keys_2)s::INTEGER, %(primary_keys_3)s::INTEGER, %(primary_keys_4)s::INTEGER, %(primary_keys_5)s::INTEGER, %(primary_keys_6)s::INTEGER, %(primary_keys_7)s::INTEGER, %(primary_keys_8)s::INTEGER, %(primary_keys_9)s::INTEGER, %(primary_keys_10)s::INTEGER, %(primary_keys_11)s::INTEGER, %(primary_keys_12)s::INTEGER, %(primary_keys_13)s::INTEGER, %(primary_keys_14)s::INTEGER, %(primary_keys_15)s::INTEGER) ORDER BY reviews.created_at DESC`
	databaseQuery := `SELECT reviews.restaurant_id, reviews.id, reviews.author, reviews.rating, reviews.comment, reviews.created_at, reviews.updated_at FROM reviews WHERE reviews.restaurant_id IN ($1::INTEGER, $2::INTEGER, $3::INTEGER, $4::INTEGER, $5::INTEGER, $6::INTEGER, $7::INTEGER, $8::INTEGER, $9::INTEGER, $10::INTEGER, $11::INTEGER, $12::INTEGER, $13::INTEGER, $14::INTEGER, $15::INTEGER) ORDER BY reviews.created_at DESC`
	for _, version := range []int{16, 17, 18} {
		t.Run(fmt.Sprintf("postgres%d", version), func(t *testing.T) {
			f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
			want := one(t, f, sqlfingerprint.PostgreSQL, databaseQuery)
			if got := one(t, f, sqlfingerprint.PostgreSQL, spanQuery); got != want {
				t.Fatalf("SQLAlchemy query and PostgreSQL statistics differ:\ngot  %s\nwant %s", got, want)
			}
			if got := one(t, f, sqlfingerprint.PostgreSQL, spanQuery+";\n"+databaseQuery); got != want {
				t.Fatalf("equivalent queries did not deduplicate: got %s, want %s", got, want)
			}
			if version == 18 {
				collapsed := `SELECT reviews.restaurant_id, reviews.id, reviews.author, reviews.rating, reviews.comment, reviews.created_at, reviews.updated_at FROM reviews WHERE reviews.restaurant_id IN ($1 /*, ... */) ORDER BY reviews.created_at DESC`
				if got := one(t, f, sqlfingerprint.PostgreSQL, collapsed); got != want {
					t.Fatalf("PostgreSQL 18 list marker differs: got %s, want %s", got, want)
				}
			}
		})
	}
}

// TestPostgresPyformatContexts covers named values, casts, JSON operators, and
// quoted text without treating the percent operator as a parameter marker.
func TestPostgresPyformatContexts(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, pair := range []struct{ query, normalized string }{
		{`SELECT %(value)s::INTEGER`, `SELECT $1::INTEGER`},
		{`SELECT %(id)s + %(id)s`, `SELECT $1 + $1`},
		{`SELECT %(user.key-1)s`, `SELECT $1`},
		{`SELECT %(value)s /* %(ignored)s */`, `SELECT $1`},
		{`INSERT INTO reviews (restaurant_id,rating) VALUES (%(restaurant)s,%(rating)s)`, `INSERT INTO reviews (restaurant_id,rating) VALUES ($1,$2)`},
		{`SELECT data ? %(key)s FROM reviews`, `SELECT data ? $1 FROM reviews`},
		{`SELECT '%(inside)s', %(id)s`, `SELECT $1, $2`},
		{`SELECT $$%(inside)s$$, %(id)s`, `SELECT $1, $2`},
		{`SELECT "%(id)s" FROM reviews WHERE id=%(id)s`, `SELECT "%(id)s" FROM reviews WHERE id=$1`},
		{`SELECT rating % (scale) FROM reviews WHERE id=%(id)s`, `SELECT rating % (scale) FROM reviews WHERE id=$1`},
		{`SELECT rating%(scale)s FROM reviews`, `SELECT rating % (scale) s FROM reviews`},
		{`SELECT rating %s FROM reviews`, `SELECT rating % s FROM reviews`},
	} {
		if one(t, f, sqlfingerprint.PostgreSQL, pair.query) != one(t, f, sqlfingerprint.PostgreSQL, pair.normalized) {
			t.Errorf("parameter context differs for %q", pair.query)
		}
	}
	for _, pair := range []struct{ query, different string }{
		{`SELECT %(id)s::INTEGER`, `SELECT $1::BIGINT`},
		{`SELECT "%(id)s" FROM reviews`, `SELECT "%(other)s" FROM reviews`},
		{`SELECT rating % (scale) FROM reviews`, `SELECT rating FROM reviews`},
	} {
		if one(t, f, sqlfingerprint.PostgreSQL, pair.query) == one(t, f, sqlfingerprint.PostgreSQL, pair.different) {
			t.Errorf("merged distinct queries %q and %q", pair.query, pair.different)
		}
	}
}

// TestPostgresPyformatListLengths keeps PostgreSQL 16/17 list arity after
// parameter replacement and applies PostgreSQL 18's existing collapse rule.
func TestPostgresPyformatListLengths(t *testing.T) {
	for _, version := range []int{16, 17, 18} {
		f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
		two := one(t, f, sqlfingerprint.PostgreSQL, `SELECT * FROM reviews WHERE id IN (%(a)s::INTEGER,%(b)s::INTEGER)`)
		three := one(t, f, sqlfingerprint.PostgreSQL, `SELECT * FROM reviews WHERE id IN ($1::INTEGER,$2::INTEGER,$3::INTEGER)`)
		if (two == three) != (version == 18) {
			t.Errorf("unexpected PostgreSQL %d IN-list equivalence", version)
		}
	}
}

// TestPostgresPyformatFailures rejects malformed named placeholders while
// allowing a later semicolon-separated statement to produce its fingerprint.
func TestPostgresPyformatFailures(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, query := range []string{
		`SELECT %()s`,
		`SELECT %(missing`,
		`SELECT %(nested(name))s`,
		`SELECT %(bad%name)s`,
		`SELECT %(name)d`,
	} {
		r := f.Fingerprint(sqlfingerprint.PostgreSQL, query)
		if len(r.Fingerprints) != 0 || len(r.Failures) != 1 {
			t.Errorf("malformed placeholder was fingerprinted in %q: %+v", query, r)
		}
	}
	r := f.Fingerprint(sqlfingerprint.PostgreSQL, `SELECT %(bad)d; SELECT %(id)s FROM reviews`)
	if len(r.Failures) != 1 || r.Failures[0].Statement != 0 || len(r.Fingerprints) != 1 {
		t.Fatalf("unexpected partial batch result: %+v", r)
	}
	if want := one(t, f, sqlfingerprint.PostgreSQL, `SELECT $1 FROM reviews`); r.Fingerprints[0] != want {
		t.Fatalf("later statement differs: got %s, want %s", r.Fingerprints[0], want)
	}
}

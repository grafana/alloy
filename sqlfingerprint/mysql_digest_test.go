package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestMySQLJavaInsert matches JDBC placeholders and nonreserved keyword columns
// to MySQL's digest text, including its collapsed multi-column value row.
func TestMySQLJavaInsert(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	spanQuery := "insert into reviews (author,comment,created_at,rating,restaurant_id,updated_at) values (?,?,?,?,?,?)"
	databaseQuery := "INSERT INTO `reviews` ( `author` , COMMENT , `created_at` , `rating` , `restaurant_id` , `updated_at` ) VALUES (...)"
	want := one(t, f, sqlfingerprint.MySQL, databaseQuery)
	for _, query := range []string{
		spanQuery,
		spanQuery + ";",
		"insert into reviews (author,CoMmEnT,created_at,rating,restaurant_id,updated_at) values (?,?,?,?,?,?)",
		"insert into reviews (author,comment,created_at,rating,restaurant_id,updated_at) values ('Alice','Great food','2026-09-28',5,42,'2026-09-28')",
	} {
		if got := one(t, f, sqlfingerprint.MySQL, query); got != want {
			t.Errorf("Java insert and MySQL digest differ for %q:\ngot  %s\nwant %s", query, got, want)
		}
	}
}

// TestMySQLCommentIdentifiers keeps quoted and qualified identifiers separate
// from unquoted keyword tokens without changing other dialects' identifiers.
func TestMySQLCommentIdentifiers(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, pair := range []struct {
		dialect           sqlfingerprint.Dialect
		query, normalized string
	}{
		{sqlfingerprint.MySQL, "select comment from reviews", "SELECT COMMENT FROM `reviews`"},
		{sqlfingerprint.MySQL, "select `comment` from reviews", "SELECT `comment` FROM `reviews`"},
		{sqlfingerprint.MySQL, "select r.comment from reviews r", "SELECT `r` . `comment` FROM `reviews` `r`"},
		{sqlfingerprint.MySQL, "select r.COMMENT from reviews r", "SELECT `r` . `COMMENT` FROM `reviews` `r`"},
		{sqlfingerprint.MySQL, "select comment.author from reviews", "SELECT `comment` . `author` FROM `reviews`"},
		{sqlfingerprint.MySQL, "select comment . author from reviews", "SELECT COMMENT . `author` FROM `reviews`"},
		{sqlfingerprint.MySQL, "select r. comment from reviews r", "SELECT `r` . COMMENT FROM `reviews` `r`"},
		{sqlfingerprint.PostgreSQL, "SELECT comment FROM reviews", `SELECT "comment" FROM "reviews"`},
		{sqlfingerprint.SQLServer, "SELECT comment FROM reviews", "SELECT [comment] FROM [reviews]"},
	} {
		if one(t, f, pair.dialect, pair.query) != one(t, f, pair.dialect, pair.normalized) {
			t.Errorf("identifier handling differs for %s query %q", pair.dialect, pair.query)
		}
	}
}

// TestMySQLInsertShape preserves column names, expressions, and the distinction
// between a single row and multiple rows when matching native value markers.
func TestMySQLInsertShape(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	want := one(t, f, sqlfingerprint.MySQL, "INSERT INTO `reviews` (`author`, COMMENT) VALUES (...)")
	for _, query := range []string{
		"INSERT INTO reviews (author,rating) VALUES (?,?)",
		"INSERT INTO reviews (author,comment) VALUES (?,?),(?,?)",
		"INSERT INTO reviews (author,comment) VALUES (?,COALESCE(?,?))",
	} {
		if one(t, f, sqlfingerprint.MySQL, query) == want {
			t.Errorf("merged distinct INSERT shape %q", query)
		}
	}
}

package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestMySQLRailsInsertColumnOrder correlates the supplied Rails INSERT with the
// MySQL statistics text even though they list the same columns in different orders.
func TestMySQLRailsInsertColumnOrder(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	databaseQuery := "INSERT INTO `restaurants` ( `name` , `neighborhood` , `food_type` , `description` , `address` , `price_range` , `created_at` , `updated_at` ) VALUES (...)"
	spanQuery := "insert into restaurants (address,created_at,description,food_type,name,neighborhood,price_range,updated_at) values (?,?,?,?,?,?,?,?)"
	want := one(t, f, sqlfingerprint.MySQL, databaseQuery)
	if got := one(t, f, sqlfingerprint.MySQL, spanQuery); got != want {
		t.Fatalf("Rails INSERT and MySQL statistics differ:\ngot  %s\nwant %s", got, want)
	}
	if got := one(t, f, sqlfingerprint.MySQL, spanQuery+";"+databaseQuery); got != want {
		t.Fatalf("equivalent INSERTs in a batch did not deduplicate: got %s, want %s", got, want)
	}
}

// TestMySQLInsertColumnPermutations covers value rows, native row markers, and
// quoted names while keeping table qualification and INSERT modifiers intact.
func TestMySQLInsertColumnPermutations(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, pair := range []struct{ query, normalized string }{
		{"INSERT INTO reviews (comment,author) VALUES (?,?)", "INSERT INTO `reviews` (`author`,COMMENT) VALUES (...)"},
		{"INSERT INTO t (b,a,c) VALUES (2,'text',NULL)", "INSERT INTO `t` (`c`,`b`,`a`) VALUES (...)"},
		{"INSERT INTO t (b,a) VALUES (?,?),(?,?)", "INSERT INTO `t` (`a`,`b`) VALUES (...) /* , ... */"},
		{"INSERT INTO t (b,a) VALUES (1,2),(3,4),(5,6)", "INSERT INTO t (a,b) VALUES (2,1),(4,3)"},
		{"INSERT INTO t (b,a) VALUES (...)", "INSERT INTO t (a,b) VALUES (...)"},
		{"INSERT INTO t (b,a) VALUES (?, ...)", "INSERT INTO t (a,b) VALUES (?,?)"},
		{"INSERT INTO t (`b,b`,`a``a`) VALUES (?,?)", "INSERT INTO `t` (`a``a`,`b,b`) VALUES (...)"},
		{"INSERT IGNORE INTO app.t (b,a) VALUES (?,?)", "INSERT IGNORE INTO `app`.`t` (`a`,`b`) VALUES (...)"},
		{"INSERT t (b,a) VALUES (?,?)", "INSERT `t` (`a`,`b`) VALUES (...)"},
	} {
		if one(t, f, sqlfingerprint.MySQL, pair.query) != one(t, f, sqlfingerprint.MySQL, pair.normalized) {
			t.Errorf("column permutation differs for %q and %q", pair.query, pair.normalized)
		}
	}
}

// TestMySQLInsertColumnOrderBoundaries prevents column sorting from discarding
// expression assignments, different column sets, or distinctions in other SQL.
func TestMySQLInsertColumnOrderBoundaries(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, pair := range []struct{ query, different string }{
		{"INSERT INTO t (b,a) VALUES (?,?)", "INSERT INTO t (a,c) VALUES (...)"},
		{"INSERT INTO t (b,a) VALUES (?,?)", "INSERT INTO other (a,b) VALUES (...)"},
		{"INSERT INTO app.t (b,a) VALUES (?,?)", "INSERT INTO t (a,b) VALUES (...)"},
		{"INSERT IGNORE INTO t (b,a) VALUES (?,?)", "INSERT INTO t (a,b) VALUES (...)"},
		{"INSERT INTO t (b,a) VALUES (?,?)", "INSERT INTO t (a,b) VALUES (?,?),(?,?)"},
		{"INSERT INTO t (b,a) VALUES (LOWER(?),?)", "INSERT INTO t (a,b) VALUES (LOWER(?),?)"},
		{"INSERT INTO t (b,a) VALUES (?,?),(LOWER(?),?)", "INSERT INTO t (a,b) VALUES (?,?),(LOWER(?),?)"},
		{"INSERT INTO t (b,a) VALUES (DEFAULT,?)", "INSERT INTO t (a,b) VALUES (DEFAULT,?)"},
		{"INSERT INTO t (b,a) SELECT x,y FROM source", "INSERT INTO t (a,b) SELECT x,y FROM source"},
		{"INSERT INTO t (b,a) VALUES (?,?) ON DUPLICATE KEY UPDATE a=VALUES(b)", "INSERT INTO t (a,b) VALUES (?,?) ON DUPLICATE KEY UPDATE a=VALUES(b)"},
		{"INSERT INTO t (b,a) VALUES (?,?) AS new(x,y) ON DUPLICATE KEY UPDATE a=x", "INSERT INTO t (a,b) VALUES (?,?) AS new(x,y) ON DUPLICATE KEY UPDATE a=x"},
		{"SELECT b,a FROM t", "SELECT a,b FROM t"},
	} {
		if one(t, f, sqlfingerprint.MySQL, pair.query) == one(t, f, sqlfingerprint.MySQL, pair.different) {
			t.Errorf("merged distinct statements %q and %q", pair.query, pair.different)
		}
	}
	for _, dialect := range []sqlfingerprint.Dialect{sqlfingerprint.PostgreSQL, sqlfingerprint.SQLServer} {
		if one(t, f, dialect, "INSERT INTO t (b,a) VALUES (1,2)") == one(t, f, dialect, "INSERT INTO t (a,b) VALUES (1,2)") {
			t.Errorf("MySQL column rule affected %s", dialect)
		}
	}
}

package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestNativeStatistics retains normalized text observed on PostgreSQL 16/18 and
// MySQL 8.0/8.4 during development. These regressions complement the documentation
// fixtures; they don't imply conformance for every statement or server version.
func TestNativeStatistics(t *testing.T) {
	tests := []struct {
		name, query, statistics string
		dialect                 sqlfingerprint.Dialect
		version                 int
	}{
		{"postgres18 array", "SELECT ARRAY[1,2,3]", "SELECT ARRAY[$1 /*, ... */]", sqlfingerprint.PostgreSQL, 18},
		{"postgres18 cast list", "SELECT * FROM t WHERE id IN (1::int,2::int,3::int)", "SELECT * FROM t WHERE id IN ($1 /*, ... */)", sqlfingerprint.PostgreSQL, 18},
		{"postgres18 string cast list", "SELECT * FROM t WHERE id IN ('1'::int,'2'::int)", "SELECT * FROM t WHERE id IN ($1 /*, ... */)", sqlfingerprint.PostgreSQL, 18},
		{"postgres18 representative single IN", "SELECT * FROM t WHERE id=-1", "SELECT * FROM t WHERE id IN ($1)", sqlfingerprint.PostgreSQL, 18},
		{"postgres18 locking", "SELECT * FROM t FOR UPDATE", "SELECT * FROM t FOR UPDATE", sqlfingerprint.PostgreSQL, 18},
		{"postgres16 array", "SELECT ARRAY[1,2,3]", "SELECT ARRAY[$1,$2,$3]", sqlfingerprint.PostgreSQL, 16},
		{"postgres16 list", "SELECT * FROM t WHERE id IN (1,2,3)", "SELECT * FROM t WHERE id IN ($1,$2,$3)", sqlfingerprint.PostgreSQL, 16},
		{"mysql aggregates", "select count(*), sum(id) from t where id=42", "SELECT COUNT ( * ) , SUM ( `id` ) FROM `t` WHERE `id` = ?", sqlfingerprint.MySQL, 0},
		{"mysql functions", "select coalesce(a,1),concat(a,b),now() from t", "SELECT COALESCE ( `a` , ? ) , `concat` ( `a` , `b` ) , NOW ( ) FROM `t`", sqlfingerprint.MySQL, 0},
		{"mysql NULL", "select * from t where a is null or b = null", "SELECT * FROM `t` WHERE `a` IS NULL OR `b` = ?", sqlfingerprint.MySQL, 0},
		{"mysql booleans", "select true,false from t", "SELECT TRUE , FALSE FROM `t`", sqlfingerprint.MySQL, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: tt.version})
			if one(t, f, tt.dialect, tt.query) != one(t, f, tt.dialect, tt.statistics) {
				t.Fatalf("native statistics didn't match %q -> %q", tt.query, tt.statistics)
			}
		})
	}
}

// TestNativeDistinctions prevents losing information still present in digest text.
func TestNativeDistinctions(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, tt := range []struct {
		dialect sqlfingerprint.Dialect
		a, b    string
	}{
		{sqlfingerprint.PostgreSQL, "SELECT ARRAY[1]", "SELECT ARRAY[1,2]"},
		{sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE id IN (1)", "SELECT * FROM t WHERE id IN (1,2)"},
		{sqlfingerprint.PostgreSQL, "SELECT Ä FROM t", "SELECT ä FROM t"},
		{sqlfingerprint.MySQL, "SELECT true FROM t", "SELECT false FROM t"},
		{sqlfingerprint.MySQL, "SELECT count FROM t", "SELECT COUNT(*) FROM t"},
	} {
		if one(t, f, tt.dialect, tt.a) == one(t, f, tt.dialect, tt.b) {
			t.Errorf("merged %q and %q", tt.a, tt.b)
		}
	}
}

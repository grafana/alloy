package sqlfingerprint_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// newFingerprinter fails the test immediately when fixture options are invalid.
func newFingerprinter(t testing.TB, options sqlfingerprint.Options) *sqlfingerprint.Fingerprinter {
	t.Helper()
	f, err := sqlfingerprint.New(options)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// one requires a complete successful match, so skipped SQL cannot make a pair
// test pass accidentally by comparing two empty result slices.
func one(t testing.TB, f *sqlfingerprint.Fingerprinter, dialect sqlfingerprint.Dialect, query string) string {
	t.Helper()
	r := f.Fingerprint(dialect, query)
	if len(r.Failures) != 0 || len(r.Fingerprints) != 1 {
		t.Fatalf("query %q: %+v", query, r)
	}
	return r.Fingerprints[0]
}

// TestDatabaseTextPairs exercises query/statistics representations, including
// native markers that ordinary SQL parsers reject. Sources are in README.md.
func TestDatabaseTextPairs(t *testing.T) {
	tests := []struct {
		name       string
		dialect    sqlfingerprint.Dialect
		raw, stats string
	}{
		{"postgres constants", sqlfingerprint.PostgreSQL, "SELECT * FROM public.orders WHERE id = 42 AND name = 'a'", "SELECT * FROM public.orders WHERE id = $1 AND name = $2"},
		{"postgres IN list", sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE id IN (1, 2, 3)", "SELECT * FROM t WHERE id IN ($1 /*, ... */)"},
		{"postgres casts", sqlfingerprint.PostgreSQL, "SELECT '-123'::int, 1.2e-3::numeric", "SELECT $1::int, $2::numeric"},
		{"postgres JSON", sqlfingerprint.PostgreSQL, "SELECT data ? 'key', data#>>'{a}' FROM t", "SELECT data ? $1, data#>>$2 FROM t"},
		{"postgres escaped string", sqlfingerprint.PostgreSQL, `SELECT E'a\';b', $tag$c;d$tag$`, "SELECT $1, $2"},
		{"postgres booleans", sqlfingerprint.PostgreSQL, "SELECT true, false, NULL", "SELECT $1, $2, $3"},
		{"postgres case", sqlfingerprint.PostgreSQL, "select ID from PUBLIC.Orders", `SELECT "id" FROM "public"."orders"`},
		{"postgres CTE", sqlfingerprint.PostgreSQL, "WITH x AS (SELECT 1 AS id) SELECT * FROM x WHERE id=2", "WITH x AS (SELECT $1 AS id) SELECT * FROM x WHERE id=$2"},
		{"mysql identifiers", sqlfingerprint.MySQL, "select * from orders where id=42", "SELECT * FROM `orders` WHERE `id` = ?"},
		{"mysql IN list", sqlfingerprint.MySQL, "SELECT * FROM t WHERE id IN (1,2,3)", "SELECT * FROM `t` WHERE `id` IN (...)"},
		{"mysql singleton IN", sqlfingerprint.MySQL, "SELECT * FROM t WHERE id IN (1)", "SELECT * FROM `t` WHERE `id` IN (...)"},
		{"mysql values", sqlfingerprint.MySQL, "INSERT INTO t VALUES (1,'a'), (2,'b')", "INSERT INTO `t` VALUES (...) /* , ... */"},
		{"mysql one column rows", sqlfingerprint.MySQL, "INSERT INTO t VALUES (1), (2)", "INSERT INTO `t` VALUES (?) /* , ... */"},
		{"mysql one row", sqlfingerprint.MySQL, "INSERT INTO t VALUES (1,'a')", "INSERT INTO `t` VALUES (...)"},
		{"mysql value lists", sqlfingerprint.MySQL, "SELECT 1,2,3 FROM t LIMIT 0,10", "SELECT ?, ... FROM `t` LIMIT ?, ..."},
		{"mysql negative literals", sqlfingerprint.MySQL, "SELECT * FROM t WHERE n=-12", "SELECT * FROM `t` WHERE `n`=?"},
		{"mysql comment rule", sqlfingerprint.MySQL, "SELECT a--1 FROM t", "SELECT `a` - ? FROM `t`"},
		{"sqlserver parameters", sqlfingerprint.SQLServer, "SELECT * FROM dbo.Orders WHERE id=42 AND name=N'x'", "SELECT * FROM [dbo].[Orders] WHERE id=@p1 AND name=@p2"},
		{"sqlserver prefix", sqlfingerprint.SQLServer, "SELECT * FROM dbo.Orders WHERE id=42", "(@p1 int)SELECT * FROM [dbo].[Orders] WHERE id=@p1"},
		{"sqlserver list", sqlfingerprint.SQLServer, "SELECT * FROM t WHERE id IN (1,2)", "SELECT * FROM t WHERE id IN (@p1,@p2)"},
		{"sqlserver escaped name", sqlfingerprint.SQLServer, `SELECT [a]];b] FROM t WHERE id=N'it''s;ok'`, `SELECT "a];b" FROM t WHERE id=@p1`},
		{"nested comments", sqlfingerprint.SQLServer, "SELECT /* a /* b; */ c */ 1", "SELECT @p1"},
	}
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := one(t, f, tt.dialect, tt.raw), one(t, f, tt.dialect, tt.stats)
			if a != b {
				t.Fatalf("raw and statistics text differ:\n%s\n%s", a, b)
			}
		})
	}
}

// TestDistinctShapes protects against plausible but misleading trace matches.
func TestDistinctShapes(t *testing.T) {
	tests := []struct {
		dialect sqlfingerprint.Dialect
		a, b    string
	}{
		{sqlfingerprint.PostgreSQL, "SELECT * FROM orders", "SELECT * FROM public.orders"},
		{sqlfingerprint.PostgreSQL, `SELECT * FROM "Orders"`, "SELECT * FROM orders"},
		{sqlfingerprint.PostgreSQL, "SELECT a-1 FROM t", "SELECT a FROM t"},
		{sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE x IS NULL", "SELECT * FROM t WHERE x = NULL"},
		{sqlfingerprint.PostgreSQL, "SELECT data ? 'x' FROM t", "SELECT data -> 'x' FROM t"},
		{sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE x IN (1,2)", "SELECT * FROM t WHERE x IN (a,b)"},
		{sqlfingerprint.PostgreSQL, "SELECT ab,c FROM t", "SELECT a,bc FROM t"},
		{sqlfingerprint.MySQL, "SELECT * FROM Orders", "SELECT * FROM orders"},
		{sqlfingerprint.MySQL, "INSERT INTO t VALUES (1),(2)", "INSERT INTO t VALUES (1,2),(3,4)"},
		{sqlfingerprint.MySQL, "INSERT INTO t VALUES (1,2)", "INSERT INTO t VALUES (1,2),(3,4)"},
		{sqlfingerprint.SQLServer, "SELECT * FROM t WHERE id IN (1,2)", "SELECT * FROM t WHERE id IN (1,2,3)"},
		{sqlfingerprint.SQLServer, "SELECT @@ROWCOUNT", "SELECT @p1"},
		{sqlfingerprint.SQLServer, "SELECT * FROM dbo.a", "SELECT * FROM dbo.b"},
	}
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, tt := range tests {
		if one(t, f, tt.dialect, tt.a) == one(t, f, tt.dialect, tt.b) {
			t.Errorf("merged distinct %s statements: %q and %q", tt.dialect, tt.a, tt.b)
		}
	}
}

// TestBatchFailures verifies deduplication, safe splitting, partial success,
// and stopping before a procedural body can leak into separate fingerprints.
func TestBatchFailures(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, dialect := range []sqlfingerprint.Dialect{sqlfingerprint.PostgreSQL, sqlfingerprint.MySQL, sqlfingerprint.SQLServer} {
		r := f.Fingerprint(dialect, ";SELECT ';' FROM a; SELECT 2 FROM a; SELECT * FROM; SELECT 3 FROM b;")
		if len(r.Fingerprints) != 2 || len(r.Failures) != 1 || r.Failures[0].Statement != 2 {
			t.Fatalf("%s batch: %+v", dialect, r)
		}
		for _, tail := range []string{"SELECT 'unterminated; SELECT 3 FROM b", "BEGIN; SELECT 3 FROM b; END", "CREATE PROCEDURE p AS SELECT 3 FROM b; SELECT 4 FROM c"} {
			r := f.Fingerprint(dialect, "SELECT 1 FROM a;"+tail)
			if len(r.Fingerprints) != 1 || len(r.Failures) != 1 {
				t.Fatalf("%s tail %q: %+v", dialect, tail, r)
			}
		}
	}
}

// TestRejectInput ensures failures never acquire a common fallback fingerprint.
func TestRejectInput(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, query := range []string{"SELECT * FROM t ...", "SELECT (1", "SELECT 1)", "SELECT /*open", "SELECT 1e+", "SELECT 0x", "SELECT 0b9", "SELECT 1 SELECT 2", "SELECT U&'escape'", "EXEC p", "SELECT /*+ hint */ 1", "SELECT /*!80000 1 */ 2", "SELECT \x00", "SELECT \xff"} {
		r := f.Fingerprint(sqlfingerprint.MySQL, query)
		if len(r.Fingerprints) != 0 || len(r.Failures) == 0 {
			t.Errorf("accepted %q: %+v", query, r)
		}
	}
	if r := f.Fingerprint(sqlfingerprint.PostgreSQL, "SELECT "+strings.Repeat("(", 129)+"1"+strings.Repeat(")", 129)); len(r.Failures) != 1 || r.Failures[0].Reason != sqlfingerprint.Limit {
		t.Fatalf("nesting limit: %+v", r)
	}
}

// TestOptions verifies the modes that affect how identical SQL bytes are read.
func TestOptions(t *testing.T) {
	for _, version := range []int{16, 17} {
		f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
		a := one(t, f, sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE id IN (1,2)")
		if a != one(t, f, sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE id IN ($1,$2)") || a == one(t, f, sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE id IN (1,2,3)") {
			t.Fatalf("PostgreSQL %d list arity", version)
		}
	}
	for _, tt := range []struct {
		dialect sqlfingerprint.Dialect
		options sqlfingerprint.Options
		a, b    string
	}{
		{sqlfingerprint.MySQL, sqlfingerprint.Options{MySQLANSIQuotes: true}, `SELECT "name" FROM "t"`, "SELECT `name` FROM `t`"},
		{sqlfingerprint.SQLServer, sqlfingerprint.Options{SQLServerQuotedIdentifierOff: true}, `SELECT "literal"`, "SELECT @p"},
		{sqlfingerprint.MySQL, sqlfingerprint.Options{MySQLNoBackslashEscapes: true}, `SELECT '\'`, "SELECT ?"},
		{sqlfingerprint.PostgreSQL, sqlfingerprint.Options{PostgreSQLBackslashEscapes: true}, `SELECT 'a\';b'`, "SELECT $1"},
	} {
		f := newFingerprinter(t, tt.options)
		if one(t, f, tt.dialect, tt.a) != one(t, f, tt.dialect, tt.b) {
			t.Fatalf("mode %+v did not match", tt.options)
		}
	}
	for _, options := range []sqlfingerprint.Options{{PostgreSQLVersion: 15}, {MaxQueryBytes: -1}} {
		if _, err := sqlfingerprint.New(options); err == nil {
			t.Fatalf("accepted invalid options %+v", options)
		}
	}
	f := newFingerprinter(t, sqlfingerprint.Options{MaxQueryBytes: 8})
	if r := f.Fingerprint(sqlfingerprint.MySQL, "SELECT 123"); len(r.Failures) != 1 || r.Failures[0].Reason != sqlfingerprint.Limit {
		t.Fatalf("size limit: %+v", r)
	}
}

// TestConcurrentUse exercises sharing a single instance between worker goroutines.
func TestConcurrentUse(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	want := one(t, f, sqlfingerprint.PostgreSQL, "SELECT * FROM t WHERE id=1")
	var workers sync.WaitGroup
	for i := range 16 {
		workers.Go(func() {
			r := f.Fingerprint(sqlfingerprint.PostgreSQL, fmt.Sprintf("SELECT * FROM t WHERE id=%d", i))
			if len(r.Fingerprints) != 1 || r.Fingerprints[0] != want {
				t.Errorf("concurrent result: %+v", r)
			}
		})
	}
	workers.Wait()
}

// FuzzFingerprint checks that arbitrary bytes never panic and results remain
// deterministic. Seeds cover INSERT column lists, quoting, nesting, native
// markers, and batch splitting.
func FuzzFingerprint(f *testing.F) {
	for _, seed := range []string{
		"SELECT 1",
		"SELECT $$a;b$$",
		"SELECT * FROM t WHERE id IN (...)",
		"SELECT 'a''b'; SELECT @p",
		"SELECT /* nested /* ; */ */ 1",
		"SELECT 'unterminated",
		"INSERT INTO t (b,a) VALUES (1,2)",
		"INSERT IGNORE INTO app.t (b,a) VALUES (...) /* , ... */",
		"INSERT INTO t (b,a) VALUES (LOWER(?),?)",
		"INSERT INTO t (b,a,) VALUES (?,?)",
	} {
		f.Add(seed)
	}
	n := newFingerprinter(f, sqlfingerprint.Options{MaxQueryBytes: 16384})
	f.Fuzz(func(t *testing.T, query string) {
		for _, dialect := range []sqlfingerprint.Dialect{sqlfingerprint.PostgreSQL, sqlfingerprint.MySQL, sqlfingerprint.SQLServer} {
			a, b := n.Fingerprint(dialect, query), n.Fingerprint(dialect, query)
			if fmt.Sprint(a) != fmt.Sprint(b) {
				t.Fatal("nondeterministic fingerprint")
			}
		}
	})
}

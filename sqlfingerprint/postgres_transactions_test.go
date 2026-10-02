package sqlfingerprint_test

import (
	"strings"
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

func TestPostgresTransactions(t *testing.T) {
	queries := []string{
		"BEGIN", "BEGIN WORK", "BEGIN TRANSACTION", "START TRANSACTION",
		"BEGIN ISOLATION LEVEL SERIALIZABLE", "BEGIN ISOLATION LEVEL REPEATABLE READ",
		"BEGIN ISOLATION LEVEL READ COMMITTED", "BEGIN ISOLATION LEVEL READ UNCOMMITTED",
		"BEGIN READ WRITE", "BEGIN READ ONLY", "BEGIN DEFERRABLE", "BEGIN NOT DEFERRABLE",
		"START TRANSACTION ISOLATION LEVEL SERIALIZABLE, READ ONLY, DEFERRABLE",
		"BEGIN WORK READ ONLY ISOLATION LEVEL SERIALIZABLE NOT DEFERRABLE",
		"SAVEPOINT active_record_1", "RELEASE active_record_1", "RELEASE SAVEPOINT active_record_1",
		`SAVEPOINT "MixedCase"`, `RELEASE SAVEPOINT "MixedCase"`,
		"ROLLBACK TO active_record_1", "ROLLBACK WORK TO SAVEPOINT active_record_1",
		`ROLLBACK TRANSACTION TO SAVEPOINT "MixedCase"`,
	}
	for _, command := range []string{"COMMIT", "END", "ROLLBACK", "ABORT"} {
		for _, noise := range []string{"", " WORK", " TRANSACTION"} {
			for _, chain := range []string{"", " AND CHAIN", " AND NO CHAIN"} {
				queries = append(queries, command+noise+chain)
			}
		}
	}
	for _, version := range []int{16, 17, 18} {
		f := newFingerprinter(t, sqlfingerprint.Options{PostgreSQLVersion: version})
		for _, query := range queries {
			want := one(t, f, sqlfingerprint.PostgreSQL, query)
			variants := []string{query + ";", "/* before */ " + query + " /* after */;", strings.ReplaceAll(query, " ", " /* gap */ "), query + " ?"}
			if !strings.Contains(query, `"`) {
				variants = append(variants, strings.ToLower(query), strings.ToUpper(query))
			}
			for _, variant := range variants {
				if got := one(t, f, sqlfingerprint.PostgreSQL, variant); got != want {
					t.Errorf("version %d: %q differs from %q", version, variant, query)
				}
			}
		}
		if one(t, f, sqlfingerprint.PostgreSQL, "COMMIT /*action='create',application='TapasFinder',controller='reviews'*/") != one(t, f, sqlfingerprint.PostgreSQL, "COMMIT") {
			t.Fatal("Rails comment changed fingerprint")
		}
	}
}

func TestPostgresTransactionDistinctions(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, pair := range [][2]string{
		{"BEGIN", "START TRANSACTION"}, {"COMMIT", "END"}, {"ROLLBACK", "ABORT"},
		{"COMMIT", "COMMIT WORK"}, {"COMMIT AND CHAIN", "COMMIT AND NO CHAIN"},
		{"BEGIN READ ONLY", "BEGIN READ WRITE"}, {"BEGIN DEFERRABLE", "BEGIN NOT DEFERRABLE"},
		{"BEGIN READ ONLY DEFERRABLE", "BEGIN DEFERRABLE READ ONLY"},
		{"SAVEPOINT a", "SAVEPOINT b"}, {`SAVEPOINT "A"`, "SAVEPOINT a"},
		{"ROLLBACK", "ROLLBACK TO a"}, {"RELEASE a", "RELEASE SAVEPOINT a"},
	} {
		if one(t, f, sqlfingerprint.PostgreSQL, pair[0]) == one(t, f, sqlfingerprint.PostgreSQL, pair[1]) {
			t.Errorf("merged distinct commands: %q", pair)
		}
	}
	if one(t, f, sqlfingerprint.PostgreSQL, `SAVEPOINT "a"`) != one(t, f, sqlfingerprint.PostgreSQL, "SAVEPOINT A") {
		t.Fatal("savepoint identifier folding differs")
	}
}

func TestPostgresTransactionFailures(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, query := range []string{
		"START", "START WORK", "BEGIN ISOLATION", "BEGIN ISOLATION LEVEL", "BEGIN ISOLATION LEVEL BAD",
		"BEGIN ISOLATION LEVEL REPEATABLE", "BEGIN ISOLATION LEVEL READ ONLY", "BEGIN READ", "BEGIN READ BAD", "BEGIN NOT",
		"BEGIN READ ONLY,", "BEGIN , READ ONLY", "COMMIT AND", "COMMIT AND NO", "COMMIT WORK TRANSACTION",
		"COMMIT SELECT 1", "BEGIN SELECT 1", "ROLLBACK TO", "ROLLBACK TO SAVEPOINT", "ROLLBACK TO a AND CHAIN",
		"SAVEPOINT", "SAVEPOINT 'a'", "SAVEPOINT $1", "SAVEPOINT a b", "RELEASE", "RELEASE SAVEPOINT",
		"ABORT TO a", "END TO a", "COMMIT /*open",
	} {
		r := f.Fingerprint(sqlfingerprint.PostgreSQL, query)
		if len(r.Fingerprints) != 0 || len(r.Failures) != 1 {
			t.Errorf("accepted malformed command %q: %+v", query, r)
		}
	}
	for _, query := range []string{"COMMIT PREPARED 'a'", "ROLLBACK PREPARED 'a'", "PREPARE TRANSACTION 'a'", "SET TRANSACTION READ ONLY"} {
		r := f.Fingerprint(sqlfingerprint.PostgreSQL, query)
		if len(r.Failures) != 1 || r.Failures[0].Reason != sqlfingerprint.Unsupported || len(r.Fingerprints) != 0 {
			t.Errorf("prepared or SET transaction accepted: %q: %+v", query, r)
		}
	}
}

func TestTransactionBatches(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for _, query := range []string{"BEGIN; SELECT 1; COMMIT", "BEGIN; SELECT 1; END"} {
		r := f.Fingerprint(sqlfingerprint.PostgreSQL, query)
		if len(r.Failures) != 0 || len(r.Fingerprints) != 3 {
			t.Fatalf("transaction batch %q: %+v", query, r)
		}
	}
	r := f.Fingerprint(sqlfingerprint.PostgreSQL, "BEGIN; SAVEPOINT a; SAVEPOINT a; ROLLBACK TO a; RELEASE a; COMMIT AND; COMMIT")
	if len(r.Fingerprints) != 5 || len(r.Failures) != 1 || r.Failures[0].Statement != 5 || r.Failures[0].Reason != sqlfingerprint.InvalidSQL {
		t.Fatalf("savepoint batch: %+v", r)
	}
	for _, dialect := range []sqlfingerprint.Dialect{sqlfingerprint.MySQL, sqlfingerprint.SQLServer} {
		r := f.Fingerprint(dialect, "SELECT 1; BEGIN; SELECT 2; COMMIT")
		if len(r.Fingerprints) != 1 || len(r.Failures) != 1 || r.Failures[0].Reason != sqlfingerprint.Unsupported {
			t.Fatalf("%s transaction guard changed: %+v", dialect, r)
		}
		if r := f.Fingerprint(dialect, "COMMIT"); len(r.Failures) != 1 || r.Failures[0].Reason != sqlfingerprint.InvalidSQL {
			t.Fatalf("%s COMMIT behavior changed: %+v", dialect, r)
		}
	}
}

package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestV6Fingerprints pins the external protocol independently of pair equality.
// Updating these values requires an intentional fingerprint-version decision.
func TestV6Fingerprints(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for dialect, expected := range map[sqlfingerprint.Dialect]string{
		sqlfingerprint.PostgreSQL: "v6:postgresql:debd3748fdd96c4a3d91437e5e7702b0e916f7e6e51724b8b427eca672a2ed2f",
		sqlfingerprint.MySQL:      "v6:mysql:4656a8275aa9099eaf857785e1d8e5c242ba8a3705e7939bde92347429534ed8",
		sqlfingerprint.SQLServer:  "v6:microsoft.sql_server:7363f2539c3b509285a8559da66a13df6de69595ee16b6392c12ae8c273c3099",
	} {
		if got := one(t, f, dialect, "SELECT id FROM orders WHERE id=42"); got != expected {
			t.Errorf("%s protocol changed: got %s, want %s", dialect, got, expected)
		}
	}
}

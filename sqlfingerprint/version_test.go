package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestV4Fingerprints pins the external protocol independently of pair equality.
// Updating these values requires an intentional fingerprint-version decision.
func TestV4Fingerprints(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for dialect, expected := range map[sqlfingerprint.Dialect]string{
		sqlfingerprint.PostgreSQL: "v4:postgresql:bcd92c00f72b11f8814bfba0c7d2e2bc8d684a7bfaa99b55780fdc4c350a5dcf",
		sqlfingerprint.MySQL:      "v4:mysql:0c4aeb2dce41b4a95b5b455614a4cf3e215c8624420ade0ab6ac554203c4c5d3",
		sqlfingerprint.SQLServer:  "v4:microsoft.sql_server:4a1f8aee17d3eeb8373ccbec66378901772e9a3910e61b62c9e323bd154ffef8",
	} {
		if got := one(t, f, dialect, "SELECT id FROM orders WHERE id=42"); got != expected {
			t.Errorf("%s protocol changed: got %s, want %s", dialect, got, expected)
		}
	}
}

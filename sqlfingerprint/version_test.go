package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestV5Fingerprints pins the external protocol independently of pair equality.
// Updating these values requires an intentional fingerprint-version decision.
func TestV5Fingerprints(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for dialect, expected := range map[sqlfingerprint.Dialect]string{
		sqlfingerprint.PostgreSQL: "v5:postgresql:2e42dec05173c67fdb07c7cd7eee79ca36dc0239ec889f0c9f57307f4ca208f6",
		sqlfingerprint.MySQL:      "v5:mysql:874d773502cb5173d39da1d699bd20772475e7ced06780ec1f6e1d41a929116a",
		sqlfingerprint.SQLServer:  "v5:microsoft.sql_server:ba933ec706bf87f93b01880a93916bb4cd5bbe684ba8a080724ba7d1df9ce6aa",
	} {
		if got := one(t, f, dialect, "SELECT id FROM orders WHERE id=42"); got != expected {
			t.Errorf("%s protocol changed: got %s, want %s", dialect, got, expected)
		}
	}
}

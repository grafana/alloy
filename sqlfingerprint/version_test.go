package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestV1Fingerprints pins the external protocol independently of pair equality.
// Updating these values requires an intentional fingerprint-version decision.
func TestV1Fingerprints(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for dialect, expected := range map[sqlfingerprint.Dialect]string{
		sqlfingerprint.PostgreSQL: "v1:postgresql:53bbfecdabdcad00985df3de40ad97edc1d58e3ec44c35dd91e5c42cc534b97f",
		sqlfingerprint.MySQL:      "v1:mysql:da985005ec9f4b0e7a88dcfaf3e57d27a2c79ea2b3a017fad010de91683a4684",
		sqlfingerprint.SQLServer:  "v1:microsoft.sql_server:31e8da82102a7c218f8542f7a55e7e029338628b0cef0873de05c684ab598e64",
	} {
		if got := one(t, f, dialect, "SELECT id FROM orders WHERE id=42"); got != expected {
			t.Errorf("%s protocol changed: got %s, want %s", dialect, got, expected)
		}
	}
}

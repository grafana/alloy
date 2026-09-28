package sqlfingerprint_test

import (
	"testing"

	"github.com/grafana/alloy/sqlfingerprint"
)

// TestV2Fingerprints pins the external protocol independently of pair equality.
// Updating these values requires an intentional fingerprint-version decision.
func TestV2Fingerprints(t *testing.T) {
	f := newFingerprinter(t, sqlfingerprint.Options{})
	for dialect, expected := range map[sqlfingerprint.Dialect]string{
		sqlfingerprint.PostgreSQL: "v2:postgresql:4567757293a7e75ea8769e88f7492373e8742eacf819976e8476963702ec3a51",
		sqlfingerprint.MySQL:      "v2:mysql:7875f9eaf1bc536bf2b777c4914b121e7717ae86218a7367f1fd89f7ef8702be",
		sqlfingerprint.SQLServer:  "v2:microsoft.sql_server:e408c4812904bde6e4495358d6d6f429aa466d4d67e4986d49d710493f3e14c4",
	} {
		if got := one(t, f, dialect, "SELECT id FROM orders WHERE id=42"); got != expected {
			t.Errorf("%s protocol changed: got %s, want %s", dialect, got, expected)
		}
	}
}

// Package extend runs secretfilter tests using a custom gitleaks config file
// (gitleaks.toml in this directory). It lives in a separate package so it runs
// in a different process from the main secretfilter tests, avoiding gitleaks'
// global viper and extendDepth state.
package extend

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/grafana/alloy/internal/component/loki/secretfilter"
)

// Why a separate package? The gitleaks library uses process-global state (viper and
// extendDepth) when loading configs with [extend] useDefault = true. If we ran these
// tests in the same package as the main secretfilter tests, they would share that
// state: viper can end up with merged config from a previous load, and extendDepth
// is never decremented, so after two loads the extend logic stops running. By
// putting the extend tests in a separate package, "go test ./..." runs them in a
// different process, so they get a clean viper and extendDepth and the custom
// gitleaks config is loaded and merged correctly.

// gitleaksConfigPath returns the path to gitleaks.toml in the extend package directory.
func gitleaksConfigPath(t *testing.T) string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "gitleaks.toml")
}

func TestSecretFiltering_WithGitleaksConfigFile(t *testing.T) {
	configPath := gitleaksConfigPath(t)
	if _, err := os.Stat(configPath); err != nil {
		t.Skipf("gitleaks.toml not found at %s: %v", configPath, err)
	}

	config := fmt.Sprintf(`
		forward_to = []
		gitleaks_config = %q
	`, configPath)

	// Same cases as default, plus the UUID allowlist case (generic-api-key when secret is a UUID).
	cases := secretfilter.DefaultTestCases()
	cases = append(cases, secretfilter.TestCase{
		Name:         "generic_api_key_uuid_allowlisted",
		InputLog:     `{"message": "audit tokenID=550e8400-e29b-41d4-a716-446655440000 resourceID=6ba7b810-9dad-11d1-80b4-00c04fd430c8"}`,
		ShouldRedact: false,
	})

	// One component, one config load for the whole test.
	secretfilter.RunTestCases(t, config, cases)
}

package extend

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/grafana/alloy/internal/component/loki/secretfilter"
)

// This test file is used to test the secretfilter component with a custom gitleaks config file.
// The reason why these tests are in a separate package is that the gitleaks library uses global variables
// when loading configs with [extend] useDefault = true. If we ran these
// tests in the same package as the main secretfilter tests, the extend logic does not work correctly.
// putting the extend tests in a separate package, "go test ./..." runs them in a
// different process, so they get a clean viper and gitleaks global state.

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

	secretfilter.RunTestCases(t, config, cases)
}

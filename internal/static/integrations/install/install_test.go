package install

import (
	"fmt"
	"strings"
	"testing"

	v1 "github.com/grafana/agent/internal/static/integrations"
	v2 "github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/stretchr/testify/require"
)

// TestV1_Shims ensures that every v1 integration has a v2 counterpart.
func TestV1_Shims(t *testing.T) {
	var (
		v2Integrations      = make(map[string]v2.Config) // Native v2 integrations
		shimmedIntegrations = make(map[string]v1.Config) // Shimmed v1 integrations
	)

	for _, v2Integration := range v2.Registered() {
		uc, ok := v2Integration.(v2.UpgradedConfig)
		if !ok {
			v2Integrations[v2Integration.Name()] = v2Integration
			continue
		}

		v1Integration, _ := uc.LegacyConfig()
		shimmedIntegrations[v1Integration.Name()] = v1Integration
	}

	for _, v1Integration := range v1.RegisteredIntegrations() {
		t.Run(v1Integration.Name(), func(t *testing.T) {
			_, v2Native := v2Integrations[v1Integration.Name()]
			_, shimmed := shimmedIntegrations[v1Integration.Name()]
			require.True(t, shimmed || v2Native, "integration not shimmed to v2 or does not have a native counterpart")
		})
	}
}

// TestV2_NoExporterSuffix ensures that v2 integrations do not have a name
// ending in _exporter. The test may be updated to exclude specific
// integrations from this requirement.
func TestV2_NoExporterSuffix(t *testing.T) {
	exceptions := map[string]struct{}{
		"node_exporter": {}, // node_exporter is an exception because its name is well-understood
	}

	var invalidNames []string

	for _, v2Integration := range v2.Registered() {
		name := v2Integration.Name()
		if _, excluded := exceptions[name]; excluded {
			continue
		}

		if strings.HasSuffix(name, "_exporter") {
			invalidNames = append(invalidNames, name)
		}
	}

	if len(invalidNames) > 0 {
		require.FailNow(
			t,
			"Found v2 integrations named with unexpected _exporter suffix",
			fmt.Sprintf("The following integrations must not end in _exporter: %s. Either drop the suffix or add them as an exception in this test.", strings.Join(invalidNames, ", ")),
		)
	}
}

package securitypolicy_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/alloy/internal/securitypolicy"
	"github.com/stretchr/testify/require"
)

func TestCheckComponent_NilPolicy(t *testing.T) {
	var p *securitypolicy.SecurityPolicy
	require.NoError(t, p.CheckComponent("remote.http"))
	require.NoError(t, p.CheckComponent("anything"))
}

func TestCheckComponent_AbsentSection(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{}
	require.NoError(t, p.CheckComponent("remote.http"))
}

func TestCheckComponent_Allowlist(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{
		Components: securitypolicy.PolicySection{
			Mode: "allowlist",
			List: []string{"loki.write", "prometheus.scrape"},
		},
	}
	require.NoError(t, p.CheckComponent("loki.write"))
	require.NoError(t, p.CheckComponent("prometheus.scrape"))
	require.Error(t, p.CheckComponent("remote.http"))
	require.Error(t, p.CheckComponent("loki.source.file"))
}

func TestCheckComponent_Denylist(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{
		Components: securitypolicy.PolicySection{
			Mode: "denylist",
			List: []string{"remote.http", "loki.write"},
		},
	}
	require.Error(t, p.CheckComponent("remote.http"))
	require.Error(t, p.CheckComponent("loki.write"))
	require.NoError(t, p.CheckComponent("prometheus.scrape"))
	require.NoError(t, p.CheckComponent("discovery.kubernetes"))
}

func TestCheckComponent_UnknownMode(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{
		Components: securitypolicy.PolicySection{
			Mode: "badmode",
			List: []string{"remote.http"},
		},
	}
	require.Error(t, p.CheckComponent("remote.http"))
}

func TestLoadFromFile_Empty(t *testing.T) {
	p, err := securitypolicy.LoadFromFile("")
	require.NoError(t, err)
	require.Nil(t, p)
}

func TestLoadFromFile_Valid(t *testing.T) {
	content := `
components:
  mode: denylist
  list:
    - remote.http
    - loki.write
`
	path := filepath.Join(t.TempDir(), "policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	p, err := securitypolicy.LoadFromFile(path)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, "denylist", p.Components.Mode)
	require.Equal(t, []string{"remote.http", "loki.write"}, p.Components.List)
	require.Error(t, p.CheckComponent("remote.http"))
	require.NoError(t, p.CheckComponent("prometheus.scrape"))
}

func TestLoadFromFile_Missing(t *testing.T) {
	_, err := securitypolicy.LoadFromFile("/nonexistent/path/policy.yaml")
	require.Error(t, err)
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	// Unclosed flow sequence is invalid YAML
	require.NoError(t, os.WriteFile(path, []byte("components: [unclosed"), 0o600))
	_, err := securitypolicy.LoadFromFile(path)
	require.Error(t, err)
}

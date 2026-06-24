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

func TestCheckEndpoint_NilPolicy(t *testing.T) {
	var p *securitypolicy.SecurityPolicy
	require.NoError(t, p.CheckEndpoint("https://evil.com/exfil"))
}

func TestCheckEndpoint_AbsentSection(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{}
	require.NoError(t, p.CheckEndpoint("https://anywhere.com/"))
}

func TestCheckEndpoint_Allowlist(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{
		Endpoints: securitypolicy.EndpointsSection{
			Mode:     "allowlist",
			Patterns: []string{"https://grafana.com/**", "https://logs.grafana.net/**"},
		},
	}
	require.NoError(t, p.CheckEndpoint("https://grafana.com/loki/api/v1/push"))
	require.NoError(t, p.CheckEndpoint("https://logs.grafana.net/loki/api/v1/push"))
	require.Error(t, p.CheckEndpoint("https://evil.com/exfil"))
	require.Error(t, p.CheckEndpoint("http://192.168.0.1:3100/loki/api/v1/push"))
}

func TestCheckEndpoint_Denylist(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{
		Endpoints: securitypolicy.EndpointsSection{
			Mode:     "denylist",
			Patterns: []string{"http://192.168.**/**", "https://evil.com/**"},
		},
	}
	require.Error(t, p.CheckEndpoint("http://192.168.0.1:3100/loki/api/v1/push"))
	require.Error(t, p.CheckEndpoint("https://evil.com/exfil"))
	require.NoError(t, p.CheckEndpoint("https://logs.grafana.net/loki/api/v1/push"))
}

func TestCheckEndpoint_URLNormalization(t *testing.T) {
	p := &securitypolicy.SecurityPolicy{
		Endpoints: securitypolicy.EndpointsSection{
			Mode:     "allowlist",
			Patterns: []string{"https://grafana.com/**"},
		},
	}
	// Default port stripped, scheme/host lowercased before matching
	require.NoError(t, p.CheckEndpoint("HTTPS://Grafana.com:443/loki/api/v1/push"))
	require.Error(t, p.CheckEndpoint("https://evil.com/"))
}

func TestCheckEndpoint_GlobPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		url     string
		wantErr bool
	}{
		// ** crosses path separators
		{"double-star matches deep path", "https://grafana.com/**", "https://grafana.com/loki/api/v1/push", false},
		{"double-star matches root slash", "https://grafana.com/**", "https://grafana.com/", false},
		// * matches within a single hostname segment
		{"single-star subdomain matches", "https://*.grafana.com/**", "https://logs.grafana.com/push", false},
		{"single-star subdomain blocks wrong host", "https://*.grafana.com/**", "https://evil.com/push", true},
		{"single-star does not cross dot", "https://*.grafana.com/**", "https://grafana.com/push", true},
		// exact host
		{"exact host matches", "https://logs.grafana.net/**", "https://logs.grafana.net/loki/api/v1/push", false},
		{"exact host blocks subdomain", "https://logs.grafana.net/**", "https://sub.logs.grafana.net/push", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &securitypolicy.SecurityPolicy{
				Endpoints: securitypolicy.EndpointsSection{
					Mode:     "allowlist",
					Patterns: []string{tc.pattern},
				},
			}
			err := p.CheckEndpoint(tc.url)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
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

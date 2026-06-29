package github_exporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/static/config"
	"github.com/grafana/alloy/internal/util"
)

func TestConfig_SecretGithub(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  github_exporter:
    enabled: true
    api_token: secret_api`
	config.CheckSecret(t, stringCfg, "secret_api")
}

func TestConfig_GitHubApp(t *testing.T) {
	yamlCfg := `
api_url: https://api.github.com
repositories:
  - grafana/alloy
github_app_id: 123456
github_app_installation_id: 789012
github_app_key_path: /etc/github-app-key.pem
github_rate_limit: 10000
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlCfg), &cfg)
	require.NoError(t, err)
	require.Equal(t, "https://api.github.com", cfg.APIURL)
	require.Equal(t, []string{"grafana/alloy"}, cfg.Repositories)
	require.Equal(t, int64(123456), cfg.GitHubAppID)
	require.Equal(t, int64(789012), cfg.GitHubAppInstallationID)
	require.Equal(t, "/etc/github-app-key.pem", cfg.GitHubAppKeyPath)
	require.Equal(t, 10000.0, cfg.GitHubRateLimit)
}

func TestConfig_DefaultValues(t *testing.T) {
	yamlCfg := `
repositories:
  - grafana/alloy
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlCfg), &cfg)
	require.NoError(t, err)
	require.Equal(t, DefaultConfig.APIURL, cfg.APIURL)
	require.Equal(t, DefaultConfig.GitHubRateLimit, cfg.GitHubRateLimit)
}

func TestConfig_MutuallyExclusiveAuth(t *testing.T) {
	yamlCfg := `
api_url: https://api.github.com
repositories:
  - grafana/alloy
api_token: my-token
github_app_id: 123456
github_app_installation_id: 789012
github_app_key_path: /etc/github-app-key.pem
`
	var cfg Config
	err := yaml.Unmarshal([]byte(yamlCfg), &cfg)
	require.NoError(t, err)

	// The New function should return an error when both auth methods are configured
	_, err = New(util.TestAlloyLogger(t).Slog(), &cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use both token authentication and GitHub App authentication")
}

func TestLatestReleaseMetricIncludesTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/repos/grafana/alloy":
			_, _ = w.Write([]byte(`{
				"name": "alloy",
				"owner": {"login": "grafana"},
				"license": {"key": "apache-2.0"},
				"language": "Go"
			}`))
		case "/repos/grafana/alloy/releases":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/grafana/alloy/pulls":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/grafana/alloy/releases/latest":
			_, _ = w.Write([]byte(`{
				"name": "Alloy v1.2.3",
				"tag_name": "v1.2.3",
				"created_at": "2026-06-01T00:00:00Z",
				"published_at": "2026-06-02T00:00:00Z"
			}`))
		case "/rate_limit":
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "4999")
			w.Header().Set("X-RateLimit-Reset", "1780000000")
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	integration, err := New(util.TestAlloyLogger(t).Slog(), &Config{
		APIURL:       server.URL,
		Repositories: []string{"grafana/alloy"},
	})
	require.NoError(t, err)

	handler, err := integration.MetricsHandler()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `github_repo_latest_release_info{created_at="2026-06-01T00:00:00Z",published_at="2026-06-02T00:00:00Z",release="Alloy v1.2.3",repo="alloy",tag="v1.2.3",user="grafana"} 1`)
}

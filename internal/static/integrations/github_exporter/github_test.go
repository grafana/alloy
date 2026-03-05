package github_exporter

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
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
	logger := log.NewNopLogger()
	_, err = New(logger, &cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot use both token authentication and GitHub App authentication")
}

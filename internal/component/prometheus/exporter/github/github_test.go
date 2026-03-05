package github

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalAlloy(t *testing.T) {
	alloyCfg := `
		api_token_file = "/etc/github-api-token"
		repositories = ["grafana/alloy"]
		organizations = ["grafana", "prometheus"]
		users = ["jcreixell"]
		api_url = "https://some-other-api.github.com"
`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "/etc/github-api-token", args.APITokenFile)
	require.Equal(t, []string{"grafana/alloy"}, args.Repositories)
	require.Contains(t, args.Organizations, "grafana")
	require.Contains(t, args.Organizations, "prometheus")
	require.Equal(t, []string{"jcreixell"}, args.Users)
	require.Equal(t, "https://some-other-api.github.com", args.APIURL)
}

func TestUnmarshalAlloyWithGitHubApp(t *testing.T) {
	alloyCfg := `
		repositories = ["grafana/alloy"]
		api_url = "https://api.github.com"
		github_app_id = 123456
		github_app_installation_id = 789012
		github_app_key_path = "/etc/github-app-key.pem"
		github_rate_limit = 10000
`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.Equal(t, []string{"grafana/alloy"}, args.Repositories)
	require.Equal(t, "https://api.github.com", args.APIURL)
	require.Equal(t, int64(123456), args.GitHubAppID)
	require.Equal(t, int64(789012), args.GitHubAppInstallationID)
	require.Equal(t, "/etc/github-app-key.pem", args.GitHubAppKeyPath)
	require.Equal(t, 10000.0, args.GitHubRateLimit)
}

func TestConvert(t *testing.T) {
	args := Arguments{
		APITokenFile:  "/etc/github-api-token",
		Repositories:  []string{"grafana/alloy"},
		Organizations: []string{"grafana", "prometheus"},
		Users:         []string{"jcreixell"},
		APIURL:        "https://some-other-api.github.com",
	}

	res := args.Convert()
	require.Equal(t, "/etc/github-api-token", res.APITokenFile)
	require.Equal(t, []string{"grafana/alloy"}, res.Repositories)
	require.Contains(t, res.Organizations, "grafana")
	require.Contains(t, res.Organizations, "prometheus")
	require.Equal(t, []string{"jcreixell"}, res.Users)
	require.Equal(t, "https://some-other-api.github.com", res.APIURL)
}

func TestConvertWithGitHubApp(t *testing.T) {
	args := Arguments{
		Repositories:            []string{"grafana/alloy"},
		APIURL:                  "https://api.github.com",
		GitHubAppID:             123456,
		GitHubAppInstallationID: 789012,
		GitHubAppKeyPath:        "/etc/github-app-key.pem",
		GitHubRateLimit:         10000,
	}

	res := args.Convert()
	require.Equal(t, []string{"grafana/alloy"}, res.Repositories)
	require.Equal(t, "https://api.github.com", res.APIURL)
	require.Equal(t, int64(123456), res.GitHubAppID)
	require.Equal(t, int64(789012), res.GitHubAppInstallationID)
	require.Equal(t, "/etc/github-app-key.pem", res.GitHubAppKeyPath)
	require.Equal(t, 10000.0, res.GitHubRateLimit)
}

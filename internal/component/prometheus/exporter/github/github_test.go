package github

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalRiver(t *testing.T) {
	riverCfg := `
		api_token_file = "/etc/github-api-token"
		repositories = ["grafana/agent"]
		organizations = ["grafana", "prometheus"]
		users = ["jcreixell"]
		api_url = "https://some-other-api.github.com"
`
	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)
	require.Equal(t, "/etc/github-api-token", args.APITokenFile)
	require.Equal(t, []string{"grafana/agent"}, args.Repositories)
	require.Contains(t, args.Organizations, "grafana")
	require.Contains(t, args.Organizations, "prometheus")
	require.Equal(t, []string{"jcreixell"}, args.Users)
	require.Equal(t, "https://some-other-api.github.com", args.APIURL)
}

func TestConvert(t *testing.T) {
	args := Arguments{
		APITokenFile:  "/etc/github-api-token",
		Repositories:  []string{"grafana/agent"},
		Organizations: []string{"grafana", "prometheus"},
		Users:         []string{"jcreixell"},
		APIURL:        "https://some-other-api.github.com",
	}

	res := args.Convert()
	require.Equal(t, "/etc/github-api-token", res.APITokenFile)
	require.Equal(t, []string{"grafana/agent"}, res.Repositories)
	require.Contains(t, res.Organizations, "grafana")
	require.Contains(t, res.Organizations, "prometheus")
	require.Equal(t, []string{"jcreixell"}, res.Users)
	require.Equal(t, "https://some-other-api.github.com", res.APIURL)
}

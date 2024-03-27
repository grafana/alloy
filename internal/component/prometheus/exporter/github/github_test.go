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

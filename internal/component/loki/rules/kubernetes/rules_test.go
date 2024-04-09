package rules

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	address = "GRAFANA_CLOUD_METRICS_URL"
	basic_auth {
		username = "GRAFANA_CLOUD_USER"
		password = "GRAFANA_CLOUD_API_KEY"
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestBadAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	address = "GRAFANA_CLOUD_METRICS_URL"
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}

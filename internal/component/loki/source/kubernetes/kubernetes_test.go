package kubernetes

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	targets    = [
		{"__address__" = "localhost:9090", "foo" = "bar"},
		{"__address__" = "localhost:8080", "foo" = "buzz"},
	]
    forward_to = []
	client {
		api_server = "localhost:9091"
	}
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
}

func TestBadRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	targets    = [
		{"__address__" = "localhost:9090", "foo" = "bar"},
		{"__address__" = "localhost:8080", "foo" = "buzz"},
	]
    forward_to = []
	client {
		api_server = "localhost:9091"
		bearer_token = "token"
		bearer_token_file = "/path/to/file.token"
	}
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}

package consul

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	server = "consul.example.com:8500"
	services = ["my-service"]
	token = "my-token"
	allow_stale = false
	node_meta = { foo = "bar" }
	http_headers = {
		"foo" = ["foobar"],
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	header := args.HTTPClientConfig.HTTPHeaders.Headers["foo"][0]
	require.Equal(t, "foobar", string(header))
}

func TestBadAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	server = "consul.example.com:8500"
	services = ["my-service"]
	basic_auth {
		username = "user"
		password = "pass"
		password_file = "/somewhere.txt"
	}
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth password & password_file must be configured")
}

package consul

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	server = "consul.example.com:8500"
	services = ["my-service"]
	token = "my-token"
	allow_stale = false
	node_meta = { foo = "bar" }
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
}

func TestBadRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
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
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth password & password_file must be configured")
}

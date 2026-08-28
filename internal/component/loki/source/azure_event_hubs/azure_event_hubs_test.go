package azure_event_hubs

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfigOAuth(t *testing.T) {
	var exampleAlloyConfig = `

	fully_qualified_namespace = "my-ns.servicebus.windows.net:9093"
	event_hubs                = ["test"]
	forward_to                = []

	authentication {
		mechanism = "oauth"
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestAlloyConfigConnectionString(t *testing.T) {
	var exampleAlloyConfig = `

	fully_qualified_namespace = "my-ns.servicebus.windows.net:9093"
	event_hubs                = ["test"]
	forward_to                = []

	authentication {
		mechanism         = "connection_string"
		connection_string = "my-conn-string"
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestAlloyConfigValidateAssignor(t *testing.T) {
	var exampleAlloyConfig = `

	fully_qualified_namespace = "my-ns.servicebus.windows.net:9093"
	event_hubs                = ["test"]
	forward_to                = []
    assignor                  = "invalid-value"

	authentication {
		mechanism         = "connection_string"
		connection_string = "my-conn-string"
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.EqualError(t, err, "assignor value invalid-value is invalid, must be one of: [sticky roundrobin range]")
}

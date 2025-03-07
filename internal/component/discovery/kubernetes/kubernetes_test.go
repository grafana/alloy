package kubernetes

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	role = "pod"
    kubeconfig_file = "/etc/k8s/kubeconfig.yaml"
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
	role = "pod"
    namespaces {
		names = ["myapp"]
	}
	bearer_token = "token"
	bearer_token_file = "/path/to/file.token"
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}

func TestAttachMetadata(t *testing.T) {
	var exampleAlloyConfig = `
        role = "pod"
    attach_metadata {
	    node = true
    }
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

package queue

import (
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestBasicAuthAndTLSBothSetError(t *testing.T) {
	args := defaultArgs()
	args.Endpoints = make([]EndpointConfig, 1)
	args.Endpoints[0] = defaultEndpointConfig()
	args.Endpoints[0].Name = "test"
	args.Endpoints[0].TLSConfig = &TLSConfig{}
	args.Endpoints[0].BasicAuth = &BasicAuth{}
	err := args.Validate()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "cannot have both BasicAuth and TLSConfig"))
}

func TestParsingTLSConfig(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal([]byte(`
    endpoint "cloud"  {
        url = "http://example.com"
        basic_auth {
            username = 12345
            password = "password"
        }
        tls_config {
            insecure_skip_verify = true
        }

    }
`), &args)

	require.NoError(t, err)
}

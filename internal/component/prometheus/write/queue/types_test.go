package queue

import (
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"testing"
)

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

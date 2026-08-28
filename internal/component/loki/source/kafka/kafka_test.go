package kafka

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	brokers                = ["localhost:9092","localhost:23456"]
	topics                 = ["quickstart-events"]
	labels                 = {component = "loki.source.kafka"}
	forward_to             = []
	use_incoming_timestamp = true
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestTLSAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	brokers                = ["localhost:9092","localhost:23456"]
	topics                 = ["quickstart-events"]
	authentication {
		type = "ssl"
		tls_config {
			cert_file = "/fake/file.cert"
            key_file  = "/fake/file.key"
		}
	}
	labels                 = {component = "loki.source.kafka"}
	forward_to             = []
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestSASLAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	brokers                = ["localhost:9092","localhost:23456"]
	topics                 = ["quickstart-events"]
	authentication {
		type = "sasl"
		sasl_config {
			user     = "user"
            password = "password"
		}
	}
	labels                 = {component = "loki.source.kafka"}
	forward_to             = []
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestSASLOAuthAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	brokers = ["localhost:9092", "localhost:23456"]
	topics  = ["quickstart-events"]

	authentication {
		type = "sasl"
		sasl_config {
			mechanism = "OAUTHBEARER"
			oauth_config {
				token_provider = "azure"
				scopes         = ["my-scope"]
			}
		}
	}
	labels     = {component = "loki.source.kafka"}
	forward_to = []
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

package nerve

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestRiverConfig(t *testing.T) {
	var exampleRiverConfig = `
	servers = ["1.2.3.4"]
	paths   = ["/nerve/services/your_http_service/services", "/nerve/services/your_tcp_service/services"]
	timeout = "15s"
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)
}

func TestBadRiverConfig(t *testing.T) {
	var (
		args        Arguments
		riverConfig string
	)

	riverConfig = `
	servers = ["1.2.3.4"]
	paths   = ["/nerve/services/your_http_service/services", "/nerve/services/your_tcp_service/services"]
	timeout = "0s"
`

	require.ErrorContains(t, syntax.Unmarshal([]byte(riverConfig), &args), "timeout must be greater than 0")
}

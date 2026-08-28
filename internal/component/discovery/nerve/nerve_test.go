package nerve

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	servers = ["1.2.3.4"]
	paths   = ["/nerve/services/your_http_service/services", "/nerve/services/your_tcp_service/services"]
	timeout = "15s"
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestBadAlloyConfig(t *testing.T) {
	var (
		args        Arguments
		alloyConfig string
	)

	alloyConfig = `
	servers = ["1.2.3.4"]
	paths   = ["/nerve/services/your_http_service/services", "/nerve/services/your_tcp_service/services"]
	timeout = "0s"
`

	require.ErrorContains(t, syntax.Unmarshal([]byte(alloyConfig), &args), "timeout must be greater than 0")
}

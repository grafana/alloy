package remotecfg

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/internal/build"
	"github.com/stretchr/testify/require"
)

func TestTunnelCarriesGraphQLRequest(t *testing.T) {
	harness := newTunnelTestHarness(t)
	harness.Start()
	requestID, reply := harness.Send(graphQLRequest(`{"query":"{ alloy { version } }"}`))
	result := <-reply

	require.NotNil(t, result)
	require.Equal(t, requestID, result.GetRequestId())
	// require.Equal(t, uint32(http.StatusOK), result.GetResponse().GetStatus())
	require.JSONEq(t,
		fmt.Sprintf(`{"data":{"alloy":{"version":%q}}}`, build.Version),
		string(result.GetResponse().GetBody()),
	)
}

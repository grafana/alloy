package remotecfg

import (
	"testing"

	tunnelv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/tunnel/v1"
	"github.com/stretchr/testify/require"
)

func TestIsAllowedTunnelRoute(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		method  tunnelv1.HTTPRequestMethod
		allowed bool
	}{
		{"GraphQL POST", "/graphql", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_POST, true},
		{"GraphQL GET", "/graphql", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_GET, false},
		{"GraphQL subpath", "/graphql/playground", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_POST, false},
		{"metrics GET", "/metrics", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_GET, true},
		{"metrics POST", "/metrics", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_POST, false},
		{"pprof root GET", "/debug/pprof", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_GET, true},
		{"pprof subpath GET", "/debug/pprof/goroutine", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_GET, true},
		{"pprof subpath POST", "/debug/pprof/goroutine", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_POST, false},
		{"support GET", "/-/support", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_GET, true},
		{"support subpath", "/-/support/redacted", tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_GET, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.allowed, isAllowedTunnelRoute(tc.path, tc.method))
		})
	}
}

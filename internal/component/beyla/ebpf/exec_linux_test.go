//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// unixPathMax is the size of sun_path in the Linux kernel's sockaddr_un.
const unixPathMax = 108

func TestAbstractSocketAddr(t *testing.T) {
	// Long remotecfg pipeline names previously pushed the name past sun_path.
	longID := "remotecfg/beyla_k8s_appo11y_k8s_monitoring_sdk_injection.default/beyla.ebpf.default"

	tests := []struct {
		name        string
		role        string
		componentID string
	}{
		{name: "short component ID", role: "otlp", componentID: "beyla.ebpf.default"},
		{name: "long remotecfg component ID", role: "otlp", componentID: longID},
		{name: "long remotecfg component ID health", role: "health", componentID: longID},
		{name: "pathologically long component ID", role: "health", componentID: strings.Repeat("x", 500)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr := abstractSocketAddr(tc.role, tc.componentID)

			require.True(t, strings.HasPrefix(addr, "@alloy-beyla-"+tc.role+"-"))
			require.LessOrEqual(t, len(addr), unixPathMax)

			lis, err := net.Listen("unix", addr)
			require.NoError(t, err)
			require.NoError(t, lis.Close())
		})
	}
}

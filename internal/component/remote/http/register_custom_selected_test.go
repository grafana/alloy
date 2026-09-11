//go:build alloy_custom_components && alloy_component_remote_http

package http

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildSelectsRemoteHTTPRegistration(t *testing.T) {
	reg, exists := component.Get("remote.http")
	require.True(t, exists)
	require.Equal(t, "remote.http", reg.Name)
}

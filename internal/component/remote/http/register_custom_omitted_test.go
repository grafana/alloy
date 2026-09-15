//go:build alloy_custom_components && !alloy_component_remote_http

package http

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildOmitsRemoteHTTPRegistration(t *testing.T) {
	_, exists := component.Get("remote.http")
	require.False(t, exists)
}

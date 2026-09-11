//go:build alloy_custom_components && alloy_component_discovery_aws && !alloy_component_discovery_ec2 && !alloy_component_discovery_lightsail

package all

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildSelectsOneRegistrationFromSharedPackage(t *testing.T) {
	_, exists := component.Get("discovery.aws")
	require.True(t, exists)

	for _, name := range []string{"discovery.ec2", "discovery.lightsail"} {
		_, exists := component.Get(name)
		require.False(t, exists, "did not expect %q registration", name)
	}
}

package util

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func Test_FeatureGates(t *testing.T) {
	reg := featuregate.GlobalRegistry()

	fgSet := make(map[string]bool)

	for _, fg := range otelFeatureGates {
		fgSet[fg.name] = fg.enabled
	}

	reg.VisitAll(func(g *featuregate.Gate) {
		requiredVal, ok := fgSet[g.ID()]
		if !ok {
			return
		}
		// Make sure that the feature gate is not already at the required value before touching it.
		// There is no point in Alloy setting a feature gate if it's already at the desired state.
		// This "require" check will fail if the Collector was upgraded and
		// a feature gate was promoted from alpha to beta.
		errMsg := "feature gate %s is already set to the required value - should it be removed from Alloy?"
		require.Equal(t, !requiredVal, g.IsEnabled(), errMsg, g.ID())
	})

	require.NoError(t, SetupOtelFeatureGates())

	reg.VisitAll(func(g *featuregate.Gate) {
		requiredVal, ok := fgSet[g.ID()]
		if !ok {
			return
		}
		// Make sure that Alloy set the gate to the desired value.
		require.Equal(t, requiredVal, g.IsEnabled())
	})
}

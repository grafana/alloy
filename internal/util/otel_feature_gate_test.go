package util

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func Test_FeatureGates(t *testing.T) {
	reg := featuregate.GlobalRegistry()

	fgSet := make(map[string]struct{})

	for _, fg := range otelFeatureGates {
		fgSet[fg] = struct{}{}
	}

	reg.VisitAll(func(g *featuregate.Gate) {
		if _, ok := fgSet[g.ID()]; !ok {
			return
		}
		// Make sure that the feature gate is disabled before touching it.
		// There is no point in Alloy enabling a feature gate
		// if it's already enabled in the Collector.
		// This "require" check will fail if the Collector was upgraded and
		// a feature gate was promoted from alpha to beta.
		require.Falsef(t, g.IsEnabled(), "feature gate %s is enabled - should it be removed from Alloy?", g.ID())
	})

	require.NoError(t, SetupOtelFeatureGates())

	reg.VisitAll(func(g *featuregate.Gate) {
		if _, ok := fgSet[g.ID()]; !ok {
			return
		}
		// Make sure that Alloy enabled the gate.
		require.True(t, g.IsEnabled())
	})
}

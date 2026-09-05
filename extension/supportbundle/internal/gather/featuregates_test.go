package gather

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func TestFeatureGatesGatherer(t *testing.T) {
	const id = "supportbundle.testgate"
	// The global registry has no unregister path, so tolerate a prior
	// registration to keep this test repeatable (for example under -count=2).
	_, err := featuregate.GlobalRegistry().Register(id, featuregate.StageAlpha,
		featuregate.WithRegisterDescription("test gate for the support bundle"))
	if err != nil && !errors.Is(err, featuregate.ErrAlreadyRegistered) {
		require.NoError(t, err)
	}

	files, err := FeatureGates{}.Gather(context.Background(), Options{})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "feature-gates.txt")
	// An alpha gate is disabled by default.
	require.Contains(t, string(m["feature-gates.txt"]), id+"=false")
}

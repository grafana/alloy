package otelcol_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/stretchr/testify/require"
)

func TestConsumerArgumentsStability(t *testing.T) {
	for _, stability := range []featuregate.Stability{
		featuregate.StabilityGenerallyAvailable, featuregate.StabilityPublicPreview, featuregate.StabilityExperimental,
	} {
		t.Run(stability.String(), func(t *testing.T) {
			args := otelcol.ConsumerArguments{}
			require.NoError(t, args.ValidateStabilityLevel(stability))
			args.Profiles = []otelcol.Consumer{nil}
			err := args.ValidateStabilityLevel(stability)
			if stability == featuregate.StabilityExperimental {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "profiles output")
				require.ErrorContains(t, err, "--stability.level")
			}
		})
	}
}

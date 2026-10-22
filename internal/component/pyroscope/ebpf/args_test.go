package ebpf

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAggregateProfilesArguments(t *testing.T) {
	for _, tc := range []struct {
		config    string
		aggregate bool
		invalid   bool
	}{
		{config: "forward_to = []"},
		{config: "forward_to = []\naggregate_profiles = true", aggregate: true},
		{config: "forward_to = []\naggregate_profiles = true\npid_label = false", aggregate: true},
		{config: "forward_to = []\naggregate_profiles = true\npid_label = true", invalid: true},
		{config: "forward_to = []\naggregate_profiles = false\npid_label = true"},
	} {
		t.Run(tc.config, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.config), &args)
			if tc.invalid {
				require.ErrorContains(t, err, "aggregate_profiles requires pid_label to be false")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.aggregate, args.AggregateProfiles)
		})
	}
}

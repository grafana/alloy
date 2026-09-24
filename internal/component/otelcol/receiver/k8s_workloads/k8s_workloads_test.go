package k8s_workloads

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestArgumentsUnmarshal(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		cluster_name = "production-eu"
		cluster_uid  = "cluster-uid"

		clustering {
			enabled = true
		}

		output {}
	`), &args))

	require.Equal(t, "production-eu", args.ClusterName)
	require.Equal(t, "cluster-uid", args.ClusterUID)
	require.True(t, args.Clustering.Enabled)
	require.NotNil(t, args.Output)
}

func TestArgumentsRequireOutput(t *testing.T) {
	var args Arguments
	err := syntax.Unmarshal(nil, &args)
	require.ErrorContains(t, err, `missing required block "output"`)
}

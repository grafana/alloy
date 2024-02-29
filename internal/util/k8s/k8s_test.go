//go:build !nonetwork && !nodocker && !race

package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCluster(t *testing.T) {
	// TODO: this is broken with go 1.20.6
	// waiting on https://github.com/testcontainers/testcontainers-go/issues/1359
	t.Skip()
	ctx := context.Background()

	cluster, err := NewCluster(ctx, Options{})
	require.NoError(t, err)
	defer cluster.Stop()

	cli, err := client.New(cluster.GetConfig(), client.Options{})
	require.NoError(t, err)

	var nss core.NamespaceList
	require.NoError(t, cli.List(ctx, &nss))

	names := make([]string, len(nss.Items))
	for i, ns := range nss.Items {
		names[i] = ns.Name
	}
	require.Contains(t, names, "kube-system")
}

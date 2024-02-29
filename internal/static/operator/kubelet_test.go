//go:build !nonetwork && !nodocker && !race

package operator

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/operator/logutil"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/agent/internal/util/k8s"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	clog "sigs.k8s.io/controller-runtime/pkg/log"
)

// TestKubelet tests the Kubelet reconciler.
func TestKubelet(t *testing.T) {
	// TODO: this is broken with go 1.20.6
	// waiting on https://github.com/testcontainers/testcontainers-go/issues/1359
	t.Skip()
	l := util.TestLogger(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ctx = clog.IntoContext(ctx, logutil.Wrap(l))

	cluster, err := k8s.NewCluster(ctx, k8s.Options{})
	require.NoError(t, err)
	defer cluster.Stop()

	cli := cluster.Client()

	nodes := []core_v1.Node{
		{
			ObjectMeta: meta_v1.ObjectMeta{Name: "node-a"},
			Status: core_v1.NodeStatus{
				Addresses: []core_v1.NodeAddress{
					{Type: core_v1.NodeInternalIP, Address: "10.0.0.10"},
				},
			},
		},
		{
			ObjectMeta: meta_v1.ObjectMeta{Name: "node-b"},
			Status: core_v1.NodeStatus{
				Addresses: []core_v1.NodeAddress{
					{Type: core_v1.NodeExternalIP, Address: "10.24.0.11"},
				},
			},
		},
		{
			ObjectMeta: meta_v1.ObjectMeta{Name: "node-c"},
			Status: core_v1.NodeStatus{
				Addresses: []core_v1.NodeAddress{
					{Type: core_v1.NodeExternalIP, Address: "10.24.0.12"},
					{Type: core_v1.NodeInternalIP, Address: "10.0.0.12"},
				},
			},
		},
	}

	for _, n := range nodes {
		err := cli.Create(ctx, &n)
		require.NoError(t, err)
	}

	ns := &core_v1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{Name: "kube-system"},
	}
	_ = cli.Create(ctx, ns)

	r := &kubeletReconciler{
		Client:           cli,
		kubeletNamespace: "kube-system",
		kubeletName:      "kubelet",
	}
	_, err = r.Reconcile(ctx, reconcile.Request{})
	require.NoError(t, err)

	var (
		eps core_v1.Endpoints
		svc core_v1.Service

		key = types.NamespacedName{Namespace: r.kubeletNamespace, Name: r.kubeletName}
	)
	require.NoError(t, cli.Get(ctx, key, &eps))
	require.NoError(t, cli.Get(ctx, key, &svc))

	require.Len(t, eps.Subsets, 1)

	expect := map[string]string{
		"node-a": "10.0.0.10",
		"node-b": "10.24.0.11",

		// When a node has internal and external IPs, use internal first.
		"node-c": "10.0.0.12",
	}
	for nodeName, expectIP := range expect {
		var epa *core_v1.EndpointAddress

		for _, addr := range eps.Subsets[0].Addresses {
			if addr.TargetRef.Name == nodeName {
				epa = &addr
				break
			}
		}

		require.NotNilf(t, epa, "did not find endpoint address for node %s", nodeName)
		require.Equalf(t, expectIP, epa.IP, "node %s had incorrect ip address", nodeName)
	}
}

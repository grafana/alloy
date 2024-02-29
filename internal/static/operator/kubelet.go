package operator

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/static/operator/clientutil"
	"github.com/grafana/agent/internal/static/operator/logutil"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controller "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kubeletReconciler struct {
	client.Client
	kubeletNamespace, kubeletName string
}

func (r *kubeletReconciler) Reconcile(ctx context.Context, req controller.Request) (res controller.Result, err error) {
	l := logutil.FromContext(ctx)
	level.Info(l).Log("msg", "reconciling node")

	var nodes core_v1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		level.Error(l).Log("msg", "failed to list nodes for kubelet service", "err", err)
		return res, fmt.Errorf("unable to list nodes: %w", err)
	}
	nodeAddrs, err := getNodeAddrs(l, &nodes)
	if err != nil {
		level.Error(l).Log("msg", "could not get addresses from all nodes", "err", err)
		return res, fmt.Errorf("unable to get addresses from nodes: %w", err)
	}

	labels := mergeMaps(managedByOperatorLabels, map[string]string{
		// Labels taken from prometheus-operator:
		// https://github.com/prometheus-operator/prometheus-operator/blob/2c81b0cf6a5673e08057499a08ddce396b19dda4/pkg/prometheus/operator.go#L586-L587
		"k8s-app":                "kubelet",
		"app.kubernetes.io/name": "kubelet",
	})

	svc := &core_v1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      r.kubeletName,
			Namespace: r.kubeletNamespace,
			Labels:    labels,
		},
		Spec: core_v1.ServiceSpec{
			Type:      core_v1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Ports: []core_v1.ServicePort{
				{Name: "https-metrics", Port: 10250},
				{Name: "http-metrics", Port: 10255},
				{Name: "cadvisor", Port: 4194},
			},
		},
	}

	eps := &core_v1.Endpoints{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      r.kubeletName,
			Namespace: r.kubeletNamespace,
			Labels:    labels,
		},
		Subsets: []core_v1.EndpointSubset{{
			Addresses: nodeAddrs,
			Ports: []core_v1.EndpointPort{
				// Taken from https://github.com/prometheus-operator/prometheus-operator/blob/2c81b0cf6a5673e08057499a08ddce396b19dda4/pkg/prometheus/operator.go#L593
				{Name: "https-metrics", Port: 10250},
				{Name: "http-metrics", Port: 10255},
				{Name: "cadvisor", Port: 4194},
			},
		}},
	}

	level.Debug(l).Log("msg", "reconciling kubelet service", "svc", client.ObjectKeyFromObject(svc))
	err = clientutil.CreateOrUpdateService(ctx, r.Client, svc)
	if err != nil {
		return res, fmt.Errorf("failed to reconcile kubelet service %s: %w", client.ObjectKeyFromObject(svc), err)
	}

	level.Debug(l).Log("msg", "reconciling kubelet endpoints", "eps", client.ObjectKeyFromObject(eps))
	err = clientutil.CreateOrUpdateEndpoints(ctx, r.Client, eps)
	if err != nil {
		return res, fmt.Errorf("failed to reconcile kubelet endpoints %s: %w", client.ObjectKeyFromObject(eps), err)
	}

	return
}

// mergeMaps merges the contents of b with a. Keys from b take precedence.
func mergeMaps(a, b map[string]string) map[string]string {
	res := make(map[string]string)
	for k, v := range a {
		res[k] = v
	}
	for k, v := range b {
		res[k] = v
	}
	return res
}

func getNodeAddrs(l log.Logger, nodes *core_v1.NodeList) (addrs []core_v1.EndpointAddress, err error) {
	var failed bool

	for _, n := range nodes.Items {
		addr, err := nodeAddress(n)
		if err != nil {
			level.Error(l).Log("msg", "failed to get address from node", "node", n.Name, "err", err)
			failed = true
		}

		addrs = append(addrs, core_v1.EndpointAddress{
			IP: addr,
			TargetRef: &core_v1.ObjectReference{
				Kind:       n.Kind,
				APIVersion: n.APIVersion,
				Name:       n.Name,
				UID:        n.UID,
			},
		})
	}

	if failed {
		return nil, fmt.Errorf("failed to get the address from one or more nodes")
	}

	// Sort endpoints to reduce performance cost on endpoint watchers
	sort.SliceStable(addrs, func(i, j int) bool {
		return addrs[i].IP < addrs[j].IP
	})

	return
}

// nodeAddresses returns the provided node's address, based on the priority:
//
// 1. NodeInternalIP
// 2. NodeExternalIP
//
// Copied from github.com/prometheus/prometheus/discovery/kubernetes/node.go
func nodeAddress(node core_v1.Node) (string, error) {
	m := map[core_v1.NodeAddressType][]string{}
	for _, a := range node.Status.Addresses {
		m[a.Type] = append(m[a.Type], a.Address)
	}

	if addresses, ok := m[core_v1.NodeInternalIP]; ok {
		return addresses[0], nil
	}
	if addresses, ok := m[core_v1.NodeExternalIP]; ok {
		return addresses[0], nil
	}
	return "", fmt.Errorf("host address unknown")
}

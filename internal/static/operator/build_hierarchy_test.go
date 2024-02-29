//go:build !nonetwork && !nodocker && !race

package operator

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/hierarchy"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/agent/internal/util/k8s"
	"github.com/grafana/agent/internal/util/structwalk"
	prom "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	controller "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Test_buildHierarchy checks that an entire resource hierarchy can be
// discovered.
func Test_buildHierarchy(t *testing.T) {
	// TODO: this is broken with go 1.20.6
	// waiting on https://github.com/testcontainers/testcontainers-go/issues/1359
	t.Skip()
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	l := util.TestLogger(t)
	cluster := NewTestCluster(ctx, t, l)
	cli := newTestControllerClient(t, cluster)

	resources := k8s.NewResourceSet(l, cluster)
	defer resources.Stop()
	require.NoError(t, resources.AddFile(ctx, "./testdata/test-resource-hierarchy.yaml"))

	// Get root resource
	var root gragent.GrafanaAgent
	err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"}, &root)
	require.NoError(t, err)

	deployment, watchers, err := buildHierarchy(ctx, l, cli, &root)
	require.NoError(t, err)

	// Check resources in hierarchy
	{
		expectedResources := []string{
			"GrafanaAgent/grafana-agent-example",
			"MetricsInstance/primary",
			"Integration/node-exporter",
			"LogsInstance/primary",
			"PodMonitor/grafana-agents",
			"PodLogs/grafana-agents",
		}
		var gotResources []string
		structwalk.Walk(&resourceWalker{
			onResource: func(c client.Object) {
				gvk, _ := apiutil.GVKForObject(c, cli.Scheme())

				key := fmt.Sprintf("%s/%s", gvk.Kind, c.GetName())
				gotResources = append(gotResources, key)
			},
		}, deployment)

		require.ElementsMatch(t, expectedResources, gotResources)
	}

	// Check secrets
	{
		expectedSecrets := []string{
			"/secrets/default/prometheus-fake-credentials/fakeUsername",
			"/secrets/default/prometheus-fake-credentials/fakePassword",
		}
		var actualSecrets []string
		for key := range deployment.Secrets {
			actualSecrets = append(actualSecrets, string(key))
		}

		require.ElementsMatch(t, expectedSecrets, actualSecrets)
	}

	// Check configured watchers
	{
		expectedWatchers := []hierarchy.Watcher{
			{
				Object: &gragent.MetricsInstance{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.LabelsSelector{
					NamespaceName: "default",
					Labels:        labels.SelectorFromSet(labels.Set{"agent": "grafana-agent-example"}),
				},
			},
			{
				Object: &gragent.LogsInstance{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.LabelsSelector{
					NamespaceName: "default",
					Labels:        labels.SelectorFromSet(labels.Set{"agent": "grafana-agent-example"}),
				},
			},
			{
				Object: &gragent.Integration{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.LabelsSelector{
					NamespaceName: "default",
					Labels:        labels.SelectorFromSet(labels.Set{"agent": "grafana-agent-example"}),
				},
			},
			{
				Object: &prom.ServiceMonitor{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.LabelsSelector{
					NamespaceName:   "default",
					NamespaceLabels: labels.Everything(),
					Labels:          labels.SelectorFromSet(labels.Set{"instance": "primary"}),
				},
			},
			{
				Object: &prom.PodMonitor{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.LabelsSelector{
					NamespaceName:   "default",
					NamespaceLabels: labels.Everything(),
					Labels:          labels.SelectorFromSet(labels.Set{"instance": "primary"}),
				},
			},
			{
				Object: &prom.Probe{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.LabelsSelector{
					NamespaceName: "default",
					Labels:        labels.Nothing(),
				},
			},
			{
				Object: &gragent.PodLogs{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.LabelsSelector{
					NamespaceName:   "default",
					NamespaceLabels: labels.Everything(),
					Labels:          labels.SelectorFromSet(labels.Set{"instance": "primary"}),
				},
			},
			{
				Object: &v1.Secret{},
				Owner:  client.ObjectKey{Namespace: "default", Name: "grafana-agent-example"},
				Selector: &hierarchy.KeySelector{
					Namespace: "default",
					Name:      "prometheus-fake-credentials",
				},
			},
		}
		require.ElementsMatch(t, expectedWatchers, watchers)
	}
}

type resourceWalker struct {
	onResource func(c client.Object)
}

func (w *resourceWalker) Visit(v interface{}) (next structwalk.Visitor) {
	if v == nil {
		return nil
	}
	if obj, ok := v.(client.Object); ok {
		w.onResource(obj)
	}
	return w
}

// newTestControllerClient creates a Kubernetes client which uses a cache and
// index for retrieving objects. This more closely matches the behavior of the
// operator instead of using cluster.Client, which lacks a cache and always
// communicates directly with Kubernetes.
func newTestControllerClient(t *testing.T, cluster *k8s.Cluster) client.Client {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr, err := controller.NewManager(cluster.GetConfig(), manager.Options{
		Scheme: cluster.Client().Scheme(),
	})
	require.NoError(t, err)

	go func() {
		require.NoError(t, mgr.Start(ctx))
	}()
	require.True(t, mgr.GetCache().WaitForCacheSync(ctx))

	return mgr.GetClient()
}

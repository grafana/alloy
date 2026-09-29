package k8s_workloads

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
)

func TestPodPaginationAndFailure(t *testing.T) {
	for _, failSecond := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "failed page"}[failSecond], func(t *testing.T) {
			ns, d, rs, pod := fixture()
			c, sink, client := testController(t, ns, d, rs)
			calls := 0
			client.PrependReactor("list", "pods", func(a ktesting.Action) (bool, runtime.Object, error) {
				calls++
				opts := a.(ktesting.ListActionImpl).GetListOptions()
				require.Positive(t, opts.Limit)
				if opts.Continue == "" {
					return true, &corev1.PodList{ListMeta: metav1.ListMeta{Continue: "page2"}, Items: []corev1.Pod{*pod}}, nil
				}
				require.Equal(t, "page2", opts.Continue)
				if failSecond {
					return true, nil, errors.New("page failed")
				}
				second := pod.DeepCopy()
				second.Status.ContainerStatuses[0].ImageID = "containerd://sha256:efgh"
				return true, &corev1.PodList{Items: []corev1.Pod{*second}}, nil
			})
			c.collectDeployments(t.Context(), ns)
			require.Equal(t, 2, calls)
			require.Len(t, sink.records(), 1)
			if failSecond {
				require.Equal(t, errorEventName, sink.records()[0].EventName())
				return
			}
			entries := body(t, sink.records()[0])["deployments"].([]any)
			containers := entries[0].(map[string]any)["containers"].([]any)
			require.Len(t, containers[0].(map[string]any)["resolved"], 2)
		})
	}
}

func TestEmptyNamespaceSkipsEnrichment(t *testing.T) {
	ns, _, _, _ := fixture()
	c, sink, client := testController(t, ns)
	for _, resource := range []string{"pods", "replicasets"} {
		client.PrependReactor("list", resource, func(ktesting.Action) (bool, runtime.Object, error) {
			t.Error("empty namespace should not load enrichment")
			return true, nil, errors.New("unexpected list")
		})
	}
	c.collectDeployments(t.Context(), ns)
	require.Len(t, sink.records(), 1)
	require.Equal(t, []any{}, body(t, sink.records()[0])["deployments"])
}

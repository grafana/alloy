package k8s_workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Exercise real HTTP decoding: fake client deep copies share string backing
// storage and hide the allocation cost of large Kubernetes API responses.
func BenchmarkCollectLargeNamespace(b *testing.B) {
	ns, deployment, rs, pod := fixture()
	pods := make([]corev1.Pod, 1000)
	for i := range pods {
		pods[i] = *pod.DeepCopy()
		pods[i].Name = fmt.Sprintf("pod-%d", i)
		pods[i].Annotations = map[string]string{"large-annotation": strings.Repeat("x", 32*1024)}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body any
		switch r.URL.Path {
		case "/apis/apps/v1/namespaces/demo/deployments":
			body = map[string]any{"apiVersion": "apps/v1", "kind": "DeploymentList", "metadata": map[string]string{"resourceVersion": "1"}, "items": []any{deployment}}
		case "/apis/apps/v1/namespaces/demo/replicasets":
			body = map[string]any{"apiVersion": "apps/v1", "kind": "ReplicaSetList", "items": []any{rs}}
		case "/api/v1/namespaces/demo/pods":
			start, _ := strconv.Atoi(r.URL.Query().Get("continue"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			end := len(pods)
			if limit > 0 && start+limit < end {
				end = start + limit
			}
			meta := map[string]string{"resourceVersion": "1"}
			if end < len(pods) {
				meta["continue"] = strconv.Itoa(end)
			}
			body = map[string]any{"apiVersion": "v1", "kind": "PodList", "metadata": meta, "items": pods[start:end]}
		case "/api/v1/namespaces/demo":
			body = ns
		default:
			http.NotFound(w, r)
			return
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			return
		}
	}))
	defer server.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL, QPS: 10000, Burst: 10000})
	if err != nil {
		b.Fatal(err)
	}
	c := newController(controllerOptions{client: client, clusterUID: "cluster", emit: func(context.Context, func() eventBatch) error { return nil }})
	defer c.queue.ShutDown()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.collectDeployments(b.Context(), ns)
	}
}

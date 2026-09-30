package otelcolreceiverk8sworkloads

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

const testNamespace = "test-k8s-workloads"
const alloyRelease = "alloy-test-k8s-workloads"
const workloadNamespace = "test-k8s-workloads-rollouts"

type rolloutEvent struct {
	name, namespace, pod string
	timestamp            uint64
	body                 map[string]any
}

func TestK8sWorkloads(t *testing.T) {
	namespace := deps.NewNamespace(deps.NamespaceOptions{Name: testNamespace, Labels: map[string]string{"alloy-integration-test": "true"}})
	alloy := deps.NewAlloy(deps.AlloyOptions{Namespace: namespace.Name(), Release: alloyRelease, ConfigPath: "./config/config.alloy", ValuesPath: "./config/alloy-values.yaml"})
	harness.Setup(t, harness.Options{Dependencies: []harness.Dependency{namespace, alloy}})
	require.Eventually(t, func() bool {
		logs, err := harness.RunCommandOutput("kubectl", "-n", testNamespace, "logs", "-l", "app.kubernetes.io/instance="+alloyRelease, "-c", "alloy", "--tail=1000")
		return err == nil && strings.Contains(logs, "Kubernetes Deployment rollout watcher ready")
	}, 2*time.Minute, time.Second)
	apply(t, map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": workloadNamespace}})
	t.Cleanup(func() {
		_ = harness.RunCommand("kubectl", "delete", "namespace", workloadNamespace, "--ignore-not-found", "--wait=false")
	})
	apply(t, deployment("web", nil))
	first := waitEvent(t, func(e rolloutEvent) bool {
		return e.namespace == workloadNamespace && e.name == "deployment.rollout.started" && e.body["name"] == "web"
	})
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "deployment.rollout.succeeded" && e.body["rollout_id"] == first.body["rollout_id"]
	})
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "scale", "deployment/web", "--replicas=2"))
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "rollout", "status", "deployment/web", "--timeout=90s"))
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "set", "image", "deployment/web", "app=unavailable.invalid/app:test"))
	broken := waitEvent(t, func(e rolloutEvent) bool {
		return e.namespace == workloadNamespace && e.name == "deployment.rollout.started" && e.body["rollout_id"] != first.body["rollout_id"]
	})
	require.Equal(t, []any{"image"}, broken.body["change_types"])
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "deployment.rollout.stalled" && e.body["rollout_id"] == broken.body["rollout_id"]
	})
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "rollout", "undo", "deployment/web"))
	superseded := waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "deployment.rollout.superseded" && e.body["rollout_id"] == broken.body["rollout_id"]
	})
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "deployment.rollout.succeeded" && e.body["rollout_id"] == superseded.body["superseded_by"]
	})
	events, err := readEvents()
	require.NoError(t, err)
	owners := map[string]bool{}
	counts := map[string]int{}
	for _, e := range events {
		require.Contains(t, []string{"deployment.rollout.started", "deployment.rollout.succeeded", "deployment.rollout.stalled", "deployment.rollout.superseded"}, e.name)
		if e.namespace == workloadNamespace {
			owners[e.pod] = true
			counts[e.name]++
		}
	}
	require.Len(t, owners, 1)
	require.Equal(t, map[string]int{"deployment.rollout.started": 3, "deployment.rollout.succeeded": 2, "deployment.rollout.stalled": 1, "deployment.rollout.superseded": 1}, counts)
}
func deployment(name string, labels map[string]string) map[string]any {
	return map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": name, "namespace": workloadNamespace, "labels": labels}, "spec": map[string]any{"replicas": 1, "progressDeadlineSeconds": 10, "selector": map[string]any{"matchLabels": map[string]string{"app": name}}, "template": map[string]any{"metadata": map[string]any{"labels": map[string]string{"app": name}}, "spec": map[string]any{"terminationGracePeriodSeconds": 1, "containers": []any{map[string]any{"name": "app", "image": "nginx:1.27-alpine"}}}}}}
}
func apply(t *testing.T, obj any) {
	t.Helper()
	data, err := json.Marshal(obj)
	require.NoError(t, err)
	require.NoError(t, harness.RunCommandStdin(string(data), "kubectl", "apply", "-f", "-"))
}
func waitEvent(t *testing.T, predicate func(rolloutEvent) bool) rolloutEvent {
	t.Helper()
	var found rolloutEvent
	require.Eventually(t, func() bool {
		events, err := readEvents()
		if err != nil {
			return false
		}
		for _, e := range events {
			if predicate(e) {
				found = e
				return true
			}
		}
		return false
	}, 2*time.Minute, time.Second)
	return found
}
func readEvents() ([]rolloutEvent, error) {
	output, err := harness.RunCommandOutput("kubectl", "-n", testNamespace, "get", "pods", "-l", "app.kubernetes.io/instance="+alloyRelease, "-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	if err != nil {
		return nil, err
	}
	events := []rolloutEvent{}
	for _, pod := range strings.Fields(output) {
		data, err := harness.RunCommandOutput("kubectl", "-n", testNamespace, "exec", pod, "-c", "alloy", "--", "sh", "-c", "if [ -f /tmp/k8s-workloads.json ]; then cat /tmp/k8s-workloads.json; fi")
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
			if line == "" {
				continue
			}
			logs, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs([]byte(line))
			if err != nil {
				return nil, err
			}
			for i := 0; i < logs.ResourceLogs().Len(); i++ {
				rl := logs.ResourceLogs().At(i)
				ns, _ := rl.Resource().Attributes().Get("k8s.namespace.name")
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					records := rl.ScopeLogs().At(j).LogRecords()
					for k := 0; k < records.Len(); k++ {
						r := records.At(k)
						var body map[string]any
						if err := json.Unmarshal([]byte(r.Body().Str()), &body); err != nil {
							return nil, err
						}
						name := strings.TrimPrefix(r.EventName(), "grafana.sdlc.k8s.")
						name = strings.TrimPrefix(name, "grafana.sdlc.")
						events = append(events, rolloutEvent{name: name, namespace: ns.Str(), pod: pod, timestamp: uint64(r.Timestamp()), body: body})
					}
				}
			}
		}
	}
	return events, nil
}

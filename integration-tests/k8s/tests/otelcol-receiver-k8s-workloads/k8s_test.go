package otelcolreceiverk8sworkloads

import (
	"encoding/json"
	"fmt"
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
const inventoryNamespace = "test-k8s-workloads-inventory"

type inventoryEvent struct {
	name, namespace, pod string
	timestamp            uint64
	body                 map[string]any
}

func TestK8sWorkloads(t *testing.T) {
	namespace := deps.NewNamespace(deps.NamespaceOptions{Name: testNamespace, Labels: map[string]string{"alloy-integration-test": "true"}})
	alloy := deps.NewAlloy(deps.AlloyOptions{Namespace: namespace.Name(), Release: alloyRelease, ConfigPath: "./config/config.alloy", ValuesPath: "./config/alloy-values.yaml"})
	harness.Setup(t, harness.Options{Dependencies: []harness.Dependency{namespace, alloy}})
	waitEvent(t, func(e inventoryEvent) bool { return e.name == "namespace.snapshot" })
	apply(t, map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": inventoryNamespace}})
	t.Cleanup(func() {
		_ = harness.RunCommand("kubectl", "delete", "namespace", inventoryNamespace, "--ignore-not-found", "--wait=false")
	})
	waitEvent(t, func(e inventoryEvent) bool { return e.name == "namespace.created" && e.namespace == inventoryNamespace })
	waitEvent(t, func(e inventoryEvent) bool {
		return e.name == "deployment.snapshot" && e.namespace == inventoryNamespace && len(e.body["deployments"].([]any)) == 0
	})

	t.Run("snapshot status and image enrichment", func(t *testing.T) {
		apply(t, deployment("web", nil))
		waitEvent(t, func(e inventoryEvent) bool {
			return e.name == "deployment.created" && e.body["name"] == "web" && e.namespace == inventoryNamespace
		})
		e := waitEvent(t, func(e inventoryEvent) bool {
			d := findDeployment(e, "web")
			return d != nil && d["status"] == "healthy" && len(d["containers"].([]any)[0].(map[string]any)["resolved"].([]any)) > 0
		})
		waitEvent(t, func(e inventoryEvent) bool {
			return e.name == "deployment.succeeded" && e.body["name"] == "web" && e.namespace == inventoryNamespace
		})
		d := findDeployment(e, "web")
		require.NotEmpty(t, d["uid"])
		require.Equal(t, true, e.body["complete"])
		image := d["containers"].([]any)[0].(map[string]any)["resolved"].([]any)[0].(map[string]any)
		require.Contains(t, image["id"], "sha256:")
		require.NotEmpty(t, image["digest"])
		first := e.timestamp
		waitEvent(t, func(e inventoryEvent) bool { return findDeployment(e, "web") != nil && e.timestamp > first })
	})
	t.Run("single clustered collector and no replica event spam", func(t *testing.T) {
		require.NoError(t, harness.RunCommand("kubectl", "-n", inventoryNamespace, "scale", "deployment/web", "--replicas=2"))
		waitEvent(t, func(e inventoryEvent) bool {
			d := findDeployment(e, "web")
			return d != nil && d["ready_replicas"] == float64(2) && d["status"] == "healthy"
		})
		events, err := readEvents()
		require.NoError(t, err)
		owners := map[string]bool{}
		created, succeeded := 0, 0
		for _, e := range events {
			require.NotContains(t, e.name, "rollout")
			require.NotContains(t, e.name, "observed")
			if e.name == "deployment.created" && e.body["name"] == "web" && e.namespace == inventoryNamespace {
				created++
			}
			if e.name == "deployment.succeeded" && e.body["name"] == "web" && e.namespace == inventoryNamespace {
				succeeded++
			}
			if e.name == "deployment.snapshot" && e.namespace == inventoryNamespace {
				owners[e.pod] = true
			}
		}
		require.Equal(t, 1, created)
		require.Equal(t, 1, succeeded, "scaling must not repeat success")
		require.Len(t, owners, 1)
	})
	t.Run("stalled notification and snapshot", func(t *testing.T) {
		require.NoError(t, harness.RunCommand("kubectl", "-n", inventoryNamespace, "set", "image", "deployment/web", "app=unavailable.invalid/app:test"))
		waitEvent(t, func(e inventoryEvent) bool {
			d := findDeployment(e, "web")
			return d != nil && d["status"] == "stalled"
		})
		waitEvent(t, func(e inventoryEvent) bool {
			return e.name == "deployment.stalled" && e.body["name"] == "web" && e.namespace == inventoryNamespace
		})
	})
	t.Run("deletion and complete empty inventory", func(t *testing.T) {
		since := uint64(time.Now().UnixNano())
		require.NoError(t, harness.RunCommand("kubectl", "-n", inventoryNamespace, "delete", "deployment/web", "--wait=true", "--timeout=90s"))
		waitEvent(t, func(e inventoryEvent) bool {
			return e.name == "deployment.deleted" && e.namespace == inventoryNamespace && e.body["name"] == "web"
		})
		waitEvent(t, func(e inventoryEvent) bool {
			return e.timestamp > since && e.name == "deployment.snapshot" && e.namespace == inventoryNamespace && len(e.body["deployments"].([]any)) == 0
		})
	})
	t.Run("oversize snapshot is a bounded error", func(t *testing.T) {
		labels := map[string]string{}
		for i := 0; i < 220; i++ {
			labels[fmt.Sprintf("label-%03d", i)] = strings.Repeat("a", 50)
		}
		apply(t, deployment("oversize", labels))
		waitEvent(t, func(e inventoryEvent) bool {
			return e.name == "reporting.error" && e.namespace == inventoryNamespace && e.body["reason"] == "payload_too_large" && e.body["operation"] == "snapshot"
		})
		events, err := readEvents()
		require.NoError(t, err)
		for _, e := range events {
			require.Nil(t, findDeployment(e, "oversize"), "oversized snapshot must not be truncated or published")
		}
	})
	t.Run("namespace deletion recovers through cluster inventory", func(t *testing.T) {
		since := uint64(time.Now().UnixNano())
		require.NoError(t, harness.RunCommand("kubectl", "delete", "namespace", inventoryNamespace, "--wait=true", "--timeout=90s"))
		waitEvent(t, func(e inventoryEvent) bool { return e.name == "namespace.deleted" && e.namespace == inventoryNamespace })
		waitEvent(t, func(e inventoryEvent) bool {
			if e.name != "namespace.snapshot" || e.timestamp <= since {
				return false
			}
			for _, ns := range e.body["namespaces"].([]any) {
				if ns.(map[string]any)["name"] == inventoryNamespace {
					return false
				}
			}
			return true
		})
	})
}
func deployment(name string, labels map[string]string) map[string]any {
	return map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": name, "namespace": inventoryNamespace, "labels": labels}, "spec": map[string]any{"replicas": 1, "progressDeadlineSeconds": 10, "selector": map[string]any{"matchLabels": map[string]string{"app": name}}, "template": map[string]any{"metadata": map[string]any{"labels": map[string]string{"app": name}}, "spec": map[string]any{"terminationGracePeriodSeconds": 1, "containers": []any{map[string]any{"name": "app", "image": "nginx:1.27-alpine"}}}}}}
}
func apply(t *testing.T, obj any) {
	t.Helper()
	data, err := json.Marshal(obj)
	require.NoError(t, err)
	require.NoError(t, harness.RunCommandStdin(string(data), "kubectl", "apply", "-f", "-"))
}
func findDeployment(e inventoryEvent, name string) map[string]any {
	if e.name != "deployment.snapshot" || e.namespace != inventoryNamespace {
		return nil
	}
	for _, item := range e.body["deployments"].([]any) {
		d := item.(map[string]any)
		if d["name"] == name {
			return d
		}
	}
	return nil
}
func waitEvent(t *testing.T, predicate func(inventoryEvent) bool) inventoryEvent {
	t.Helper()
	var found inventoryEvent
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
func readEvents() ([]inventoryEvent, error) {
	output, err := harness.RunCommandOutput("kubectl", "-n", testNamespace, "get", "pods", "-l", "app.kubernetes.io/instance="+alloyRelease, "-o", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`)
	if err != nil {
		return nil, err
	}
	events := []inventoryEvent{}
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
						events = append(events, inventoryEvent{name: name, namespace: ns.Str(), pod: pod, timestamp: uint64(r.Timestamp()), body: body})
					}
				}
			}
		}
	}
	return events, nil
}

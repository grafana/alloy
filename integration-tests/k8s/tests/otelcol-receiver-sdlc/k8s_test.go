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

const testNamespace = "test-sdlc"
const alloyRelease = "alloy-test-sdlc"
const workloadNamespace = "test-sdlc-rollouts"

// rolloutEvent is one grafana.sdlc.change record. name is its
// provider.resource.change.type; body is the change.diff map.
type rolloutEvent struct {
	name, namespace, resourceType, resourceID, pod string
	timestamp                                      uint64
	body                                           map[string]any
	attrs                                          map[string]any
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
	added := waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "created" && e.resourceID == workloadNamespace+"/web"
	})
	require.Equal(t, "Created", added.attrs["grafana.sdlc.change.summary"])
	first := waitEvent(t, func(e rolloutEvent) bool {
		return e.namespace == workloadNamespace && e.name == "rollout_started" && e.resourceID == workloadNamespace+"/web"
	})
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_succeeded" && e.body["rollout_id"] == first.body["rollout_id"]
	})
	images := waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_images_resolved" && e.body["rollout_id"] == first.body["rollout_id"]
	})
	require.Equal(t, true, images.body["complete"])
	containers := images.body["resolved_containers"].([]any)
	require.NotEmpty(t, containers[0].(map[string]any)["runtime_image_ids"])
	require.Equal(t, "UNSET", images.attrs["grafana.sdlc.status"])
	require.Equal(t, "1", images.attrs["grafana.sdlc.schema.version"])
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "scale", "deployment/web", "--replicas=2"))
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "rollout", "status", "deployment/web", "--timeout=90s"))
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "set", "image", "deployment/web", "app=unavailable.invalid/app:test"))
	broken := waitEvent(t, func(e rolloutEvent) bool {
		return e.namespace == workloadNamespace && e.name == "rollout_started" && e.body["rollout_id"] != first.body["rollout_id"]
	})
	require.Equal(t, "intent", broken.attrs["grafana.sdlc.type"])
	require.Contains(t, broken.body["changes"], "containers.app.image")
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_stalled" && e.body["rollout_id"] == broken.body["rollout_id"]
	})
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "rollout", "undo", "deployment/web"))
	superseded := waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_superseded" && e.body["rollout_id"] == broken.body["rollout_id"]
	})
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_succeeded" && e.body["rollout_id"] == superseded.body["superseded_by"]
	})
	partial := waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_images_resolved" && e.body["rollout_id"] == broken.body["rollout_id"]
	})
	require.Equal(t, false, partial.body["complete"])
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_images_resolved" && e.body["rollout_id"] == superseded.body["superseded_by"]
	})
	events, err := readEvents()
	require.NoError(t, err)
	owners := map[string]bool{}
	counts := map[string]int{}
	for _, e := range events {
		require.Equal(t, "k8s", e.attrs["grafana.sdlc.provider.type"])
		if e.resourceType != "deployment" {
			continue
		}
		require.Contains(t, []string{"created", "rollout_started", "rollout_succeeded", "rollout_stalled", "rollout_superseded", "rollout_images_resolved"}, e.name)
		if e.namespace == workloadNamespace {
			owners[e.pod] = true
			counts[e.name]++
		}
	}
	require.Len(t, owners, 1)
	require.Equal(t, map[string]int{"created": 1, "rollout_started": 3, "rollout_succeeded": 2, "rollout_stalled": 1, "rollout_superseded": 1, "rollout_images_resolved": 3}, counts)

	const secretValue = "integration-secret-value"
	apply(t, map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "settings", "namespace": workloadNamespace}, "data": map[string]string{"LOG_LEVEL": "info"}})
	apply(t, map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": "creds", "namespace": workloadNamespace}, "stringData": map[string]string{"password": secretValue}})
	config := func(resourceType, name, changeType string) func(rolloutEvent) bool {
		return func(e rolloutEvent) bool {
			return e.resourceType == resourceType && e.resourceID == workloadNamespace+"/"+name && e.name == changeType
		}
	}
	waitEvent(t, config("configmap", "settings", "created"))
	waitEvent(t, config("secret", "creds", "created"))
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "patch", "configmap/settings", "--type=merge", "-p", `{"data":{"LOG_LEVEL":"debug"}}`))
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "patch", "secret/creds", "--type=merge", "-p", `{"stringData":{"password":"rotated-`+secretValue+`"}}`))
	updated := waitEvent(t, config("configmap", "settings", "updated"))
	require.Equal(t, "effect", updated.attrs["grafana.sdlc.type"])
	require.Equal(t, map[string]any{"changes": map[string]any{"data.LOG_LEVEL": map[string]any{"operation": "modified"}}}, updated.body)
	rotated := waitEvent(t, config("secret", "creds", "updated"))
	require.Equal(t, map[string]any{"changes": map[string]any{"data": map[string]any{"operation": "modified"}}}, rotated.body)
	events, err = readEvents()
	require.NoError(t, err)
	for _, e := range events {
		attrs, err := json.Marshal(e.attrs)
		require.NoError(t, err)
		require.NotContains(t, string(attrs), secretValue)
	}

	apply(t, statefulSet("db"))
	workload := func(resourceType, name, step string) func(rolloutEvent) bool {
		return func(e rolloutEvent) bool {
			return e.resourceType == resourceType && e.resourceID == workloadNamespace+"/"+name && e.name == step
		}
	}
	created := waitEvent(t, workload("statefulset", "db", "rollout_started"))
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_succeeded" && e.body["rollout_id"] == created.body["rollout_id"]
	})
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "set", "image", "statefulset/db", "app=nginx:1.28-alpine"))
	updatedSet := waitEvent(t, func(e rolloutEvent) bool {
		return e.resourceType == "statefulset" && e.name == "rollout_started" && e.body["rollout_id"] != created.body["rollout_id"]
	})
	require.Equal(t, "RollingUpdate", updatedSet.body["update_strategy"])
	require.Contains(t, updatedSet.body["changes"], "containers.app.image")
	require.Equal(t, created.attrs["grafana.sdlc.change.version.after"], updatedSet.attrs["grafana.sdlc.change.version.before"])
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_succeeded" && e.body["rollout_id"] == updatedSet.body["rollout_id"]
	})
	setImages := waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_images_resolved" && e.body["rollout_id"] == updatedSet.body["rollout_id"]
	})
	require.Equal(t, true, setImages.body["complete"])

	apply(t, daemonSet("agent"))
	createdDaemons := waitEvent(t, workload("daemonset", "agent", "rollout_started"))
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_succeeded" && e.body["rollout_id"] == createdDaemons.body["rollout_id"]
	})
	require.NoError(t, harness.RunCommand("kubectl", "-n", workloadNamespace, "set", "image", "daemonset/agent", "app=nginx:1.28-alpine"))
	updatedDaemons := waitEvent(t, func(e rolloutEvent) bool {
		return e.resourceType == "daemonset" && e.name == "rollout_started" && e.body["rollout_id"] != createdDaemons.body["rollout_id"]
	})
	require.Equal(t, "RollingUpdate", updatedDaemons.body["update_strategy"])
	require.Contains(t, updatedDaemons.body["changes"], "containers.app.image")
	require.Equal(t, createdDaemons.attrs["grafana.sdlc.change.version.after"], updatedDaemons.attrs["grafana.sdlc.change.version.before"])
	waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_succeeded" && e.body["rollout_id"] == updatedDaemons.body["rollout_id"]
	})
	daemonImages := waitEvent(t, func(e rolloutEvent) bool {
		return e.name == "rollout_images_resolved" && e.body["rollout_id"] == updatedDaemons.body["rollout_id"]
	})
	require.Equal(t, true, daemonImages.body["complete"])
}
func daemonSet(name string) map[string]any {
	return map[string]any{"apiVersion": "apps/v1", "kind": "DaemonSet", "metadata": map[string]any{"name": name, "namespace": workloadNamespace}, "spec": map[string]any{"selector": map[string]any{"matchLabels": map[string]string{"app": name}}, "template": map[string]any{"metadata": map[string]any{"labels": map[string]string{"app": name}}, "spec": map[string]any{"terminationGracePeriodSeconds": 1, "containers": []any{map[string]any{"name": "app", "image": "nginx:1.27-alpine"}}}}}}
}
func statefulSet(name string) map[string]any {
	return map[string]any{"apiVersion": "apps/v1", "kind": "StatefulSet", "metadata": map[string]any{"name": name, "namespace": workloadNamespace}, "spec": map[string]any{"replicas": 2, "serviceName": name, "selector": map[string]any{"matchLabels": map[string]string{"app": name}}, "template": map[string]any{"metadata": map[string]any{"labels": map[string]string{"app": name}}, "spec": map[string]any{"terminationGracePeriodSeconds": 1, "containers": []any{map[string]any{"name": "app", "image": "nginx:1.27-alpine"}}}}}}
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
		data, err := harness.RunCommandOutput("kubectl", "-n", testNamespace, "exec", pod, "-c", "alloy", "--", "sh", "-c", "if [ -f /tmp/sdlc.json ]; then cat /tmp/sdlc.json; fi")
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
				resource := rl.Resource().Attributes().AsRaw()
				resourceID, _ := resource["grafana.sdlc.provider.resource.id"].(string)
				resourceType, _ := resource["grafana.sdlc.provider.resource.type"].(string)
				changeType, _ := resource["grafana.sdlc.provider.resource.change.type"].(string)
				namespace, _, _ := strings.Cut(resourceID, "/")
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					records := rl.ScopeLogs().At(j).LogRecords()
					for k := 0; k < records.Len(); k++ {
						r := records.At(k)
						if r.EventName() != "grafana.sdlc.change" {
							return nil, fmt.Errorf("unexpected event name %q", r.EventName())
						}
						attrs := r.Attributes().AsRaw()
						attrs["grafana.sdlc.provider.type"] = resource["grafana.sdlc.provider.type"]
						body, _ := attrs["grafana.sdlc.change.diff"].(map[string]any)
						events = append(events, rolloutEvent{name: changeType, namespace: namespace, resourceType: resourceType, resourceID: resourceID, pod: pod, timestamp: uint64(r.Timestamp()), body: body, attrs: attrs})
					}
				}
			}
		}
	}
	return events, nil
}

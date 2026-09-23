package otelcolreceiverkubernetesrollouts

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	testNamespace = "test-kubernetes-rollouts"
	alloyRelease  = "alloy-test-kubernetes-rollouts"
	rolloutPrefix = "grafana.sdlc.k8s.deployment.rollout."
	deployPrefix  = "grafana.sdlc.k8s.deployment."
)

type rolloutEvent struct {
	name       string
	generation int64
	resource   pcommon.Map
	attributes pcommon.Map
}

func TestKubernetesRollouts(t *testing.T) {
	namespace := deps.NewNamespace(deps.NamespaceOptions{
		Name:   testNamespace,
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  namespace.Name(),
		Release:    alloyRelease,
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{Dependencies: []harness.Dependency{namespace, alloy}})

	t.Run("single clustered watcher", func(t *testing.T) {
		applyDeployment(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clustered
spec:
  replicas: 1
  progressDeadlineSeconds: 60
  selector:
    matchLabels:
      app: clustered
  template:
    metadata:
      labels:
        app: clustered
    spec:
      containers:
        - name: app
          image: unavailable.invalid/app:clustered
          imagePullPolicy: Never
`)

		waitForPhases(t, "clustered", "started")
		require.Never(t, func() bool {
			eventsByPod, err := readDeploymentEventsByPod("clustered")
			if err != nil {
				return true
			}
			return countPhase(eventsByPod, "started") != 1
		}, 5*time.Second, 250*time.Millisecond, "expected exactly one clustered Alloy pod to emit started")

		eventsByPod, err := readDeploymentEventsByPod("clustered")
		require.NoError(t, err)
		require.Len(t, eventsByPod, 3)
		require.Equal(t, 1, countPhase(eventsByPod, "started"))
		require.Len(t, podsWithPhase(eventsByPod, "started"), 1)
	})

	t.Run("deployment inventory", func(t *testing.T) {
		applyDeployment(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inventory
  labels:
    app.kubernetes.io/name: inventory
    team: platform
spec:
  replicas: 1
  progressDeadlineSeconds: 60
  selector:
    matchLabels:
      app: inventory
  template:
    metadata:
      labels:
        app: inventory
    spec:
      containers:
        - name: app
          image: unavailable.invalid/app:inventory
          imagePullPolicy: Never
`)

		events := waitForPhases(t, "inventory", "observed")
		observed := requireEvent(t, events, "observed")
		requireMapString(t, observed.resource, "k8s.cluster.uid", "kind-integration-test-uid")
		requireMapString(t, observed.resource, "k8s.namespace.name", testNamespace)
		requireMapString(t, observed.resource, "k8s.deployment.name", "inventory")
		requireNestedMapString(t, observed.attributes, "grafana.sdlc.k8s.deployment.labels", "team", "platform")
		requireContainerImageReference(t, observed.attributes, "app", "unavailable.invalid/app:inventory")

		require.NoError(t, harness.RunCommand(
			"kubectl", "--namespace", testNamespace, "delete", "deployment/inventory", "--wait=true",
		))
		events = waitForPhases(t, "inventory", "deleted")
		requireEvent(t, events, "deleted")
	})

	t.Run("successful rollout", func(t *testing.T) {
		applyDeployment(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: successful
spec:
  replicas: 1
  minReadySeconds: 5
  progressDeadlineSeconds: 30
  selector:
    matchLabels:
      app: successful
  template:
    metadata:
      labels:
        app: successful
    spec:
      containers:
        - name: app
          image: prom-gen:latest
          imagePullPolicy: Never
`)

		events := waitForPhases(t, "successful", "started", "image_resolved", "succeeded")
		requirePhaseOrder(t, events, "started", "image_resolved", "succeeded")

		resolved := requireEvent(t, events, "image_resolved")
		requireMapString(t, resolved.resource, "k8s.cluster.uid", "kind-integration-test-uid")
		requireMapString(t, resolved.resource, "k8s.cluster.name", "kind-integration-test")
		requireMapString(t, resolved.resource, "k8s.namespace.name", testNamespace)
		requireMapString(t, resolved.resource, "k8s.deployment.name", "successful")
		requireMapString(t, resolved.attributes, "grafana.sdlc.container.image.reference", "prom-gen:latest")
		requireMapString(t, resolved.attributes, "container.image.name", "prom-gen")
		requireMapStringContains(t, resolved.attributes, "container.image.id", "sha256:")
		requireMapSliceStringPrefix(t, resolved.attributes, "container.image.repo_digests", "prom-gen@sha256:")
	})

	t.Run("stalled rollout", func(t *testing.T) {
		applyDeployment(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stalled
spec:
  replicas: 1
  progressDeadlineSeconds: 5
  selector:
    matchLabels:
      app: stalled
  template:
    metadata:
      labels:
        app: stalled
    spec:
      containers:
        - name: app
          image: unavailable.invalid/app:stalled
          imagePullPolicy: Never
`)

		events := waitForPhases(t, "stalled", "started", "stalled")
		requirePhaseOrder(t, events, "started", "stalled")
	})

	t.Run("superseded rollout", func(t *testing.T) {
		applyDeployment(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: superseded
spec:
  replicas: 1
  progressDeadlineSeconds: 60
  selector:
    matchLabels:
      app: superseded
  template:
    metadata:
      labels:
        app: superseded
    spec:
      containers:
        - name: app
          image: unavailable.invalid/app:first
          imagePullPolicy: Never
`)

		firstEvents := waitForPhases(t, "superseded", "started")
		firstStarted := requireEvent(t, firstEvents, "started")

		require.NoError(t, harness.RunCommand(
			"kubectl", "--namespace", testNamespace, "set", "image",
			"deployment/superseded", "app=unavailable.invalid/app:second",
		))

		events := waitForSupersededSequence(t, "superseded", firstStarted.generation)
		requirePhaseOrder(t, events, "started", "superseded", "started")

		superseded := requireEventWithGeneration(t, events, "superseded", firstStarted.generation)
		requireContainerImageReference(t, superseded.attributes, "app", "unavailable.invalid/app:first")

		secondStarted := requireEventAfterGeneration(t, events, "started", firstStarted.generation)
		requireContainerImageReference(t, secondStarted.attributes, "app", "unavailable.invalid/app:second")
	})
}

func applyDeployment(t *testing.T, manifest string) {
	t.Helper()
	require.NoError(t, harness.RunCommandStdin(
		manifest,
		"kubectl", "apply", "--namespace", testNamespace, "--filename", "-",
	))
}

func waitForPhases(t *testing.T, deployment string, phases ...string) []rolloutEvent {
	t.Helper()
	var events []rolloutEvent
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var err error
		events, err = readDeploymentEvents(deployment)
		if !assert.NoError(c, err) {
			return
		}
		for _, phase := range phases {
			assert.True(c, containsPhase(events, phase), "missing %q event; got %v", phase, eventNames(events))
		}
	}, 2*time.Minute, time.Second)
	return events
}

func waitForSupersededSequence(t *testing.T, deployment string, firstGeneration int64) []rolloutEvent {
	t.Helper()
	var events []rolloutEvent
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var err error
		events, err = readDeploymentEvents(deployment)
		if !assert.NoError(c, err) {
			return
		}
		assert.True(c, containsPhaseGeneration(events, "superseded", firstGeneration),
			"missing superseded event for generation %d; got %v", firstGeneration, eventNames(events))
		assert.True(c, containsPhaseAfterGeneration(events, "started", firstGeneration),
			"missing started event after generation %d; got %v", firstGeneration, eventNames(events))
	}, 2*time.Minute, time.Second)
	return events
}

func readDeploymentEvents(deployment string) ([]rolloutEvent, error) {
	eventsByPod, err := readDeploymentEventsByPod(deployment)
	if err != nil {
		return nil, err
	}

	var events []rolloutEvent
	for _, podEvents := range eventsByPod {
		events = append(events, podEvents...)
	}
	return events, nil
}

func readDeploymentEventsByPod(deployment string) (map[string][]rolloutEvent, error) {
	podOutput, err := harness.RunCommandOutput(
		"kubectl", "--namespace", testNamespace, "get", "pods",
		"--selector", "app.kubernetes.io/instance="+alloyRelease,
		"--output", `jsonpath={range .items[*]}{.metadata.name}{"\n"}{end}`,
	)
	if err != nil {
		return nil, err
	}

	eventsByPod := make(map[string][]rolloutEvent)
	for _, pod := range strings.Fields(podOutput) {
		contents, readErr := harness.RunCommandOutput(
			"kubectl", "--namespace", testNamespace, "exec", "pod/"+pod,
			"--container", "alloy", "--", "sh", "-c",
			"if [ -f /tmp/kubernetes-rollouts.json ]; then cat /tmp/kubernetes-rollouts.json; fi",
		)
		if readErr != nil {
			return nil, fmt.Errorf("read rollout events from Alloy pod %s: %w", pod, readErr)
		}
		podEvents, decodeErr := decodeDeploymentEvents(contents, deployment)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode rollout events from Alloy pod %s: %w", pod, decodeErr)
		}
		eventsByPod[pod] = podEvents
	}
	return eventsByPod, nil
}

func decodeDeploymentEvents(contents, deployment string) ([]rolloutEvent, error) {
	var events []rolloutEvent
	for lineNumber, line := range strings.Split(strings.TrimSpace(contents), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		logs, err := (&plog.JSONUnmarshaler{}).UnmarshalLogs([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("decode OTLP JSON line %d: %w", lineNumber+1, err)
		}
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			resourceLogs := logs.ResourceLogs().At(i)
			name, ok := resourceLogs.Resource().Attributes().Get("k8s.deployment.name")
			if !ok || name.Str() != deployment {
				continue
			}
			for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
				records := resourceLogs.ScopeLogs().At(j).LogRecords()
				for k := 0; k < records.Len(); k++ {
					record := records.At(k)
					generation, _ := record.Attributes().Get("grafana.sdlc.deployment.generation")
					name := strings.TrimPrefix(record.EventName(), deployPrefix)
					if strings.HasPrefix(record.EventName(), rolloutPrefix) {
						name = strings.TrimPrefix(record.EventName(), rolloutPrefix)
					}
					events = append(events, rolloutEvent{
						name:       name,
						generation: generation.Int(),
						resource:   resourceLogs.Resource().Attributes(),
						attributes: record.Attributes(),
					})
				}
			}
		}
	}
	return events, nil
}

func countPhase(eventsByPod map[string][]rolloutEvent, phase string) int {
	count := 0
	for _, events := range eventsByPod {
		for _, event := range events {
			if event.name == phase {
				count++
			}
		}
	}
	return count
}

func podsWithPhase(eventsByPod map[string][]rolloutEvent, phase string) []string {
	var pods []string
	for pod, events := range eventsByPod {
		if containsPhase(events, phase) {
			pods = append(pods, pod)
		}
	}
	return pods
}

func containsPhase(events []rolloutEvent, phase string) bool {
	for _, event := range events {
		if event.name == phase {
			return true
		}
	}
	return false
}

func containsPhaseGeneration(events []rolloutEvent, phase string, generation int64) bool {
	for _, event := range events {
		if event.name == phase && event.generation == generation {
			return true
		}
	}
	return false
}

func containsPhaseAfterGeneration(events []rolloutEvent, phase string, generation int64) bool {
	for _, event := range events {
		if event.name == phase && event.generation > generation {
			return true
		}
	}
	return false
}

func requirePhaseOrder(t *testing.T, events []rolloutEvent, phases ...string) {
	t.Helper()
	next := 0
	for _, event := range events {
		if next < len(phases) && event.name == phases[next] {
			next++
		}
	}
	require.Equal(t, len(phases), next, "phases %v did not appear in order; got %v", phases, eventNames(events))
}

func requireEvent(t *testing.T, events []rolloutEvent, phase string) rolloutEvent {
	t.Helper()
	for _, event := range events {
		if event.name == phase {
			return event
		}
	}
	require.FailNow(t, "event not found", "phase %q; got %v", phase, eventNames(events))
	return rolloutEvent{}
}

func requireEventWithGeneration(t *testing.T, events []rolloutEvent, phase string, generation int64) rolloutEvent {
	t.Helper()
	for _, event := range events {
		if event.name == phase && event.generation == generation {
			return event
		}
	}
	require.FailNow(t, "event not found", "phase %q generation %d; got %v", phase, generation, eventNames(events))
	return rolloutEvent{}
}

func requireEventAfterGeneration(t *testing.T, events []rolloutEvent, phase string, generation int64) rolloutEvent {
	t.Helper()
	for _, event := range events {
		if event.name == phase && event.generation > generation {
			return event
		}
	}
	require.FailNow(t, "event not found", "phase %q after generation %d; got %v", phase, generation, eventNames(events))
	return rolloutEvent{}
}

func eventNames(events []rolloutEvent) []string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, fmt.Sprintf("%s(generation=%d)", event.name, event.generation))
	}
	return names
}

func requireMapString(t *testing.T, attributes pcommon.Map, key, expected string) {
	t.Helper()
	value, ok := attributes.Get(key)
	require.True(t, ok, "missing attribute %q", key)
	require.Equal(t, expected, value.Str())
}

func requireMapStringContains(t *testing.T, attributes pcommon.Map, key, substring string) {
	t.Helper()
	value, ok := attributes.Get(key)
	require.True(t, ok, "missing attribute %q", key)
	require.Contains(t, value.Str(), substring)
}

func requireMapSliceStringPrefix(t *testing.T, attributes pcommon.Map, key, prefix string) {
	t.Helper()
	value, ok := attributes.Get(key)
	require.True(t, ok, "missing attribute %q", key)
	require.Positive(t, value.Slice().Len(), "attribute %q is empty", key)
	require.True(t, strings.HasPrefix(value.Slice().At(0).Str(), prefix),
		"%q does not start with %q", value.Slice().At(0).Str(), prefix)
}

func requireNestedMapString(t *testing.T, attributes pcommon.Map, key, nestedKey, expected string) {
	t.Helper()
	value, ok := attributes.Get(key)
	require.True(t, ok, "missing attribute %q", key)
	requireMapString(t, value.Map(), nestedKey, expected)
}

func requireContainerImageReference(t *testing.T, attributes pcommon.Map, name, reference string) {
	t.Helper()
	value, ok := attributes.Get("grafana.sdlc.deployment.containers")
	require.True(t, ok, "missing deployment containers")
	for i := 0; i < value.Slice().Len(); i++ {
		container := value.Slice().At(i).Map()
		containerName, hasName := container.Get("name")
		imageReference, hasReference := container.Get("image.reference")
		if hasName && hasReference && containerName.Str() == name && imageReference.Str() == reference {
			return
		}
	}
	require.FailNow(t, "container image not found", "name %q reference %q", name, reference)
}

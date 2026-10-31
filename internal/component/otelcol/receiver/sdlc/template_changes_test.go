package sdlc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestTemplateChangesValuesAndRedaction(t *testing.T) {
	d := deploymentFixture()
	old := d.Spec.Template.DeepCopy()
	old.Spec.Containers[0].Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")}
	next := old.DeepCopy()
	next.Spec.Containers[0].Image = "nginx:1.28"
	next.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("500m")
	next.Spec.Containers[0].Resources.Limits = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")}
	next.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "TOKEN", Value: "secret-env"}}
	next.Spec.Containers[0].Args = []string{"secret-arg"}
	next.Annotations = map[string]string{"token": "secret-annotation"}
	next.Spec.Volumes = []corev1.Volume{{Name: "private", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "secret-volume"}}}}
	changes := templateChanges(*old, *next)
	byField := map[string]fieldChange{}
	for _, change := range changes {
		byField[strings.TrimPrefix(change.Path, "containers.app.")] = change
	}
	require.Equal(t, "nginx:1.27", *byField["image"].Before)
	require.Equal(t, "nginx:1.28", *byField["image"].After)
	require.Equal(t, "250m", *byField["resources.requests.cpu"].Before)
	require.Equal(t, "500m", *byField["resources.requests.cpu"].After)
	require.Nil(t, byField["resources.limits.memory"].Before)
	require.Equal(t, "512Mi", *byField["resources.limits.memory"].After)
	require.Equal(t, "added", byField["resources.limits.memory"].Operation)
	// Environment variables report which one changed, never its value.
	require.Equal(t, "added", byField["env.TOKEN"].Operation)
	require.Nil(t, byField["env.TOKEN"].After)
	for _, field := range []string{"args", "metadata.annotations", "spec.volumes"} {
		require.Contains(t, byField, field)
		require.Nil(t, byField[field].Before)
		require.Nil(t, byField[field].After)
	}
	encoded, err := json.Marshal(changes)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret-")
	// Removing a resource preserves its previous value and absence of a new value.
	reverse := templateChanges(*next, *old)
	for _, change := range reverse {
		if change.Path == "containers.app.resources.limits.memory" {
			require.Equal(t, "removed", change.Operation)
			require.Equal(t, "512Mi", *change.Before)
			require.Nil(t, change.After)
		}
	}
	// Canonically equivalent quantities don't appear as changes.
	equivalent := old.DeepCopy()
	equivalent.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("0.25")
	changes = templateChanges(*old, *equivalent)
	require.Empty(t, changes)
}

func TestTemplateChangesContainers(t *testing.T) {
	d := deploymentFixture()
	old := d.Spec.Template.DeepCopy()
	old.Spec.InitContainers = []corev1.Container{{Name: "setup", Image: "setup:v1"}}
	next := old.DeepCopy()
	next.Spec.InitContainers[0].Image = "setup:v2"
	next.Spec.Containers = []corev1.Container{{Name: "replacement", Image: "app:v1"}}
	changes := templateChanges(*old, *next)
	require.Contains(t, changes, fieldChange{Path: "containers.app", Operation: "removed"})
	require.Contains(t, changes, fieldChange{Path: "containers.replacement", Operation: "added"})
	found := false
	for _, change := range changes {
		if strings.HasPrefix(change.Path, "initContainers.setup") {
			found = true
			require.Equal(t, "initContainers.setup.image", change.Path)
			require.Equal(t, "setup:v1", *change.Before)
			require.Equal(t, "setup:v2", *change.After)
		}
	}
	require.True(t, found)
	old.Spec.Containers = append(old.Spec.Containers, corev1.Container{Name: "sidecar", Image: "sidecar:v1"})
	next = old.DeepCopy()
	next.Spec.Containers[0], next.Spec.Containers[1] = next.Spec.Containers[1], next.Spec.Containers[0]
	changes = templateChanges(*old, *next)
	require.Equal(t, []fieldChange{{Path: "spec.containers.order", Operation: "modified"}}, changes)
}

func TestTemplateChangesAllowlistedValues(t *testing.T) {
	old := deploymentFixture().Spec.Template.DeepCopy()
	old.Labels = map[string]string{"app": "web", "tier": "front"}
	old.Spec.ServiceAccountName = "web"
	old.Spec.NodeSelector = map[string]string{"disk": "ssd"}
	old.Spec.Containers[0].Ports = []corev1.ContainerPort{{ContainerPort: 80}}
	old.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "KEEP", Value: "a"}, {Name: "CHANGE", Value: "old-secret"}, {Name: "DROP", Value: "x"}}
	next := old.DeepCopy()
	next.Labels = map[string]string{"app": "web", "version": "2"}
	next.Spec.ServiceAccountName = "web-v2"
	next.Spec.NodeSelector = nil
	next.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	next.Spec.Containers[0].Ports = []corev1.ContainerPort{{ContainerPort: 80}, {ContainerPort: 443, Protocol: corev1.ProtocolTCP}}
	next.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "KEEP", Value: "a"}, {Name: "CHANGE", Value: "new-secret"}, {Name: "ADD", Value: "y"}}
	byPath := map[string]fieldChange{}
	for _, change := range templateChanges(*old, *next) {
		byPath[change.Path] = change
	}
	str := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	for path, want := range map[string][3]string{
		"metadata.labels.tier":           {"removed", "front", "<nil>"},
		"metadata.labels.version":        {"added", "<nil>", "2"},
		"spec.serviceAccountName":        {"modified", "web", "web-v2"},
		"spec.nodeSelector.disk":         {"removed", "ssd", "<nil>"},
		"containers.app.imagePullPolicy": {"added", "<nil>", "Always"},
		"containers.app.ports":           {"modified", "80/TCP", "443/TCP,80/TCP"},
		"containers.app.env.CHANGE":      {"modified", "<nil>", "<nil>"},
		"containers.app.env.ADD":         {"added", "<nil>", "<nil>"},
		"containers.app.env.DROP":        {"removed", "<nil>", "<nil>"},
	} {
		got, ok := byPath[path]
		require.True(t, ok, path)
		require.Equal(t, want, [3]string{got.Operation, str(got.Before), str(got.After)}, path)
	}
	require.NotContains(t, byPath, "containers.app.env.KEEP")
	encoded, err := json.Marshal(byPath)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
}

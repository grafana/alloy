package sdlc

import (
	"encoding/json"
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
	byField := map[string]templateChange{}
	for _, change := range changes {
		byField[change.Field] = change
	}
	require.Equal(t, "nginx:1.27", *byField["image"].Before)
	require.Equal(t, "nginx:1.28", *byField["image"].After)
	require.Equal(t, "250m", *byField["resources.requests.cpu"].Before)
	require.Equal(t, "500m", *byField["resources.requests.cpu"].After)
	require.Nil(t, byField["resources.limits.memory"].Before)
	require.Equal(t, "512Mi", *byField["resources.limits.memory"].After)
	require.Equal(t, "added", byField["resources.limits.memory"].Operation)
	for _, field := range []string{"env", "args", "metadata.annotations", "spec.volumes"} {
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
		if change.Field == "resources.limits.memory" {
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
	require.Contains(t, changes, templateChange{Container: "app", Field: "container", Operation: "removed"})
	require.Contains(t, changes, templateChange{Container: "replacement", Field: "container", Operation: "added"})
	found := false
	for _, change := range changes {
		if change.Container == "setup" {
			found = true
			require.True(t, change.Init)
			require.Equal(t, "image", change.Field)
			require.Equal(t, "setup:v1", *change.Before)
			require.Equal(t, "setup:v2", *change.After)
		}
	}
	require.True(t, found)
	old.Spec.Containers = append(old.Spec.Containers, corev1.Container{Name: "sidecar", Image: "sidecar:v1"})
	next = old.DeepCopy()
	next.Spec.Containers[0], next.Spec.Containers[1] = next.Spec.Containers[1], next.Spec.Containers[0]
	changes = templateChanges(*old, *next)
	require.Equal(t, []templateChange{{Field: "spec.containers.order", Operation: "modified"}}, changes)
}

package sdlc

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestPodTemplateSnapshotKeepsValues(t *testing.T) {
	template := deploymentFixture().Spec.Template.DeepCopy()
	app := &template.Spec.Containers[0]
	app.Env = []corev1.EnvVar{{Name: "MODE", Value: "fast"}, {Name: "FROM", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "creds"}, Key: "token"}}}}
	app.Command = []string{"app", "--verbose"}
	app.Args = []string{"serve"}
	app.ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/ready?full=1", Port: intstr.FromInt32(8080)}}}
	app.Ports = []corev1.ContainerPort{{ContainerPort: 80, Protocol: corev1.ProtocolTCP}}
	app.Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")}
	template.Labels = map[string]string{"app": "web"}
	template.Annotations = map[string]string{"owner": "team-a", lastAppliedAnnotation: `{"kind":"Deployment"}`}
	template.Spec.SecurityContext = &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000))}
	template.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: "Memory"}}}}

	got := snapshot(podTemplateRule, template)
	spec := got["spec"].(map[string]any)
	container := spec["containers"].([]any)[0].(map[string]any)
	require.Equal(t, "nginx:1.27", container["image"])
	require.Equal(t, []any{"app", "--verbose"}, container["command"])
	require.Equal(t, []any{"serve"}, container["args"])
	require.Equal(t, []any{
		map[string]any{"name": "MODE", "value": "fast"},
		map[string]any{"name": "FROM", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "creds", "key": "token"}}},
	}, container["env"])
	require.Equal(t, map[string]any{"httpGet": map[string]any{"path": "/ready?full=1", "port": int64(8080)}}, container["readinessProbe"])
	require.Equal(t, []any{map[string]any{"containerPort": int64(80), "protocol": "TCP"}}, container["ports"])
	require.Equal(t, map[string]any{"requests": map[string]any{"cpu": "250m"}}, container["resources"])
	require.Equal(t, map[string]any{"runAsUser": int64(1000)}, spec["securityContext"])
	require.Equal(t, []any{map[string]any{"name": "data", "emptyDir": map[string]any{"medium": "Memory"}}}, spec["volumes"])
	require.Equal(t, map[string]any{
		"labels":      map[string]any{"app": "web"},
		"annotations": map[string]any{"owner": "team-a"},
	}, got["metadata"], "last-applied-configuration is left out")
	require.Nil(t, snapshot[corev1.PodTemplateSpec](podTemplateRule, nil))
}

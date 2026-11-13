package sdlc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestPodTemplateSnapshotRedactsCredentials(t *testing.T) {
	template := deploymentFixture().Spec.Template.DeepCopy()
	app := &template.Spec.Containers[0]
	app.Env = []corev1.EnvVar{{Name: "TOKEN", Value: "secret-env"}, {Name: "FROM", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "creds"}, Key: "token"}}}}
	app.Command = []string{"app", "--token=secret-command"}
	app.Args = []string{"secret-arg"}
	headers := []corev1.HTTPHeader{{Name: "Authorization", Value: "Bearer secret-header"}}
	app.ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/ready?key=secret-query", Port: intstr.FromInt32(8080), HTTPHeaders: headers}}}
	app.LivenessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"check", "secret-probe"}}}, PeriodSeconds: 5}
	app.Lifecycle = &corev1.Lifecycle{PreStop: &corev1.LifecycleHandler{Exec: &corev1.ExecAction{Command: []string{"stop", "secret-hook"}}}}
	app.Ports = []corev1.ContainerPort{{ContainerPort: 80, Protocol: corev1.ProtocolTCP}}
	app.Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")}
	template.Labels = map[string]string{"app": "web"}
	template.Annotations = map[string]string{"owner": "team-a", lastAppliedAnnotation: `{"secret-manifest":1}`}
	template.Spec.SecurityContext = &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000))}
	template.Spec.NodeSelector = map[string]string{"pool": "general"}
	template.Spec.Volumes = []corev1.Volume{
		{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: "Memory"}}},
		{Name: "vault", VolumeSource: corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: "secrets.example.com", VolumeAttributes: map[string]string{"token": "secret-csi"}}}},
		{Name: "flex", VolumeSource: corev1.VolumeSource{FlexVolume: &corev1.FlexVolumeSource{Driver: "example/flex", Options: map[string]string{"password": "secret-flex"}}}},
	}

	got := snapshot(podTemplateRule, template)
	spec := got["spec"].(map[string]any)
	container := spec["containers"].([]any)[0].(map[string]any)
	require.Equal(t, []any{redacted("app"), redacted("--token=secret-command")}, container["command"])
	require.Equal(t, []any{redacted("secret-arg")}, container["args"])
	require.Equal(t, []any{
		map[string]any{"name": "TOKEN", "value": redacted("secret-env")},
		// References keep their values.
		map[string]any{"name": "FROM", "valueFrom": map[string]any{"secretKeyRef": map[string]any{"name": "creds", "key": "token"}}},
	}, container["env"])
	require.Equal(t, map[string]any{"httpGet": map[string]any{"path": "/ready?" + redacted("key=secret-query"), "port": int64(8080), "httpHeaders": []any{map[string]any{"name": "Authorization", "value": redacted("Bearer secret-header")}}}}, container["readinessProbe"])
	require.Equal(t, map[string]any{"exec": map[string]any{"command": []any{redacted("check"), redacted("secret-probe")}}, "periodSeconds": int64(5)}, container["livenessProbe"])
	require.Equal(t, map[string]any{"preStop": map[string]any{"exec": map[string]any{"command": []any{redacted("stop"), redacted("secret-hook")}}}}, container["lifecycle"])
	require.Equal(t, []any{
		map[string]any{"name": "data", "emptyDir": map[string]any{"medium": "Memory"}},
		map[string]any{"name": "vault", "csi": map[string]any{"driver": "secrets.example.com", "volumeAttributes": map[string]any{"token": redacted("secret-csi")}}},
		map[string]any{"name": "flex", "flexVolume": map[string]any{"driver": "example/flex", "options": map[string]any{"password": redacted("secret-flex")}}},
	}, spec["volumes"])
	// Other fields keep their values.
	require.Equal(t, "nginx:1.27", container["image"])
	require.Equal(t, []any{map[string]any{"containerPort": int64(80), "protocol": "TCP"}}, container["ports"])
	require.Equal(t, map[string]any{"requests": map[string]any{"cpu": "250m"}}, container["resources"])
	require.Equal(t, map[string]any{"runAsUser": int64(1000)}, spec["securityContext"])
	require.Equal(t, map[string]any{"pool": "general"}, spec["nodeSelector"])
	require.Equal(t, map[string]any{
		"labels":      map[string]any{"app": "web"},
		"annotations": map[string]any{"owner": "team-a"},
	}, got["metadata"], "last-applied-configuration is left out")
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret-")
	require.Nil(t, snapshot[corev1.PodTemplateSpec](podTemplateRule, nil))
}

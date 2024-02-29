package operator

import (
	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

func generateLogsDaemonSet(
	cfg *Config,
	name string,
	d gragent.Deployment,
) (*apps_v1.DaemonSet, error) {

	d = *(&d).DeepCopy()

	opts := logsPodTemplateOptions()
	tmpl, selector, err := generatePodTemplate(cfg, name, d, opts)
	if err != nil {
		return nil, err
	}

	spec := apps_v1.DaemonSetSpec{
		UpdateStrategy: apps_v1.DaemonSetUpdateStrategy{
			Type: apps_v1.RollingUpdateDaemonSetStrategyType,
		},
		Selector: selector,
		Template: tmpl,
	}

	return &apps_v1.DaemonSet{
		ObjectMeta: metadataFromPodTemplate(name, d, tmpl),
		Spec:       spec,
	}, nil
}

func logsPodTemplateOptions() podTemplateOptions {
	return podTemplateOptions{
		Privileged: true,
		ExtraSelectorLabels: map[string]string{
			agentTypeLabel: "logs",
		},
		ExtraVolumes: []v1.Volume{
			{
				Name: "varlog",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{Path: "/var/log"},
				},
			},
			{
				// Needed for docker. Kubernetes will symlink to this directory. For CRI
				// platforms, this doesn't change anything.
				Name: "dockerlogs",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{Path: "/var/lib/docker/containers"},
				},
			},
			{
				// Needed for storing positions for recovery.
				Name: "data",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{Path: "/var/lib/grafana-agent/data"},
				},
			},
		},
		ExtraVolumeMounts: []v1.VolumeMount{
			{
				Name:      "varlog",
				ReadOnly:  true,
				MountPath: "/var/log",
			}, {
				Name:      "dockerlogs",
				ReadOnly:  true,
				MountPath: "/var/lib/docker/containers",
			}, {
				Name:      "data",
				MountPath: "/var/lib/grafana-agent/data",
			},
		},
		ExtraEnvVars: []v1.EnvVar{
			{
				Name: "HOSTNAME",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
				},
			},
			{
				// Not used anywhere for logs but passed to the config-reloader since it
				// expects everything is coming from a StatefulSet.
				Name:  "SHARD",
				Value: "0",
			},
		},
	}
}

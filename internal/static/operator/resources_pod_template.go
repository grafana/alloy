package operator

import (
	"fmt"
	"path"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/grafana/agent/internal/build"
	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/clientutil"
)

type podTemplateOptions struct {
	ExtraSelectorLabels map[string]string
	ExtraVolumes        []core_v1.Volume
	ExtraVolumeMounts   []core_v1.VolumeMount
	ExtraEnvVars        []core_v1.EnvVar
	Privileged          bool
}

func generatePodTemplate(
	cfg *Config,
	name string,
	d gragent.Deployment,
	opts podTemplateOptions,
) (core_v1.PodTemplateSpec, *meta_v1.LabelSelector, error) {

	// generatePodTemplate assumes that the deployment has default values applied
	// to it.
	applyDeploymentDefaults(&d)

	useVersion := d.Agent.Spec.Version
	if useVersion == "" {
		useVersion = DefaultAgentVersion
	}
	imagePath := fmt.Sprintf("%s:%s", DefaultAgentBaseImage, useVersion)
	if d.Agent.Spec.Image != nil && *d.Agent.Spec.Image != "" {
		imagePath = *d.Agent.Spec.Image
	}

	agentArgs := []string{
		"-config.file=/var/lib/grafana-agent/config/agent.yml",
		"-config.expand-env=true",
		"-server.http.address=0.0.0.0:8080",
		"-enable-features=integrations-next",
	}

	enableConfigReadAPI := d.Agent.Spec.EnableConfigReadAPI
	if enableConfigReadAPI {
		agentArgs = append(agentArgs, "-config.enable-read-api")
	}

	disableReporting := d.Agent.Spec.DisableReporting
	if disableReporting {
		agentArgs = append(agentArgs, "-disable-reporting")
	}

	if d.Agent.Spec.DisableSupportBundle {
		agentArgs = append(agentArgs, "-disable-support-bundle")
	}

	// NOTE(rfratto): the Prometheus Operator supports a ListenLocal to prevent a
	// service from being created. Given the intent is that Agents can connect to
	// each other, ListenLocal isn't currently supported and we always create a
	// port.
	ports := []core_v1.ContainerPort{{
		Name:          d.Agent.Spec.PortName,
		ContainerPort: 8080,
		Protocol:      core_v1.ProtocolTCP,
	}}

	volumes := []core_v1.Volume{
		{
			Name: "config",
			VolumeSource: core_v1.VolumeSource{
				Secret: &core_v1.SecretVolumeSource{
					SecretName: fmt.Sprintf("%s-config", name),
				},
			},
		},
		{
			// We need a separate volume for storing the rendered config with
			// environment variables replaced. While the Agent supports environment
			// variable substitution, the value for __replica__ can only be
			// determined at runtime. We use a dedicated container for both config
			// reloading and rendering.
			Name: "config-out",
			VolumeSource: core_v1.VolumeSource{
				EmptyDir: &core_v1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "secrets",
			VolumeSource: core_v1.VolumeSource{
				Secret: &core_v1.SecretVolumeSource{
					SecretName: fmt.Sprintf("%s-secrets", d.Agent.Name),
				},
			},
		},
	}
	volumes = append(volumes, opts.ExtraVolumes...)
	volumes = append(volumes, d.Agent.Spec.Volumes...)

	volumeMounts := []core_v1.VolumeMount{
		{
			Name:      "config",
			ReadOnly:  true,
			MountPath: "/var/lib/grafana-agent/config-in",
		},
		{
			Name:      "config-out",
			MountPath: "/var/lib/grafana-agent/config",
		},
		{
			Name:      "secrets",
			ReadOnly:  true,
			MountPath: "/var/lib/grafana-agent/secrets",
		},
	}
	volumeMounts = append(volumeMounts, opts.ExtraVolumeMounts...)
	volumeMounts = append(volumeMounts, d.Agent.Spec.VolumeMounts...)

	for _, s := range d.Agent.Spec.Secrets {
		volumes = append(volumes, core_v1.Volume{
			Name: clientutil.SanitizeVolumeName("secret-" + s),
			VolumeSource: core_v1.VolumeSource{
				Secret: &core_v1.SecretVolumeSource{SecretName: s},
			},
		})
		volumeMounts = append(volumeMounts, core_v1.VolumeMount{
			Name:      clientutil.SanitizeVolumeName("secret-" + s),
			ReadOnly:  true,
			MountPath: path.Join("/var/lib/grafana-agent/extra-secrets", s),
		})
	}

	for _, c := range d.Agent.Spec.ConfigMaps {
		volumes = append(volumes, core_v1.Volume{
			Name: clientutil.SanitizeVolumeName("configmap-" + c),
			VolumeSource: core_v1.VolumeSource{
				ConfigMap: &core_v1.ConfigMapVolumeSource{
					LocalObjectReference: core_v1.LocalObjectReference{Name: c},
				},
			},
		})
		volumeMounts = append(volumeMounts, core_v1.VolumeMount{
			Name:      clientutil.SanitizeVolumeName("configmap-" + c),
			ReadOnly:  true,
			MountPath: path.Join("/var/lib/grafana-agent/extra-configmaps", c),
		})
	}

	var (
		podAnnotations = map[string]string{}
		podLabels      = map[string]string{
			// version can be a pod label, but should not go in selectors
			versionLabelName: clientutil.SanitizeVolumeName(build.Version),
		}
		podSelectorLabels = map[string]string{
			"app.kubernetes.io/name":     "grafana-agent",
			"app.kubernetes.io/instance": d.Agent.Name,
			"grafana-agent":              d.Agent.Name,
			managedByOperatorLabel:       managedByOperatorLabelValue,
			agentNameLabelName:           d.Agent.Name,
		}
	)
	for k, v := range opts.ExtraSelectorLabels {
		podSelectorLabels[k] = v
	}

	if d.Agent.Spec.PodMetadata != nil {
		for k, v := range d.Agent.Spec.PodMetadata.Labels {
			podLabels[k] = v
		}
		for k, v := range d.Agent.Spec.PodMetadata.Annotations {
			podAnnotations[k] = v
		}
	}
	for k, v := range podSelectorLabels {
		podLabels[k] = v
	}

	podAnnotations["kubectl.kubernetes.io/default-container"] = "grafana-agent"

	var (
		finalSelectorLabels = cfg.Labels.Merge(podSelectorLabels)
		finalLabels         = cfg.Labels.Merge(podLabels)
	)

	envVars := []core_v1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &core_v1.EnvVarSource{
				FieldRef: &core_v1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
		// Allows the agent to identify this is an operator-created pod.
		{
			Name:  "AGENT_DEPLOY_MODE",
			Value: "operator",
		},
	}
	envVars = append(envVars, opts.ExtraEnvVars...)

	useConfigReloaderVersion := d.Agent.Spec.ConfigReloaderVersion
	if useConfigReloaderVersion == "" {
		useConfigReloaderVersion = DefaultConfigReloaderVersion
	}
	imagePathConfigReloader := fmt.Sprintf("%s:%s", DefaultConfigReloaderBaseImage, useConfigReloaderVersion)
	if d.Agent.Spec.ConfigReloaderImage != nil && *d.Agent.Spec.ConfigReloaderImage != "" {
		imagePathConfigReloader = *d.Agent.Spec.ConfigReloaderImage
	}

	boolFalse := false
	boolTrue := true
	operatorContainers := []core_v1.Container{
		{
			Name:         "config-reloader",
			Image:        imagePathConfigReloader,
			VolumeMounts: volumeMounts,
			Env:          envVars,
			SecurityContext: &core_v1.SecurityContext{
				AllowPrivilegeEscalation: &boolFalse,
				ReadOnlyRootFilesystem:   &boolTrue,
				Capabilities: &core_v1.Capabilities{
					Drop: []core_v1.Capability{"ALL"},
				},
			},
			Args: []string{
				"--config-file=/var/lib/grafana-agent/config-in/agent.yml",
				"--config-envsubst-file=/var/lib/grafana-agent/config/agent.yml",

				"--watch-interval=1m",
				"--statefulset-ordinal-from-envvar=POD_NAME",
				"--reload-url=http://127.0.0.1:8080/-/reload",
			},
		},
		{
			Name:         "grafana-agent",
			Image:        imagePath,
			Ports:        ports,
			Args:         agentArgs,
			VolumeMounts: volumeMounts,
			Env:          envVars,
			ReadinessProbe: &core_v1.Probe{
				ProbeHandler: core_v1.ProbeHandler{
					HTTPGet: &core_v1.HTTPGetAction{
						Path: "/-/ready",
						Port: intstr.FromString(d.Agent.Spec.PortName),
					},
				},
				TimeoutSeconds:   probeTimeoutSeconds,
				PeriodSeconds:    5,
				FailureThreshold: 120, // Allow up to 10m on startup for data recovery
			},
			Resources: d.Agent.Spec.Resources,
			SecurityContext: &core_v1.SecurityContext{
				Privileged: ptr.To(opts.Privileged),
			},
			TerminationMessagePolicy: core_v1.TerminationMessageFallbackToLogsOnError,
		},
	}

	containers, err := clientutil.MergePatchContainers(operatorContainers, d.Agent.Spec.Containers)
	if err != nil {
		return core_v1.PodTemplateSpec{}, nil, fmt.Errorf("failed to merge containers spec: %w", err)
	}

	var pullSecrets []core_v1.LocalObjectReference
	if len(d.Agent.Spec.ImagePullSecrets) > 0 {
		pullSecrets = d.Agent.Spec.ImagePullSecrets
	}

	template := core_v1.PodTemplateSpec{
		ObjectMeta: meta_v1.ObjectMeta{
			Labels:      finalLabels,
			Annotations: podAnnotations,
		},
		Spec: core_v1.PodSpec{
			Containers:                    containers,
			ImagePullSecrets:              pullSecrets,
			InitContainers:                d.Agent.Spec.InitContainers,
			SecurityContext:               d.Agent.Spec.SecurityContext,
			ServiceAccountName:            d.Agent.Spec.ServiceAccountName,
			NodeSelector:                  d.Agent.Spec.NodeSelector,
			PriorityClassName:             d.Agent.Spec.PriorityClassName,
			RuntimeClassName:              d.Agent.Spec.RuntimeClassName,
			TerminationGracePeriodSeconds: ptr.To(int64(4800)),
			Volumes:                       volumes,
			Tolerations:                   d.Agent.Spec.Tolerations,
			Affinity:                      d.Agent.Spec.Affinity,
			TopologySpreadConstraints:     d.Agent.Spec.TopologySpreadConstraints,
		},
	}
	return template, &meta_v1.LabelSelector{MatchLabels: finalSelectorLabels}, nil
}

func applyDeploymentDefaults(d *gragent.Deployment) {
	if d.Agent.Spec.Metrics.Replicas != nil && *d.Agent.Spec.Metrics.Replicas < 0 {
		intZero := int32(0)
		d.Agent.Spec.Metrics.Replicas = &intZero
	}

	if d.Agent.Spec.Resources.Requests == nil {
		d.Agent.Spec.Resources.Requests = core_v1.ResourceList{}
	}

	if d.Agent.Spec.Metrics.Replicas == nil {
		d.Agent.Spec.Metrics.Replicas = &minReplicas
	}

	if d.Agent.Spec.PortName == "" {
		d.Agent.Spec.PortName = defaultPortName
	}
}

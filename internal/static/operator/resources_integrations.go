package operator

import (
	"fmt"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/assets"
	"github.com/grafana/agent/internal/static/operator/config"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
)

func newIntegrationsDaemonSet(cfg *Config, name string, d gragent.Deployment) (*apps_v1.DaemonSet, error) {
	opts := integrationsPodTemplateOptions(name, d, true)
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

func newIntegrationsDeployment(cfg *Config, name string, d gragent.Deployment) (*apps_v1.Deployment, error) {
	opts := integrationsPodTemplateOptions(name, d, false)
	tmpl, selector, err := generatePodTemplate(cfg, name, d, opts)
	if err != nil {
		return nil, err
	}

	spec := apps_v1.DeploymentSpec{
		Strategy: apps_v1.DeploymentStrategy{
			Type: apps_v1.RollingUpdateDeploymentStrategyType,
		},
		Selector: selector,
		Template: tmpl,
	}

	return &apps_v1.Deployment{
		ObjectMeta: metadataFromPodTemplate(name, d, tmpl),
		Spec:       spec,
	}, nil
}

func integrationsPodTemplateOptions(name string, d gragent.Deployment, daemonset bool) podTemplateOptions {
	// Integrations expect that the metrics and logs instances exist. This means
	// that we have to merge the podTemplateOptions used for metrics and logs
	// with the options used for integrations.

	// Since integrations may be running as a DaemonSet, it's not possible for us
	// to rely on a PVC template that metrics might be using. We'll force the WAL
	// to use an empty volume.
	d.Agent.Spec.Storage = nil

	integrationOpts := podTemplateOptions{
		ExtraSelectorLabels: map[string]string{
			agentTypeLabel: "integrations",
		},
		Privileged: daemonset,
	}

	// We need to iterate over all of our integrations to append extra Volumes,
	// VolumesMounts, and references to Secrets or ConfigMaps from the resource
	// hierarchy.
	var (
		secretsPaths []core_v1.KeyToPath
		mountedKeys  = map[assets.Key]struct{}{}
	)

	for _, i := range d.Integrations {
		inst := i.Instance
		volumePrefix := fmt.Sprintf("%s-%s-", inst.Namespace, inst.Name)

		for _, v := range inst.Spec.Volumes {
			// Prefix the key of the Integration CR so it doesn't potentially collide
			// with other loaded Integration CRs.
			v = *v.DeepCopy()
			v.Name = volumePrefix + v.Name

			integrationOpts.ExtraVolumes = append(integrationOpts.ExtraVolumes, v)
		}
		for _, vm := range inst.Spec.VolumeMounts {
			// Prefix the key of the Integration CR so it doesn't potentially collide
			// with other loaded Integration CRs.
			vm = *vm.DeepCopy()
			vm.Name = volumePrefix + vm.Name

			integrationOpts.ExtraVolumeMounts = append(integrationOpts.ExtraVolumeMounts, vm)
		}

		for _, s := range inst.Spec.Secrets {
			// We need to determine what the value for this Secret was in the shared
			// Secret resource.
			key := assets.KeyForSecret(inst.Namespace, &s)
			if _, mounted := mountedKeys[key]; mounted {
				continue
			}
			mountedKeys[key] = struct{}{}

			secretsPaths = append(secretsPaths, core_v1.KeyToPath{
				Key:  config.SanitizeLabelName(string(key)),
				Path: fmt.Sprintf("secrets/%s/%s/%s", inst.Namespace, s.Name, s.Key),
			})
		}

		for _, cm := range inst.Spec.ConfigMaps {
			// We need to determine what the value for this ConfigMap was in the shared
			// Secret resource.
			key := assets.KeyForConfigMap(inst.Namespace, &cm)
			if _, mounted := mountedKeys[key]; mounted {
				continue
			}
			mountedKeys[key] = struct{}{}

			secretsPaths = append(secretsPaths, core_v1.KeyToPath{
				Key:  config.SanitizeLabelName(string(key)),
				Path: fmt.Sprintf("configMaps/%s/%s/%s", inst.Namespace, cm.Name, cm.Key),
			})
		}
	}

	if len(secretsPaths) > 0 {
		// Load in references to Secrets and ConfigMaps.
		integrationSecretsName := fmt.Sprintf("%s-integrations-secrets", d.Agent.Name)

		integrationOpts.ExtraVolumes = append(integrationOpts.ExtraVolumes, core_v1.Volume{
			Name: integrationSecretsName,
			VolumeSource: core_v1.VolumeSource{
				Secret: &core_v1.SecretVolumeSource{
					// The reconcile-wide Secret holds all secrets and config maps
					// integrations may have used.
					SecretName: fmt.Sprintf("%s-secrets", d.Agent.Name),
					Items:      secretsPaths,
				},
			},
		})

		integrationOpts.ExtraVolumeMounts = append(integrationOpts.ExtraVolumeMounts, core_v1.VolumeMount{
			Name:      integrationSecretsName,
			MountPath: "/etc/grafana-agent/integrations",
			ReadOnly:  true,
		})
	}

	// Extra options to merge in.
	//
	// NOTE(rfratto): Merge order is important, as subsequent podTemplateOptions
	// have placeholders necessary to generate configs.
	var (
		metricsOpts = metricsPodTemplateOptions(name, d, 0)
		logsOpts    = logsPodTemplateOptions()
	)
	return mergePodTemplateOptions(&integrationOpts, &metricsOpts, &logsOpts)
}

// mergePodTemplateOptions merges the provided inputs into a single
// podTemplateOptions. Precedence for existing values is taken in input order;
// if an environment variable is defined in both inputs[0] and inputs[1], the
// value from inputs[0] is used.
func mergePodTemplateOptions(inputs ...*podTemplateOptions) podTemplateOptions {
	res := podTemplateOptions{
		ExtraSelectorLabels: make(map[string]string),
	}

	// Volumes are unique by both mount path or name. If a mount path already
	// exists, we want to ignore that volume and the respective volume mount
	// that uses it.

	var (
		mountNames  = map[string]struct{}{} // Consumed mount names
		mountPaths  = map[string]struct{}{} // Consumed mount paths
		volumeNames = map[string]struct{}{} // Consumed volume names
		varNames    = map[string]struct{}{} // Consumed variable names
	)

	for _, input := range inputs {
		for k, v := range input.ExtraSelectorLabels {
			if _, exist := res.ExtraSelectorLabels[k]; exist {
				continue
			}
			res.ExtraSelectorLabels[k] = v
		}

		// Merge in VolumeMounts before Volumes, allowing us to detect what volume
		// names specific to this input should be ignored.
		ignoreVolumes := map[string]struct{}{}

		for _, vm := range input.ExtraVolumeMounts {
			// Ignore a volume if the mount path or volume name already exists.
			var (
				_, exists  = mountNames[vm.Name]
				_, mounted = mountPaths[vm.MountPath]
			)
			if exists || mounted {
				ignoreVolumes[vm.Name] = struct{}{}
				continue
			}

			res.ExtraVolumeMounts = append(res.ExtraVolumeMounts, vm)
			mountNames[vm.Name] = struct{}{}
			mountPaths[vm.MountPath] = struct{}{}
		}

		// Merge in volumes that haven't been ignored or have a unique name.
		for _, v := range input.ExtraVolumes {
			if _, ignored := ignoreVolumes[v.Name]; ignored {
				continue
			} else if _, exists := volumeNames[v.Name]; exists {
				continue
			}

			res.ExtraVolumes = append(res.ExtraVolumes, v)
			volumeNames[v.Name] = struct{}{}
		}

		for _, ev := range input.ExtraEnvVars {
			if _, exists := varNames[ev.Name]; exists {
				continue
			}

			res.ExtraEnvVars = append(res.ExtraEnvVars, ev)
			varNames[ev.Name] = struct{}{}
		}
	}

	return res
}

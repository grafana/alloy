package operator

import (
	"context"
	"fmt"
	"strings"

	prom_operator "github.com/prometheus-operator/prometheus-operator/pkg/operator"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
)

const (
	defaultPortName = "http-metrics"
)

var (
	minShards                   int32 = 1
	minReplicas                 int32 = 1
	managedByOperatorLabel            = "app.kubernetes.io/managed-by"
	managedByOperatorLabelValue       = "grafana-agent-operator"
	versionLabelName                  = "app.kubernetes.io/version"
	managedByOperatorLabels           = map[string]string{
		managedByOperatorLabel: managedByOperatorLabelValue,
	}
	shardLabelName            = "operator.agent.grafana.com/shard"
	agentNameLabelName        = "operator.agent.grafana.com/name"
	agentTypeLabel            = "operator.agent.grafana.com/type"
	probeTimeoutSeconds int32 = 3
)

// deleteManagedResource deletes a managed resource. Ignores resources that are
// not managed.
func deleteManagedResource(ctx context.Context, cli client.Client, key client.ObjectKey, o client.Object) error {
	err := cli.Get(ctx, key, o)
	if k8s_errors.IsNotFound(err) || !isManagedResource(o) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to find stale resource %s: %w", key, err)
	}
	err = cli.Delete(ctx, o)
	if err != nil {
		return fmt.Errorf("failed to delete stale resource %s: %w", key, err)
	}
	return nil
}

// isManagedResource returns true if the given object has a managed-by
// grafana-agent-operator label.
func isManagedResource(obj client.Object) bool {
	labelValue := obj.GetLabels()[managedByOperatorLabel]
	return labelValue == managedByOperatorLabelValue
}

func governingServiceName(agentName string) string {
	return fmt.Sprintf("%s-operated", agentName)
}

func generateMetricsStatefulSetService(cfg *Config, d gragent.Deployment) *core_v1.Service {
	d = *d.DeepCopy()

	if d.Agent.Spec.PortName == "" {
		d.Agent.Spec.PortName = defaultPortName
	}

	return &core_v1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      governingServiceName(d.Agent.Name),
			Namespace: d.Agent.Namespace,
			OwnerReferences: []meta_v1.OwnerReference{{
				APIVersion:         d.Agent.APIVersion,
				Kind:               d.Agent.Kind,
				Name:               d.Agent.Name,
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
				UID:                d.Agent.UID,
			}},
			Labels: cfg.Labels.Merge(map[string]string{
				managedByOperatorLabel: managedByOperatorLabelValue,
				agentNameLabelName:     d.Agent.Name,
				"operated-agent":       "true",
			}),
		},
		Spec: core_v1.ServiceSpec{
			ClusterIP: "None",
			Ports: []core_v1.ServicePort{{
				Name:       d.Agent.Spec.PortName,
				Port:       8080,
				TargetPort: intstr.FromString(d.Agent.Spec.PortName),
			}},
			Selector: map[string]string{
				"app.kubernetes.io/name": "grafana-agent",
				agentNameLabelName:       d.Agent.Name,
			},
		},
	}
}

func generateMetricsStatefulSet(
	cfg *Config,
	name string,
	d gragent.Deployment,
	shard int32,
) (*apps_v1.StatefulSet, error) {

	d = *d.DeepCopy()

	opts := metricsPodTemplateOptions(name, d, shard)
	templateSpec, selector, err := generatePodTemplate(cfg, d.Agent.Name, d, opts)
	if err != nil {
		return nil, err
	}

	spec := &apps_v1.StatefulSetSpec{
		ServiceName:         governingServiceName(d.Agent.Name),
		Replicas:            d.Agent.Spec.Metrics.Replicas,
		PodManagementPolicy: apps_v1.ParallelPodManagement,
		UpdateStrategy: apps_v1.StatefulSetUpdateStrategy{
			Type: apps_v1.RollingUpdateStatefulSetStrategyType,
		},
		Selector: selector,
		Template: templateSpec,
	}

	ss := &apps_v1.StatefulSet{
		ObjectMeta: metadataFromPodTemplate(name, d, templateSpec),
		Spec:       *spec,
	}

	if deploymentUseVolumeClaimTemplate(&d) {
		storageSpec := d.Agent.Spec.Storage
		pvcTemplate := prom_operator.MakeVolumeClaimTemplate(storageSpec.VolumeClaimTemplate)
		if pvcTemplate.Name == "" {
			pvcTemplate.Name = fmt.Sprintf("%s-wal", name)
		}
		if storageSpec.VolumeClaimTemplate.Spec.AccessModes == nil {
			pvcTemplate.Spec.AccessModes = []core_v1.PersistentVolumeAccessMode{core_v1.ReadWriteOnce}
		} else {
			pvcTemplate.Spec.AccessModes = storageSpec.VolumeClaimTemplate.Spec.AccessModes
		}
		pvcTemplate.Spec.Resources = storageSpec.VolumeClaimTemplate.Spec.Resources
		pvcTemplate.Spec.Selector = storageSpec.VolumeClaimTemplate.Spec.Selector
		ss.Spec.VolumeClaimTemplates = append(ss.Spec.VolumeClaimTemplates, *pvcTemplate)
	}

	return ss, nil
}

func deploymentUseVolumeClaimTemplate(d *gragent.Deployment) bool {
	return d.Agent.Spec.Storage != nil && d.Agent.Spec.Storage.EmptyDir == nil
}

func metricsPodTemplateOptions(name string, d gragent.Deployment, shard int32) podTemplateOptions {
	shards := minShards
	if reqShards := d.Agent.Spec.Metrics.Shards; reqShards != nil && *reqShards > 1 {
		shards = *reqShards
	}

	walVolumeName := fmt.Sprintf("%s-wal", name)
	if d.Agent.Spec.Storage != nil {
		if d.Agent.Spec.Storage.VolumeClaimTemplate.Name != "" {
			walVolumeName = d.Agent.Spec.Storage.VolumeClaimTemplate.Name
		}
	}

	opts := podTemplateOptions{
		ExtraSelectorLabels: map[string]string{
			shardLabelName: fmt.Sprintf("%d", shard),
			agentTypeLabel: "metrics",
		},
		ExtraVolumeMounts: []core_v1.VolumeMount{{
			Name:      walVolumeName,
			ReadOnly:  false,
			MountPath: "/var/lib/grafana-agent/data",
		}},
		ExtraEnvVars: []core_v1.EnvVar{
			{
				Name:  "SHARD",
				Value: fmt.Sprintf("%d", shard),
			},
			{
				Name:  "SHARDS",
				Value: fmt.Sprintf("%d", shards),
			},
		},
	}

	// Add volumes if there's no PVC template
	storageSpec := d.Agent.Spec.Storage
	if storageSpec == nil {
		opts.ExtraVolumes = append(opts.ExtraVolumes, core_v1.Volume{
			Name: walVolumeName,
			VolumeSource: core_v1.VolumeSource{
				EmptyDir: &core_v1.EmptyDirVolumeSource{},
			},
		})
	} else if storageSpec.EmptyDir != nil {
		emptyDir := storageSpec.EmptyDir
		opts.ExtraVolumes = append(opts.ExtraVolumes, core_v1.Volume{
			Name: walVolumeName,
			VolumeSource: core_v1.VolumeSource{
				EmptyDir: emptyDir,
			},
		})
	}

	return opts
}

func metadataFromPodTemplate(name string, d gragent.Deployment, tmpl core_v1.PodTemplateSpec) meta_v1.ObjectMeta {
	labels := make(map[string]string, len(tmpl.Labels))
	for k, v := range tmpl.Labels {
		// do not put version label on the statefulset, as that will prevent us from updating it
		// in the future. Statefulset labels are immutable.
		if k != versionLabelName {
			labels[k] = v
		}
	}
	return meta_v1.ObjectMeta{
		Name:        name,
		Namespace:   d.Agent.Namespace,
		Labels:      labels,
		Annotations: prepareAnnotations(d.Agent.Annotations),
		OwnerReferences: []meta_v1.OwnerReference{{
			APIVersion:         d.Agent.APIVersion,
			Kind:               d.Agent.Kind,
			BlockOwnerDeletion: ptr.To(true),
			Controller:         ptr.To(true),
			Name:               d.Agent.Name,
			UID:                d.Agent.UID,
		}},
	}
}

// prepareAnnotations returns annotations that are safe to be added to a
// generated resource.
func prepareAnnotations(source map[string]string) map[string]string {
	res := make(map[string]string, len(source))
	for k, v := range source {
		// Ignore kubectl annotations so kubectl doesn't prune the resource we
		// generated.
		if !strings.HasPrefix(k, "kubectl.kubernetes.io/") {
			res[k] = v
		}
	}
	return res
}

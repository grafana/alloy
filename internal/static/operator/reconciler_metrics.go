package operator

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/go-jsonnet"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/clientutil"
	"github.com/grafana/agent/internal/static/operator/config"
)

// createMetricsConfigurationSecret creates the Grafana Agent metrics configuration and stores
// it into a secret.
func (r *reconciler) createMetricsConfigurationSecret(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	name := fmt.Sprintf("%s-config", d.Agent.Name)
	return r.createTelemetryConfigurationSecret(ctx, l, name, d, config.MetricsType)
}

func (r *reconciler) createTelemetryConfigurationSecret(
	ctx context.Context,
	l log.Logger,
	name string,
	d gragent.Deployment,
	ty config.Type,
) error {

	key := types.NamespacedName{
		Namespace: d.Agent.Namespace,
		Name:      name,
	}

	var shouldCreate bool
	switch ty {
	case config.MetricsType:
		shouldCreate = len(d.Metrics) > 0
	case config.LogsType:
		shouldCreate = len(d.Logs) > 0
	case config.IntegrationsType:
		shouldCreate = len(d.Integrations) > 0
	default:
		return fmt.Errorf("unknown telemetry type %s", ty)
	}

	// Delete the old Secret if one exists and we have nothing to create.
	if !shouldCreate {
		var secret core_v1.Secret
		return deleteManagedResource(ctx, r.Client, key, &secret)
	}

	rawConfig, err := config.BuildConfig(&d, ty)

	var jsonnetError jsonnet.RuntimeError
	if errors.As(err, &jsonnetError) {
		// Dump Jsonnet errors to the console to retain newlines and make them
		// easier to digest.
		fmt.Fprintf(os.Stderr, "%s", jsonnetError.Error())
	}
	if err != nil {
		return fmt.Errorf("unable to build config: %w", err)
	}

	const maxUncompressed = 100 * 1024 // only compress secrets over 100kB
	rawBytes := []byte(rawConfig)
	if len(rawBytes) > maxUncompressed {
		buf := &bytes.Buffer{}
		w := gzip.NewWriter(buf)
		if _, err = w.Write(rawBytes); err != nil {
			return fmt.Errorf("unable to compress config: %w", err)
		}
		if err = w.Close(); err != nil {
			return fmt.Errorf("closing gzip writer: %w", err)
		}
		rawBytes = buf.Bytes()
	}

	secret := core_v1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    r.config.Labels.Merge(managedByOperatorLabels),
			OwnerReferences: []v1.OwnerReference{{
				APIVersion:         d.Agent.APIVersion,
				BlockOwnerDeletion: ptr.To(true),
				Kind:               d.Agent.Kind,
				Name:               d.Agent.Name,
				UID:                d.Agent.UID,
			}},
		},
		Data: map[string][]byte{"agent.yml": rawBytes},
	}

	level.Info(l).Log("msg", "reconciling secret", "secret", secret.Name)
	err = clientutil.CreateOrUpdateSecret(ctx, r.Client, &secret)
	if err != nil {
		return fmt.Errorf("failed to reconcile secret: %w", err)
	}
	return nil
}

// createMetricsGoverningService creates the service that governs the (eventual)
// StatefulSet. It must be created before the StatefulSet.
func (r *reconciler) createMetricsGoverningService(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	svc := generateMetricsStatefulSetService(r.config, d)

	// Delete the old Service if one exists and we have no prometheus instances.
	if len(d.Metrics) == 0 {
		var service core_v1.Service
		key := types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}
		return deleteManagedResource(ctx, r.Client, key, &service)
	}

	level.Info(l).Log("msg", "reconciling statefulset service", "service", svc.Name)
	err := clientutil.CreateOrUpdateService(ctx, r.Client, svc)
	if err != nil {
		return fmt.Errorf("failed to reconcile statefulset governing service: %w", err)
	}
	return nil
}

// createMetricsStatefulSets creates a set of Grafana Agent StatefulSets, one per shard.
func (r *reconciler) createMetricsStatefulSets(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	shards := minShards
	if reqShards := d.Agent.Spec.Metrics.Shards; reqShards != nil && *reqShards > 1 {
		shards = *reqShards
	}

	// Keep track of generated stateful sets so we can delete ones that should
	// no longer exist.
	generated := make(map[string]struct{})

	for shard := int32(0); shard < shards; shard++ {
		// Don't generate anything if there weren't any instances.
		if len(d.Metrics) == 0 {
			continue
		}

		name := d.Agent.Name
		if shard > 0 {
			name = fmt.Sprintf("%s-shard-%d", name, shard)
		}

		ss, err := generateMetricsStatefulSet(r.config, name, d, shard)
		if err != nil {
			return fmt.Errorf("failed to generate statefulset for shard: %w", err)
		}

		level.Info(l).Log("msg", "reconciling statefulset", "statefulset", ss.Name)
		err = clientutil.CreateOrUpdateStatefulSet(ctx, r.Client, ss, l)
		if err != nil {
			return fmt.Errorf("failed to reconcile statefulset for shard: %w", err)
		}
		generated[ss.Name] = struct{}{}
	}

	// Clean up statefulsets that should no longer exist.
	var statefulSets apps_v1.StatefulSetList
	err := r.List(ctx, &statefulSets, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			managedByOperatorLabel: managedByOperatorLabelValue,
			agentNameLabelName:     d.Agent.Name,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to list statefulsets: %w", err)
	}
	for _, ss := range statefulSets.Items {
		if _, keep := generated[ss.Name]; keep || !isManagedResource(&ss) {
			continue
		}
		level.Info(l).Log("msg", "deleting stale statefulset", "name", ss.Name)
		if err := r.Delete(ctx, &ss); err != nil {
			return fmt.Errorf("failed to delete stale statefulset %s: %w", ss.Name, err)
		}
	}

	return nil
}

package operator

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/clientutil"
	"github.com/grafana/agent/internal/static/operator/config"
	apps_v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *reconciler) newIntegrationsDeploymentSecret(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	// The Deployment for integrations only has integrations where AllNodes is
	// false.
	d = deploymentIntegrationSubset(d, false)

	name := fmt.Sprintf("%s-integrations-deploy-config", d.Agent.Name)
	return r.createTelemetryConfigurationSecret(ctx, l, name, d, config.IntegrationsType)
}

func (r *reconciler) newIntegrationsDaemonSetSecret(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	// The DaemonSet for integrations only has integrations where AllNodes is
	// true.
	d = deploymentIntegrationSubset(d, true)

	name := fmt.Sprintf("%s-integrations-ds-config", d.Agent.Name)
	return r.createTelemetryConfigurationSecret(ctx, l, name, d, config.IntegrationsType)
}

func deploymentIntegrationSubset(d gragent.Deployment, allNodes bool) gragent.Deployment {
	res := *d.DeepCopy()

	filteredIntegrations := make([]gragent.IntegrationsDeployment, 0, len(d.Integrations))
	for _, i := range d.Integrations {
		if i.Instance.Spec.Type.AllNodes == allNodes {
			filteredIntegrations = append(filteredIntegrations, i)
		}
	}

	res.Integrations = filteredIntegrations
	return res
}

func (r *reconciler) newIntegrationsDeployment(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	// The Deployment for integrations only has integrations where AllNodes is
	// false.
	d = deploymentIntegrationSubset(d, false)

	name := fmt.Sprintf("%s-integrations-deploy", d.Agent.Name)
	deploy, err := newIntegrationsDeployment(r.config, name, d)
	if err != nil {
		return fmt.Errorf("failed to generate integrations Deployment: %w", err)
	}
	key := types.NamespacedName{Namespace: deploy.Namespace, Name: deploy.Name}

	if len(d.Integrations) == 0 {
		// There's nothing to deploy; delete anything that might've been deployed
		// from a previous reconcile.
		level.Info(l).Log("msg", "deleting integrations Deployment", "deploy", key)
		var deploy apps_v1.Deployment
		return deleteManagedResource(ctx, r.Client, key, &deploy)
	}

	level.Info(l).Log("msg", "reconciling integrations Deployment", "deploy", key)
	err = clientutil.CreateOrUpdateDeployment(ctx, r.Client, deploy, l)
	if err != nil {
		return fmt.Errorf("failed to reconcile integrations Deployment: %w", err)
	}
	return nil
}

func (r *reconciler) newIntegrationsDaemonSet(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	// The DaemonSet for integrations only has integrations where AllNodes is
	// true.
	d = deploymentIntegrationSubset(d, true)

	name := fmt.Sprintf("%s-integrations-ds", d.Agent.Name)
	ds, err := newIntegrationsDaemonSet(r.config, name, d)
	if err != nil {
		return fmt.Errorf("failed to generate integrations DaemonSet: %w", err)
	}
	key := types.NamespacedName{Namespace: ds.Namespace, Name: ds.Name}

	if len(d.Integrations) == 0 {
		// There's nothing to deploy; delete anything that might've been deployed
		// from a previous reconcile.
		level.Info(l).Log("msg", "deleting integrations DaemonSet", "ds", key)
		var ds apps_v1.DaemonSet
		return deleteManagedResource(ctx, r.Client, key, &ds)
	}

	level.Info(l).Log("msg", "reconciling integrations DaemonSet", "ds", key)
	err = clientutil.CreateOrUpdateDaemonSet(ctx, r.Client, ds, l)
	if err != nil {
		return fmt.Errorf("failed to reconcile integrations DaemonSet: %w", err)
	}
	return nil
}

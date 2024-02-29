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

// createLogsConfigurationSecret creates the Grafana Agent logs configuration
// and stores it into a secret.
func (r *reconciler) createLogsConfigurationSecret(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	name := fmt.Sprintf("%s-logs-config", d.Agent.Name)
	return r.createTelemetryConfigurationSecret(ctx, l, name, d, config.LogsType)
}

// createLogsDaemonSet creates a DaemonSet for logs.
func (r *reconciler) createLogsDaemonSet(
	ctx context.Context,
	l log.Logger,
	d gragent.Deployment,
) error {

	name := fmt.Sprintf("%s-logs", d.Agent.Name)
	ds, err := generateLogsDaemonSet(r.config, name, d)
	if err != nil {
		return fmt.Errorf("failed to generate DaemonSet: %w", err)
	}
	key := types.NamespacedName{Namespace: ds.Namespace, Name: ds.Name}

	if len(d.Logs) == 0 {
		// There's nothing to deploy; delete anything that might've been deployed
		// from a previous reconcile.
		var ds apps_v1.DaemonSet
		return deleteManagedResource(ctx, r.Client, key, &ds)
	}

	level.Info(l).Log("msg", "reconciling logs daemonset", "ds", key)
	err = clientutil.CreateOrUpdateDaemonSet(ctx, r.Client, ds, l)
	if err != nil {
		return fmt.Errorf("failed to reconcile statefulset governing service: %w", err)
	}
	return nil
}

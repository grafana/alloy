package operator

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"github.com/grafana/agent/internal/static/operator/assets"
	"github.com/grafana/agent/internal/static/operator/config"
	"github.com/grafana/agent/internal/static/operator/hierarchy"
	prom "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// buildHierarchy constructs a resource hierarchy starting from root.
func buildHierarchy(ctx context.Context, l log.Logger, cli client.Client, root *gragent.GrafanaAgent) (deployment gragent.Deployment, watchers []hierarchy.Watcher, err error) {
	deployment.Agent = root

	// search is used throughout BuildHierarchy, where it will perform a list for
	// a set of objects in the hierarchy and populate the watchers return
	// variable.
	search := func(resources []hierarchyResource) error {
		for _, res := range resources {
			sel, err := res.Find(ctx, cli)
			if err != nil {
				gvk, _ := apiutil.GVKForObject(res.List, cli.Scheme())
				return fmt.Errorf("failed to find %q resource: %w", gvk.String(), err)
			}

			watchers = append(watchers, hierarchy.Watcher{
				Object:   res.Selector.ObjectType,
				Owner:    client.ObjectKeyFromObject(root),
				Selector: sel,
			})
		}
		return nil
	}

	// Root resources
	var (
		metricInstances gragent.MetricsInstanceList
		logsInstances   gragent.LogsInstanceList
		integrations    gragent.IntegrationList
	)
	var roots = []hierarchyResource{
		{List: &metricInstances, Selector: root.MetricsInstanceSelector()},
		{List: &logsInstances, Selector: root.LogsInstanceSelector()},
		{List: &integrations, Selector: root.IntegrationsSelector()},
	}
	if err := search(roots); err != nil {
		return deployment, nil, err
	}

	// Metrics resources
	for _, metricsInst := range metricInstances.Items {
		var (
			serviceMonitors prom.ServiceMonitorList
			podMonitors     prom.PodMonitorList
			probes          prom.ProbeList
		)
		var children = []hierarchyResource{
			{List: &serviceMonitors, Selector: metricsInst.ServiceMonitorSelector()},
			{List: &podMonitors, Selector: metricsInst.PodMonitorSelector()},
			{List: &probes, Selector: metricsInst.ProbeSelector()},
		}
		if err := search(children); err != nil {
			return deployment, nil, err
		}

		deployment.Metrics = append(deployment.Metrics, gragent.MetricsDeployment{
			Instance:        metricsInst,
			ServiceMonitors: filterServiceMonitors(l, root, &serviceMonitors).Items,
			PodMonitors:     podMonitors.Items,
			Probes:          probes.Items,
		})
	}

	// Logs resources
	for _, logsInst := range logsInstances.Items {
		var (
			podLogs gragent.PodLogsList
		)
		var children = []hierarchyResource{
			{List: &podLogs, Selector: logsInst.PodLogsSelector()},
		}
		if err := search(children); err != nil {
			return deployment, nil, err
		}

		deployment.Logs = append(deployment.Logs, gragent.LogsDeployment{
			Instance: logsInst,
			PodLogs:  podLogs.Items,
		})
	}

	// Integration resources
	for _, integration := range integrations.Items {
		deployment.Integrations = append(deployment.Integrations, gragent.IntegrationsDeployment{
			Instance: integration,
		})
	}

	// Finally, find all referenced secrets
	secrets, secretWatchers, err := buildSecrets(ctx, cli, deployment)
	if err != nil {
		return deployment, nil, fmt.Errorf("failed to discover secrets: %w", err)
	}
	deployment.Secrets = secrets
	watchers = append(watchers, secretWatchers...)

	return deployment, watchers, nil
}

type hierarchyResource struct {
	List     client.ObjectList      // List to populate
	Selector gragent.ObjectSelector // Raw selector to use for list
}

func (hr *hierarchyResource) Find(ctx context.Context, cli client.Client) (hierarchy.Selector, error) {
	sel, err := toSelector(hr.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to build selector: %w", err)
	}
	err = hierarchy.List(ctx, cli, hr.List, sel)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}
	return sel, nil
}

func toSelector(os gragent.ObjectSelector) (hierarchy.Selector, error) {
	var res hierarchy.LabelsSelector
	res.NamespaceName = os.ParentNamespace

	if os.NamespaceSelector != nil {
		sel, err := metav1.LabelSelectorAsSelector(os.NamespaceSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid namespace selector: %w", err)
		}
		res.NamespaceLabels = sel
	}

	sel, err := metav1.LabelSelectorAsSelector(os.Labels)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %w", err)
	}
	res.Labels = sel
	return &res, nil
}

func filterServiceMonitors(l log.Logger, root *gragent.GrafanaAgent, list *prom.ServiceMonitorList) *prom.ServiceMonitorList {
	items := make([]*prom.ServiceMonitor, 0, len(list.Items))

Item:
	for _, item := range list.Items {
		if root.Spec.Metrics.ArbitraryFSAccessThroughSMs.Deny {
			for _, ep := range item.Spec.Endpoints {
				err := testForArbitraryFSAccess(ep)
				if err == nil {
					continue
				}

				level.Warn(l).Log(
					"msg", "skipping service monitor",
					"agent", client.ObjectKeyFromObject(root),
					"servicemonitor", client.ObjectKeyFromObject(item),
					"err", err,
				)
				continue Item
			}
		}
		items = append(items, item)
	}

	return &prom.ServiceMonitorList{
		TypeMeta: list.TypeMeta,
		ListMeta: *list.ListMeta.DeepCopy(),
		Items:    items,
	}
}

func testForArbitraryFSAccess(e prom.Endpoint) error {
	if e.BearerTokenFile != "" {
		return fmt.Errorf("it accesses file system via bearer token file which is disallowed via GrafanaAgent specification")
	}

	if e.TLSConfig == nil {
		return nil
	}

	if e.TLSConfig.CAFile != "" || e.TLSConfig.CertFile != "" || e.TLSConfig.KeyFile != "" {
		return fmt.Errorf("it accesses file system via TLS config which is disallowed via GrafanaAgent specification")
	}

	return nil
}

func buildSecrets(ctx context.Context, cli client.Client, deploy gragent.Deployment) (secrets assets.SecretStore, watchers []hierarchy.Watcher, err error) {
	secrets = make(assets.SecretStore)

	// KeySelector caches to make sure we don't create duplicate watchers.
	var (
		usedSecretSelectors    = map[hierarchy.KeySelector]struct{}{}
		usedConfigMapSelectors = map[hierarchy.KeySelector]struct{}{}
	)

	for _, ref := range config.AssetReferences(deploy) {
		var (
			objectList client.ObjectList
			sel        hierarchy.KeySelector
		)

		switch {
		case ref.Reference.Secret != nil:
			objectList = &corev1.SecretList{}
			sel = hierarchy.KeySelector{
				Namespace: ref.Namespace,
				Name:      ref.Reference.Secret.Name,
			}
		case ref.Reference.ConfigMap != nil:
			objectList = &corev1.ConfigMapList{}
			sel = hierarchy.KeySelector{
				Namespace: ref.Namespace,
				Name:      ref.Reference.ConfigMap.Name,
			}
		}

		gvk, _ := apiutil.GVKForObject(objectList, cli.Scheme())
		if err := hierarchy.List(ctx, cli, objectList, &sel); err != nil {
			return nil, nil, fmt.Errorf("failed to find %q resource: %w", gvk.String(), err)
		}

		err := meta.EachListItem(objectList, func(o runtime.Object) error {
			var value string

			switch o := o.(type) {
			case *corev1.Secret:
				rawValue, ok := o.Data[ref.Reference.Secret.Key]
				if !ok {
					return fmt.Errorf("no key %s in Secret %s", ref.Reference.Secret.Key, o.Name)
				}
				value = string(rawValue)
			case *corev1.ConfigMap:
				var (
					dataValue, dataFound     = o.Data[ref.Reference.ConfigMap.Key]
					binaryValue, binaryFound = o.BinaryData[ref.Reference.ConfigMap.Key]
				)

				if dataFound {
					value = dataValue
				} else if binaryFound {
					value = string(binaryValue)
				} else {
					return fmt.Errorf("no key %s in ConfigMap %s", ref.Reference.ConfigMap.Key, o.Name)
				}
			}

			secrets[assets.KeyForSelector(ref.Namespace, &ref.Reference)] = value
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to iterate over %q list: %w", gvk.String(), err)
		}

		switch {
		case ref.Reference.Secret != nil:
			if _, used := usedSecretSelectors[sel]; used {
				continue
			}
			watchers = append(watchers, hierarchy.Watcher{
				Object:   &corev1.Secret{},
				Owner:    client.ObjectKeyFromObject(deploy.Agent),
				Selector: &sel,
			})
			usedSecretSelectors[sel] = struct{}{}
		case ref.Reference.ConfigMap != nil:
			if _, used := usedConfigMapSelectors[sel]; used {
				continue
			}
			watchers = append(watchers, hierarchy.Watcher{
				Object:   &corev1.ConfigMap{},
				Owner:    client.ObjectKeyFromObject(deploy.Agent),
				Selector: &sel,
			})
			usedConfigMapSelectors[sel] = struct{}{}
		}
	}

	return secrets, watchers, nil
}

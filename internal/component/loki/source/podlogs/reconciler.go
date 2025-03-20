package podlogs

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/loki/source/kubernetes/kubetail"
	monitoringv1alpha2 "github.com/grafana/alloy/internal/component/loki/source/podlogs/internal/apis/monitoring/v1alpha2"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	prom_lbls "github.com/prometheus/prometheus/model/labels"
	promlabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/util/strutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The reconciler reconciles the state of PodLogs on Kubernetes with targets to
// collect logs from.
type reconciler struct {
	log     log.Logger
	tailer  *kubetail.Manager
	cluster cluster.Cluster

	reconcileMut             sync.RWMutex
	podLogsSelector          labels.Selector
	podLogsNamespaceSelector labels.Selector
	shouldDistribute         bool

	debugMut  sync.RWMutex
	debugInfo []DiscoveredPodLogs
}

// newReconciler creates a new reconciler which synchronizes targets with the
// provided tailer whenever Reconcile is called.
func newReconciler(l log.Logger, tailer *kubetail.Manager, cluster cluster.Cluster) *reconciler {
	return &reconciler{
		log:     l,
		tailer:  tailer,
		cluster: cluster,

		podLogsSelector:          labels.Everything(),
		podLogsNamespaceSelector: labels.Everything(),
	}
}

// UpdateSelectors updates the selectors used by the reconciler.
func (r *reconciler) UpdateSelectors(podLogs, namespace labels.Selector) {
	r.reconcileMut.Lock()
	defer r.reconcileMut.Unlock()

	r.podLogsSelector = podLogs
	r.podLogsNamespaceSelector = namespace
}

// SetDistribute configures whether targets are distributed amongst the cluster.
func (r *reconciler) SetDistribute(distribute bool) {
	r.reconcileMut.Lock()
	defer r.reconcileMut.Unlock()

	r.shouldDistribute = distribute
}

func (r *reconciler) getShouldDistribute() bool {
	r.reconcileMut.RLock()
	defer r.reconcileMut.RUnlock()

	return r.shouldDistribute
}

// Reconcile synchronizes the set of running kubetail targets with the set of
// discovered PodLogs.
func (r *reconciler) Reconcile(ctx context.Context, cli client.Client) error {
	var newDebugInfo []DiscoveredPodLogs
	var newTasks []*kubetail.Target

	listOpts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: r.podLogsSelector},
	}
	var podLogsList monitoringv1alpha2.PodLogsList
	if err := cli.List(ctx, &podLogsList, listOpts...); err != nil {
		return fmt.Errorf("could not list PodLogs: %w", err)
	}

	for _, podLogs := range podLogsList.Items {
		key := client.ObjectKeyFromObject(podLogs)

		// Skip over this podLogs if it doesn't match the namespace selector.
		podLogsNamespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: podLogs.Namespace}}
		if err := cli.Get(ctx, client.ObjectKeyFromObject(&podLogsNamespace), &podLogsNamespace); err != nil {
			level.Error(r.log).Log("msg", "failed to reconcile PodLogs", "operation", "get namespace", "key", key, "err", err)
			continue
		}
		if !r.podLogsNamespaceSelector.Matches(labels.Set(podLogsNamespace.Labels)) {
			continue
		}

		targets, discoveredPodLogs := r.reconcilePodLogs(ctx, cli, podLogs)

		newTasks = append(newTasks, targets...)
		newDebugInfo = append(newDebugInfo, discoveredPodLogs)
	}

	// Distribute targets if clustering is enabled.
	if r.getShouldDistribute() {
		newTasks = distributeTargets(r.cluster, newTasks)
	}

	if err := r.tailer.SyncTargets(ctx, newTasks); err != nil {
		level.Error(r.log).Log("msg", "failed to apply new tailers to run", "err", err)
	}

	r.debugMut.Lock()
	r.debugInfo = newDebugInfo
	r.debugMut.Unlock()

	return nil
}

func filterLabels(lbls prom_lbls.Labels, keysToKeep []string) prom_lbls.Labels {
	var res prom_lbls.Labels
	for _, k := range lbls {
		if slices.Contains(keysToKeep, k.Name) {
			res = append(res, prom_lbls.Label{Name: k.Name, Value: k.Value})
		}
	}
	sort.Sort(res)
	return res
}

func distributeTargets(c cluster.Cluster, targets []*kubetail.Target) []*kubetail.Target {
	if c == nil {
		return targets
	}

	peerCount := len(c.Peers())
	resCap := len(targets) + 1
	if peerCount != 0 {
		resCap = (len(targets) + 1) / peerCount
	}

	res := make([]*kubetail.Target, 0, resCap)

	for _, target := range targets {
		// Only take into account the labels necessary to uniquely identify a pod/container instance.
		// If we take into account more labels than necessary, there may be issues due to labels changing
		// over the lifetime of the pod.
		clusteringLabels := filterLabels(target.DiscoveryLabels(), kubetail.ClusteringLabels)
		peers, err := c.Lookup(shard.StringKey(clusteringLabels.String()), 1, shard.OpReadWrite)
		if err != nil {
			// This can only fail in case we ask for more owners than the
			// available peers. This will never happen, but in any case we fall
			// back to owning the target ourselves.
			res = append(res, target)
		}
		if len(peers) == 0 || peers[0].Self {
			res = append(res, target)
		}
	}

	return res
}

func (r *reconciler) reconcilePodLogs(ctx context.Context, cli client.Client, podLogs *monitoringv1alpha2.PodLogs) ([]*kubetail.Target, DiscoveredPodLogs) {
	var targets []*kubetail.Target

	discoveredPodLogs := DiscoveredPodLogs{
		Namespace:     podLogs.Namespace,
		Name:          podLogs.Name,
		LastReconcile: time.Now(),
	}

	key := client.ObjectKeyFromObject(podLogs)
	level.Debug(r.log).Log("msg", "reconciling PodLogs", "key", key)

	relabelRules, err := convertRelabelConfig(podLogs.Spec.RelabelConfigs)
	if err != nil {
		discoveredPodLogs.ReconcileError = fmt.Sprintf("invalid relabelings: %s", err)
		level.Error(r.log).Log("msg", "failed to reconcile PodLogs", "operation", "convert relabelings", "key", key, "err", err)
		return targets, discoveredPodLogs
	}

	sel, err := metav1.LabelSelectorAsSelector(&podLogs.Spec.Selector)
	if err != nil {
		discoveredPodLogs.ReconcileError = fmt.Sprintf("invalid Pod selector: %s", err)
		level.Error(r.log).Log("msg", "failed to reconcile PodLogs", "operation", "convert selector", "key", key, "err", err)
		return targets, discoveredPodLogs
	}

	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: sel},
	}

	var podList corev1.PodList
	if err := cli.List(ctx, &podList, opts...); err != nil {
		discoveredPodLogs.ReconcileError = fmt.Sprintf("failed to list Pods: %s", err)
		level.Error(r.log).Log("msg", "failed to reconcile PodLogs", "operation", "list Pods", "key", key, "err", err)
		return targets, discoveredPodLogs
	}

	namespaceSel, err := metav1.LabelSelectorAsSelector(&podLogs.Spec.NamespaceSelector)
	if err != nil {
		discoveredPodLogs.ReconcileError = fmt.Sprintf("invalid Pod namespaceSelector: %s", err)
		level.Error(r.log).Log("msg", "failed to reconcile PodLogs", "operation", "convert namespaceSelector", "key", key, "err", err)
		return targets, discoveredPodLogs
	}

	for _, pod := range podList.Items {
		discoveredPod := DiscoveredPod{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}

		// Skip over this pod if it doesn't match the namespace selector.
		namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: pod.Namespace}}
		if err := cli.Get(ctx, client.ObjectKeyFromObject(&namespace), &namespace); err != nil {
			level.Error(r.log).Log("msg", "failed to reconcile PodLogs", "operation", "get namespace for Pod", "key", key, "err", err)
			continue
		}
		if !namespaceSel.Matches(labels.Set(namespace.Labels)) {
			continue
		}

		level.Debug(r.log).Log("msg", "found matching Pod", "key", key, "pod", client.ObjectKeyFromObject(&pod))

		handleContainer := func(container *corev1.Container, initContainer bool) {
			targetLabels := buildTargetLabels(discoveredContainer{
				PodLogs:       podLogs,
				PodNamespace:  &namespace,
				Pod:           &pod,
				Container:     container,
				InitContainer: initContainer,
			})
			processedLabels, _ := relabel.Process(targetLabels.Copy(), relabelRules...)

			defaultJob := fmt.Sprintf("%s/%s:%s", podLogs.Namespace, podLogs.Name, container.Name)
			finalLabels, err := kubetail.PrepareLabels(processedLabels, defaultJob)

			if err != nil {
				discoveredPod.Containers = append(discoveredPod.Containers, DiscoveredContainer{
					DiscoveredLabels: targetLabels.Map(),
					Labels:           processedLabels.Map(),
					ReconcileError:   fmt.Sprintf("invalid labels: %s", err),
				})
				return
			}

			target := kubetail.NewTarget(targetLabels.Copy(), finalLabels)
			if len(processedLabels) != 0 {
				targets = append(targets, target)
			}

			discoveredPod.Containers = append(discoveredPod.Containers, DiscoveredContainer{
				DiscoveredLabels: targetLabels.Map(),
				Labels:           target.Labels().Map(),
			})
		}

		for _, container := range pod.Spec.InitContainers {
			handleContainer(&container, true)
		}
		for _, container := range pod.Spec.Containers {
			handleContainer(&container, false)
		}

		discoveredPodLogs.Pods = append(discoveredPodLogs.Pods, discoveredPod)
	}

	return targets, discoveredPodLogs
}

// DebugInfo returns the current debug information for the reconciler.
func (r *reconciler) DebugInfo() []DiscoveredPodLogs {
	r.debugMut.RLock()
	defer r.debugMut.RUnlock()

	return r.debugInfo
}

type discoveredContainer struct {
	PodLogs       *monitoringv1alpha2.PodLogs
	PodNamespace  *corev1.Namespace
	Pod           *corev1.Pod
	Container     *corev1.Container
	InitContainer bool
}

func buildTargetLabels(opts discoveredContainer) promlabels.Labels {
	targetLabels := promlabels.NewBuilder(nil)

	targetLabels.Set("__meta_kubernetes_podlogs_namespace", opts.PodLogs.Namespace)
	targetLabels.Set("__meta_kubernetes_podlogs_name", opts.PodLogs.Name)
	for key, value := range opts.PodLogs.Labels {
		key = strutil.SanitizeLabelName(key)
		targetLabels.Set("__meta_kubernetes_podlogs_label_"+key, value)
		targetLabels.Set("__meta_kubernetes_podlogs_labelpresent_"+key, "true")
	}
	for key, value := range opts.PodLogs.Annotations {
		key = strutil.SanitizeLabelName(key)
		targetLabels.Set("__meta_kubernetes_podlogs_annotation_"+key, value)
		targetLabels.Set("__meta_kubernetes_podlogs_annotationpresent_"+key, "true")
	}

	targetLabels.Set("__meta_kubernetes_namespace", opts.Pod.Namespace)
	for key, value := range opts.PodNamespace.Labels {
		key = strutil.SanitizeLabelName(key)
		targetLabels.Set("__meta_kubernetes_namespace_label_"+key, value)
		targetLabels.Set("__meta_kubernetes_namespace_labelpresent_"+key, "true")
	}
	for key, value := range opts.PodNamespace.Annotations {
		key = strutil.SanitizeLabelName(key)
		targetLabels.Set("__meta_kubernetes_namespace_annotation_"+key, value)
		targetLabels.Set("__meta_kubernetes_namespace_annotationpresent_"+key, "true")
	}

	targetLabels.Set("__meta_kubernetes_pod_name", opts.Pod.Name)
	targetLabels.Set("__meta_kubernetes_pod_ip", opts.Pod.Status.PodIP)
	for key, value := range opts.Pod.Labels {
		key = strutil.SanitizeLabelName(key)
		targetLabels.Set("__meta_kubernetes_pod_label_"+key, value)
		targetLabels.Set("__meta_kubernetes_pod_labelpresent_"+key, "true")
	}
	for key, value := range opts.Pod.Annotations {
		key = strutil.SanitizeLabelName(key)
		targetLabels.Set("__meta_kubernetes_pod_annotation_"+key, value)
		targetLabels.Set("__meta_kubernetes_pod_annotationpresent_"+key, "true")
	}
	targetLabels.Set("__meta_kubernetes_pod_container_init", fmt.Sprint(opts.InitContainer))
	targetLabels.Set("__meta_kubernetes_pod_container_name", opts.Container.Name)
	targetLabels.Set("__meta_kubernetes_pod_container_image", opts.Container.Image)
	targetLabels.Set("__meta_kubernetes_pod_ready", string(podReady(opts.Pod)))
	targetLabels.Set("__meta_kubernetes_pod_phase", string(opts.Pod.Status.Phase))
	targetLabels.Set("__meta_kubernetes_pod_node_name", opts.Pod.Spec.NodeName)
	targetLabels.Set("__meta_kubernetes_pod_host_ip", opts.Pod.Status.HostIP)
	targetLabels.Set("__meta_kubernetes_pod_uid", string(opts.Pod.UID))

	for _, ref := range opts.Pod.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			targetLabels.Set("__meta_kubernetes_pod_controller_kind", ref.Kind)
			targetLabels.Set("__meta_kubernetes_pod_controller_name", ref.Name)
			break
		}
	}

	// Add labels needed for collecting.
	targetLabels.Set(kubetail.LabelPodNamespace, opts.Pod.Namespace)
	targetLabels.Set(kubetail.LabelPodName, opts.Pod.Name)
	targetLabels.Set(kubetail.LabelPodContainerName, opts.Container.Name)
	targetLabels.Set(kubetail.LabelPodUID, string(opts.Pod.GetUID()))

	// Add default labels (job, instance)
	targetLabels.Set(model.InstanceLabel, fmt.Sprintf("%s/%s:%s", opts.Pod.Namespace, opts.Pod.Name, opts.Container.Name))
	targetLabels.Set(model.JobLabel, fmt.Sprintf("%s/%s", opts.PodLogs.Namespace, opts.PodLogs.Name))

	res := targetLabels.Labels()
	sort.Sort(res)
	return res
}

func podReady(pod *corev1.Pod) model.LabelValue {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return model.LabelValue(strings.ToLower(string(cond.Status)))
		}
	}
	return model.LabelValue(strings.ToLower(string(corev1.ConditionUnknown)))
}

type DiscoveredPodLogs struct {
	Namespace      string    `alloy:"namespace,attr"`
	Name           string    `alloy:"name,attr"`
	LastReconcile  time.Time `alloy:"last_reconcile,attr,optional"`
	ReconcileError string    `alloy:"reconcile_error,attr,optional"`

	Pods []DiscoveredPod `alloy:"pod,block"`
}

type DiscoveredPod struct {
	Namespace      string `alloy:"namespace,attr"`
	Name           string `alloy:"name,attr"`
	ReconcileError string `alloy:"reconcile_error,attr,optional"`

	Containers []DiscoveredContainer `alloy:"container,block"`
}

type DiscoveredContainer struct {
	DiscoveredLabels map[string]string `alloy:"discovered_labels,attr"`
	Labels           map[string]string `alloy:"labels,attr"`
	ReconcileError   string            `alloy:"reconcile_error,attr,optional"`
}

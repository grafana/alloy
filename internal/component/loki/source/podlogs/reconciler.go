package podlogs

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	promlabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/util/strutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component/loki/source/kubernetes/kubetail"
	monitoringv1alpha2 "github.com/grafana/alloy/internal/component/loki/source/podlogs/internal/apis/monitoring/v1alpha2"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
)

const (
	kubePodlogsNamespace         = "__meta_kubernetes_podlogs_namespace"
	kubePodlogsName              = "__meta_kubernetes_podlogs_name"
	kubePodlogsLabel             = "__meta_kubernetes_podlogs_label_"
	kubePodlogsLabelPresent      = "__meta_kubernetes_podlogs_labelpresent_"
	kubePodlogsAnnotation        = "__meta_kubernetes_podlogs_annotation_"
	kubePodlogsAnnotationPresent = "__meta_kubernetes_podlogs_annotationpresent_"

	kubeNamespace                  = "__meta_kubernetes_namespace"
	kubeNamespaceLabel             = "__meta_kubernetes_namespace_label_"
	kubeNamespaceLabelPresent      = "__meta_kubernetes_namespace_labelpresent_"
	kubeNamespaceAnnotation        = "__meta_kubernetes_namespace_annotation_"
	kubeNamespaceAnnotationPresent = "__meta_kubernetes_namespace_annotationpresent_"

	kubePodName              = "__meta_kubernetes_pod_name"
	kubePodIP                = "__meta_kubernetes_pod_ip"
	kubePodLabel             = "__meta_kubernetes_pod_label_"
	kubePodLabelPresent      = "__meta_kubernetes_pod_labelpresent_"
	kubePodAnnotation        = "__meta_kubernetes_pod_annotation_"
	kubePodAnnotationPresent = "__meta_kubernetes_pod_annotationpresent_"
	kubePodContainerInit     = "__meta_kubernetes_pod_container_init"
	kubePodContainerName     = "__meta_kubernetes_pod_container_name"
	kubePodContainerImage    = "__meta_kubernetes_pod_container_image"
	kubePodReady             = "__meta_kubernetes_pod_ready"
	kubePodPhase             = "__meta_kubernetes_pod_phase"
	kubePodNodeName          = "__meta_kubernetes_pod_node_name"
	kubePodHostIP            = "__meta_kubernetes_pod_host_ip"
	kubePodUID               = "__meta_kubernetes_pod_uid"
	kubePodControllerKind    = "__meta_kubernetes_pod_controller_kind"
	kubePodControllerName    = "__meta_kubernetes_pod_controller_name"
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
	nodeFilterEnabled        bool
	nodeFilterName           string

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

// UpdateNodeFilter updates the node filter configuration used by the reconciler.
func (r *reconciler) UpdateNodeFilter(enabled bool, nodeName string) {
	r.reconcileMut.Lock()
	defer r.reconcileMut.Unlock()

	r.nodeFilterEnabled = enabled
	r.nodeFilterName = nodeName
}

// getNodeFilterName returns the effective node name to filter by.
// If enabled but no node name is provided, it falls back to the NODE_NAME environment variable.
func (r *reconciler) getNodeFilterName() string {
	r.reconcileMut.RLock()
	defer r.reconcileMut.RUnlock()

	if !r.nodeFilterEnabled {
		return ""
	}

	if r.nodeFilterName != "" {
		return r.nodeFilterName
	}

	// Fall back to NODE_NAME environment variable
	return os.Getenv("NODE_NAME")
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

func filterLabels(lbls promlabels.Labels, keysToKeep []string) promlabels.Labels {
	var res promlabels.Labels
	for _, k := range lbls {
		if slices.Contains(keysToKeep, k.Name) {
			res = append(res, promlabels.Label{Name: k.Name, Value: k.Value})
		}
	}
	sort.Sort(res)
	return res
}

func distributeTargets(c cluster.Cluster, targets []*kubetail.Target) []*kubetail.Target {
	if c == nil {
		return targets
	}
	if !c.Ready() { // take no traffic if cluster is not yet ready
		return make([]*kubetail.Target, 0)
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

	// Add node filtering if enabled
	if nodeFilterName := r.getNodeFilterName(); nodeFilterName != "" {
		level.Debug(r.log).Log("msg", "applying node filter for pod discovery", "node", nodeFilterName, "key", key)
		opts = append(opts, client.MatchingFieldsSelector{
			Selector: fields.OneTermEqualSelector("spec.nodeName", nodeFilterName),
		})
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

	// Extract labels and annotations from the PodLogs object outside of the container loop to spend less time sanitizing labels.
	podLogsTargetLabels := buildPodLogsTargetLabels(podLogs)

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

		// Extract labels and annotations from the Pods object outside of the container loop to spend less time sanitizing labels.
		podTargetLabels := buildPodsAndNamespacesTargetLabels(podLogsTargetLabels, pod, namespace)

		handleContainer := func(container *corev1.Container, initContainer bool) {
			targetLabels := buildContainerTargetLabels(discoveredContainer{
				PodLogs:       podLogs,
				Pod:           &pod,
				Container:     container,
				InitContainer: initContainer,
			}, podTargetLabels)
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

// buildPodLogsTargetLabels builds the target labels for a PodLogs object.
func buildPodLogsTargetLabels(podLogs *monitoringv1alpha2.PodLogs) promlabels.Labels {
	podLogsTargetLabels := promlabels.NewBuilder(nil)
	podLogsTargetLabels.Set(kubePodlogsNamespace, podLogs.Namespace)
	podLogsTargetLabels.Set(kubePodlogsName, podLogs.Name)
	for key, value := range podLogs.Labels {
		key = strutil.SanitizeLabelName(key)
		podLogsTargetLabels.Set(kubePodlogsLabel+key, value)
		podLogsTargetLabels.Set(kubePodlogsLabelPresent+key, "true")
	}
	for key, value := range podLogs.Annotations {
		key = strutil.SanitizeLabelName(key)
		podLogsTargetLabels.Set(kubePodlogsAnnotation+key, value)
		podLogsTargetLabels.Set(kubePodlogsAnnotationPresent+key, "true")
	}
	return podLogsTargetLabels.Labels()
}

// buildPodsAndNamespacesTargetLabels builds the target labels for a Pod and its
// Namespace.
func buildPodsAndNamespacesTargetLabels(podLogsTargetLabels promlabels.Labels, pod corev1.Pod, namespace corev1.Namespace) promlabels.Labels {
	podTargetLabels := promlabels.NewBuilder(podLogsTargetLabels)

	// Namespace specific labels
	podTargetLabels.Set(kubeNamespace, pod.Namespace)
	for key, value := range namespace.Labels {
		key = strutil.SanitizeLabelName(key)
		podTargetLabels.Set(kubeNamespaceLabel+key, value)
		podTargetLabels.Set(kubeNamespaceLabelPresent+key, "true")
	}
	for key, value := range namespace.Annotations {
		key = strutil.SanitizeLabelName(key)
		podTargetLabels.Set(kubeNamespaceAnnotation+key, value)
		podTargetLabels.Set(kubeNamespaceAnnotationPresent+key, "true")
	}

	// Pod specific labels
	podTargetLabels.Set(kubePodUID, string(pod.UID))
	podTargetLabels.Set(kubePodName, pod.Name)
	podTargetLabels.Set(kubePodNodeName, pod.Spec.NodeName)
	podTargetLabels.Set(kubePodIP, pod.Status.PodIP)
	podTargetLabels.Set(kubePodHostIP, pod.Status.HostIP)
	podTargetLabels.Set(kubePodReady, string(podReady(&pod)))
	podTargetLabels.Set(kubePodPhase, string(pod.Status.Phase))

	for key, value := range pod.Labels {
		key = strutil.SanitizeLabelName(key)
		podTargetLabels.Set(kubePodLabel+key, value)
		podTargetLabels.Set(kubePodLabelPresent+key, "true")
	}

	for key, value := range pod.Annotations {
		key = strutil.SanitizeLabelName(key)
		podTargetLabels.Set(kubePodAnnotation+key, value)
		podTargetLabels.Set(kubePodAnnotationPresent+key, "true")
	}

	for _, ref := range pod.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			podTargetLabels.Set(kubePodControllerKind, ref.Kind)
			podTargetLabels.Set(kubePodControllerName, ref.Name)
			break
		}
	}

	// Add labels needed for collecting.
	podTargetLabels.Set(kubetail.LabelPodNamespace, pod.Namespace)
	podTargetLabels.Set(kubetail.LabelPodName, pod.Name)
	podTargetLabels.Set(kubetail.LabelPodUID, string(pod.GetUID()))

	return podTargetLabels.Labels()
}

type discoveredContainer struct {
	PodLogs       *monitoringv1alpha2.PodLogs
	Pod           *corev1.Pod
	Container     *corev1.Container
	InitContainer bool
}

// buildContainerTargetLabels builds the target labels for a container and merge it with prediscoveredLabels.
func buildContainerTargetLabels(opts discoveredContainer, prediscoveredLabels promlabels.Labels) promlabels.Labels {
	targetLabels := promlabels.NewBuilder(prediscoveredLabels)

	targetLabels.Set(kubePodContainerInit, fmt.Sprint(opts.InitContainer))
	targetLabels.Set(kubePodContainerName, opts.Container.Name)
	targetLabels.Set(kubePodContainerImage, opts.Container.Image)

	// Add labels needed for collecting.
	targetLabels.Set(kubetail.LabelPodContainerName, opts.Container.Name)

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

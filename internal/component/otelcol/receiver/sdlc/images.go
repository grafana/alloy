package sdlc

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/pager"
)

const rolloutImagesResolved = "images_resolved"

// controllerRevisionHashLabel marks StatefulSet and DaemonSet Pods, and the
// ControllerRevisions of DaemonSets, with their revision.
const controllerRevisionHashLabel = appsv1.ControllerRevisionHashLabelKey

type resolvedContainer struct {
	name            string
	init            bool
	image           string
	runtimeImageIDs []string
}
type imageCollection struct {
	workload  workload
	state     rolloutState
	cutoff    time.Time
	succeeded bool
	ids       map[string]map[string]bool
	// Deployment Pods belong to the revision through its ReplicaSets; other
	// workloads own their Pods directly and label them with hash.
	replicaSets map[string]bool
	hash        string
}

// Transform each bounded list page before the reflector accumulates its initial
// snapshot. Cache transforms alone run too late to bound initial list memory.
func pagedList(ctx context.Context, options metav1.ListOptions, list cache.ListWithContextFunc, transform cache.TransformFunc) (runtime.Object, error) {
	if options.ResourceVersion == "0" {
		// Kubernetes watch caches may ignore Limit for RV=0.
		options.ResourceVersion = ""
		options.ResourceVersionMatch = ""
	}
	options.Limit = 250
	pages := pager.New(func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		page, err := list(ctx, opts)
		if err != nil {
			return nil, err
		}
		metadata, err := meta.ListAccessor(page)
		if err != nil {
			return nil, err
		}
		result := &metainternal.List{ListMeta: metav1.ListMeta{ResourceVersion: metadata.GetResourceVersion(), Continue: metadata.GetContinue()}}
		err = meta.EachListItem(page, func(item runtime.Object) error {
			transformed, err := transform(item)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, transformed.(runtime.Object))
			return nil
		})
		return result, err
	})
	// Let the reflector restart an expired snapshot instead of fetching everything
	// in an unbounded fallback request.
	pages.FullListIfExpired = false
	result, _, err := pages.ListWithAlloc(ctx, options)
	return result, err
}

func newImageInformer(client kubernetes.Interface, example runtime.Object, list cache.ListWithContextFunc, watch cache.WatchFuncWithContext) cache.SharedIndexInformer {
	return newPagedInformer(client, example, list, watch, imageWatchTransform)
}

// The informer applies transform to watch events (and streamed lists) too.
func newPagedInformer(client any, example runtime.Object, list cache.ListWithContextFunc, watch cache.WatchFuncWithContext, transform cache.TransformFunc) cache.SharedIndexInformer {
	informer := cache.NewSharedIndexInformer(cache.ToListWatcherWithWatchListSemantics(&cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return pagedList(ctx, options, list, transform)
		},
		WatchFuncWithContext: watch,
	}, client), example, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	_ = informer.SetTransform(transform) // Only fails after the informer has started.
	return informer
}

// Keep only image-resolution fields in informer caches and handler queues. A
// cluster-wide Pod cache otherwise retains full specs, annotations, managed
// fields, and container state even though this receiver never uses them.
func imageWatchTransform(obj any) (any, error) {
	metadata := func(m metav1.ObjectMeta) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: m.Name, Namespace: m.Namespace, UID: m.UID,
			ResourceVersion: m.ResourceVersion, CreationTimestamp: m.CreationTimestamp,
			DeletionTimestamp: m.DeletionTimestamp, OwnerReferences: m.OwnerReferences}
	}
	statuses := func(input []corev1.ContainerStatus) []corev1.ContainerStatus {
		out := make([]corev1.ContainerStatus, 0, len(input))
		for _, status := range input {
			out = append(out, corev1.ContainerStatus{Name: status.Name, ImageID: status.ImageID})
		}
		return out
	}
	switch value := obj.(type) {
	case *corev1.Pod:
		return &corev1.Pod{TypeMeta: value.TypeMeta, ObjectMeta: metadata(value.ObjectMeta), Status: corev1.PodStatus{
			Phase: value.Status.Phase, ContainerStatuses: statuses(value.Status.ContainerStatuses),
			InitContainerStatuses: statuses(value.Status.InitContainerStatuses)}}, nil
	case *appsv1.ReplicaSet:
		meta := metadata(value.ObjectMeta)
		meta.Annotations = map[string]string{revisionAnnotation: value.Annotations[revisionAnnotation]}
		return &appsv1.ReplicaSet{TypeMeta: value.TypeMeta, ObjectMeta: meta}, nil
	default:
		return obj, nil
	}
}

func containerKey(name string, init bool) string { return strconv.FormatBool(init) + ":" + name }

// A single shared Pod/ReplicaSet watch serves collections opened only by newly
// observed rollout revisions. Initial discovery and subsequent scaling stay silent.
func (c *controller) watchImages(pods, replicaSets cache.SharedIndexInformer) error {
	_, err := pods.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.observePod(obj.(*corev1.Pod)) },
		UpdateFunc: func(_, obj any) { c.observePod(obj.(*corev1.Pod)) },
		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			if p, ok := obj.(*corev1.Pod); ok {
				c.mu.Lock()
				c.removePod(p)
				c.mu.Unlock()
			}
		},
	})
	if err != nil {
		return err
	}
	_, err = replicaSets.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.observeReplicaSet(obj.(*appsv1.ReplicaSet)) },
		UpdateFunc: func(_, obj any) { c.observeReplicaSet(obj.(*appsv1.ReplicaSet)) },
		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			if rs, ok := obj.(*appsv1.ReplicaSet); ok {
				c.mu.Lock()
				c.removeReplicaSet(rs)
				c.mu.Unlock()
			}
		},
	})
	return err
}

// Index owner relationships so watch callbacks never scan unrelated workloads
// while holding the lifecycle mutex. Pods can arrive before their ReplicaSet.
// An empty kind matches a controller of any kind.
func controllerUID(m metav1.Object, kind string) string {
	owner := metav1.GetControllerOf(m)
	if owner != nil && (kind == "" || owner.Kind == kind) {
		return string(owner.UID)
	}
	return ""
}
func (c *controller) removePod(p *corev1.Pod) {
	id := string(p.UID)
	if old := c.pods[id]; old != nil {
		key := controllerUID(old, "")
		delete(c.podsByOwner[key], id)
		if len(c.podsByOwner[key]) == 0 {
			delete(c.podsByOwner, key)
		}
	}
	delete(c.pods, id)
}
func (c *controller) removeReplicaSet(rs *appsv1.ReplicaSet) {
	id := string(rs.UID)
	if old := c.replicaSets[id]; old != nil {
		key := controllerUID(old, "Deployment")
		delete(c.replicaSetsByDeployment[key], id)
		if len(c.replicaSetsByDeployment[key]) == 0 {
			delete(c.replicaSetsByDeployment, key)
		}
	}
	delete(c.replicaSets, id)
}
func (c *controller) observePod(p *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removePod(p)
	ownerID := controllerUID(p, "")
	if ownerID == "" {
		return
	}
	c.pods[string(p.UID)] = p
	if c.podsByOwner[ownerID] == nil {
		c.podsByOwner[ownerID] = map[string]*corev1.Pod{}
	}
	c.podsByOwner[ownerID][string(p.UID)] = p
	// Owners are unique by UID: a ReplicaSet of a Deployment, or a workload.
	workloadID := ownerID
	if rs := c.replicaSets[ownerID]; rs != nil {
		workloadID = controllerUID(rs, "Deployment")
	}
	if collection := c.collections[workloadID]; collection != nil {
		collection.observe(p)
	}
}
func (c *controller) observeReplicaSet(rs *appsv1.ReplicaSet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeReplicaSet(rs)
	depID := controllerUID(rs, "Deployment")
	if depID == "" {
		return
	}
	id := string(rs.UID)
	c.replicaSets[id] = rs
	if c.replicaSetsByDeployment[depID] == nil {
		c.replicaSetsByDeployment[depID] = map[string]*appsv1.ReplicaSet{}
	}
	c.replicaSetsByDeployment[depID][id] = rs
	if collection := c.collections[depID]; collection != nil {
		collection.track(rs)
		if collection.replicaSets[id] {
			for _, p := range c.podsByOwner[id] {
				collection.observe(p)
			}
		}
	}
}
func (c *controller) openImages(w workload, state rolloutState) {
	uid := string(w.GetUID())
	collection := &imageCollection{workload: w.copy(), state: state, ids: map[string]map[string]bool{}, replicaSets: map[string]bool{}, hash: w.revisionHash()}
	c.collections[uid] = collection
	for id, rs := range c.replicaSetsByDeployment[uid] {
		collection.track(rs)
		if collection.replicaSets[id] {
			for _, p := range c.podsByOwner[id] {
				collection.observe(p)
			}
		}
	}
	for _, p := range c.podsByOwner[uid] {
		collection.observe(p)
	}
}
func (c *controller) closeImages(w workload, succeeded bool) {
	uid := string(w.GetUID())
	collection := c.collections[uid]
	if collection == nil {
		return
	}
	delete(c.collections, uid)
	collection.cutoff = c.now()
	collection.succeeded = succeeded
	// Use the closing replica count/selector, but retain the original template
	// and revision hash.
	collection.workload = w.copy()
	c.imageQueue.Add(collection)
}
func (x *imageCollection) track(rs *appsv1.ReplicaSet) {
	owner := metav1.GetControllerOf(rs)
	revision, _ := strconv.ParseInt(rs.Annotations[revisionAnnotation], 10, 64)
	if rs.Namespace == x.workload.GetNamespace() && owner != nil && owner.Kind == "Deployment" && owner.UID == x.workload.GetUID() && revision == x.state.revision {
		x.replicaSets[string(rs.UID)] = true
	}
}

// owns reports whether p runs the collection's revision.
func (x *imageCollection) owns(p *corev1.Pod) bool {
	owner := metav1.GetControllerOf(p)
	if p.Namespace != x.workload.GetNamespace() || owner == nil {
		return false
	}
	if x.hash != "" {
		return owner.UID == x.workload.GetUID() && p.Labels[controllerRevisionHashLabel] == x.hash
	}
	return owner.Kind == "ReplicaSet" && x.replicaSets[string(owner.UID)]
}
func (x *imageCollection) observe(p *corev1.Pod) bool {
	if !x.owns(p) || (!x.cutoff.IsZero() && p.CreationTimestamp.Time.After(x.cutoff)) {
		return false
	}
	complete := true
	collect := func(containers []corev1.Container, statuses []corev1.ContainerStatus, init bool) {
		for _, container := range containers {
			key := containerKey(container.Name, init)
			found := false
			for _, status := range statuses {
				if status.Name == container.Name && status.ImageID != "" {
					if x.ids[key] == nil {
						x.ids[key] = map[string]bool{}
					}
					x.ids[key][status.ImageID] = true
					found = true
				}
			}
			if !found {
				complete = false
			}
		}
	}
	collect(x.state.template.Spec.Containers, p.Status.ContainerStatuses, false)
	collect(x.state.template.Spec.InitContainers, p.Status.InitContainerStatuses, true)
	return complete
}

// Final reads are outside informer locks and do not delay lifecycle events.
// Failed reads produce an explicitly partial result, not an endless collection.
func (c *controller) resolveImages(ctx context.Context) {
	for {
		collection, shutdown := c.imageQueue.Get()
		if shutdown {
			return
		}
		if ctx.Err() == nil {
			c.finalizeImages(ctx, collection)
		}
		c.imageQueue.Forget(collection)
		c.imageQueue.Done(collection)
	}
}
func (c *controller) finalizeImages(ctx context.Context, x *imageCollection) {
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	complete := x.succeeded
	namespace := x.workload.GetNamespace()
	selector, err := metav1.LabelSelectorAsSelector(x.workload.selector())
	if err != nil {
		complete = false
	}
	readyPods := 0
	if err == nil && c.opts.client != nil {
		if x.hash == "" {
			sets, listErr := c.opts.client.AppsV1().ReplicaSets(namespace).List(readCtx, metav1.ListOptions{LabelSelector: selector.String()})
			if listErr != nil {
				complete = false
			} else {
				for i := range sets.Items {
					x.track(&sets.Items[i])
				}
			}
		}
		pods, listErr := c.opts.client.CoreV1().Pods(namespace).List(readCtx, metav1.ListOptions{LabelSelector: selector.String()})
		if listErr != nil {
			complete = false
		} else {
			for i := range pods.Items {
				p := &pods.Items[i]
				if !x.owns(p) || p.CreationTimestamp.Time.After(x.cutoff) || p.DeletionTimestamp != nil || p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded {
					continue
				}
				readyPods++
				if !x.observe(p) {
					complete = false
				}
			}
		}
	} else {
		complete = false
	}
	desired := x.workload.desiredPods()
	if desired == 0 || readyPods < int(desired) {
		complete = false
	}
	rolloutID := c.rolloutID(x.workload, x.state.revision)
	var containers []resolvedContainer
	add := func(list []corev1.Container, init bool) {
		for _, container := range list {
			ids := []string{}
			for id := range x.ids[containerKey(container.Name, init)] {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			if len(ids) == 0 {
				complete = false
			}
			containers = append(containers, resolvedContainer{name: container.Name, init: init, image: container.Image, runtimeImageIDs: ids})
		}
	}
	add(x.state.template.Spec.Containers, false)
	add(x.state.template.Spec.InitContainers, true)
	event := rolloutEvent(x.workload, x.state, rolloutImagesResolved, rolloutID, x.cutoff)
	event.summary = imagesSummary(containers, complete)
	event.diff = func(diff pcommon.Map) {
		c.putRolloutDiff(diff, x.workload, x.state, rolloutID)
		diff.PutBool("complete", complete)
		resolved := diff.PutEmptySlice("resolved_containers")
		for _, container := range containers {
			m := resolved.AppendEmpty().SetEmptyMap()
			m.PutStr("name", container.name)
			m.PutBool("init", container.init)
			m.PutStr("image", container.image)
			ids := m.PutEmptySlice("runtime_image_ids")
			for _, id := range container.runtimeImageIDs {
				ids.AppendEmpty().SetStr(id)
			}
		}
	}
	c.enqueueChange(event)
}

// imagesSummary lists runtime image IDs per container, e.g.
// "Resolved runtime images: web=sha256:abc, init:setup=sha256:def".
func imagesSummary(containers []resolvedContainer, complete bool) string {
	parts := make([]string, 0, len(containers))
	for _, container := range containers {
		name := container.name
		if container.init {
			name = "init:" + name
		}
		ids := "unresolved"
		if len(container.runtimeImageIDs) > 0 {
			ids = strings.Join(container.runtimeImageIDs, "|")
		}
		parts = append(parts, name+"="+ids)
	}
	summary := "Resolved runtime images: " + strings.Join(parts, ", ")
	if !complete {
		summary = "Partially resolved runtime images: " + strings.Join(parts, ", ")
	}
	return summary
}

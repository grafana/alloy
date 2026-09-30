package k8s_workloads

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

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

type resolvedContainer struct {
	Name            string   `json:"name"`
	Init            bool     `json:"init"`
	Image           string   `json:"image"`
	RuntimeImageIDs []string `json:"runtime_image_ids"`
}
type imagesEvent struct {
	Name       string              `json:"name"`
	UID        string              `json:"uid"`
	RolloutID  string              `json:"rollout_id"`
	Revision   int64               `json:"revision"`
	ObservedAt string              `json:"observed_at"`
	Complete   bool                `json:"complete"`
	Containers []resolvedContainer `json:"containers"`
}
type imageCollection struct {
	deployment  *appsv1.Deployment
	state       rolloutState
	cutoff      time.Time
	succeeded   bool
	ids         map[string]map[string]bool
	replicaSets map[string]bool
}

// Transform each bounded list page before the reflector accumulates its initial
// snapshot. Cache transforms alone run too late to bound initial list memory.
func imageList(ctx context.Context, options metav1.ListOptions, list cache.ListWithContextFunc) (runtime.Object, error) {
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
			transformed, err := imageWatchTransform(item)
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
	return cache.NewSharedIndexInformer(cache.ToListWatcherWithWatchListSemantics(&cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return imageList(ctx, options, list)
		},
		WatchFuncWithContext: watch,
	}, client), example, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
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
				delete(c.pods, string(p.UID))
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
				delete(c.replicaSets, string(rs.UID))
				c.mu.Unlock()
			}
		},
	})
	return err
}
func (c *controller) observePod(p *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pods[string(p.UID)] = p
	for _, collection := range c.collections {
		collection.observe(p)
	}
}
func (c *controller) observeReplicaSet(rs *appsv1.ReplicaSet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replicaSets[string(rs.UID)] = rs
	for _, collection := range c.collections {
		collection.track(rs)
		for _, p := range c.pods {
			collection.observe(p)
		}
	}
}
func (c *controller) openImages(d *appsv1.Deployment, state rolloutState) {
	collection := &imageCollection{deployment: d.DeepCopy(), state: state, ids: map[string]map[string]bool{}, replicaSets: map[string]bool{}}
	c.collections[string(d.UID)] = collection
	for _, rs := range c.replicaSets {
		collection.track(rs)
	}
	for _, p := range c.pods {
		collection.observe(p)
	}
}
func (c *controller) closeImages(d *appsv1.Deployment, succeeded bool) {
	collection := c.collections[string(d.UID)]
	if collection == nil {
		return
	}
	delete(c.collections, string(d.UID))
	collection.cutoff = c.now()
	collection.succeeded = succeeded
	// Use the closing replica count/selector, but retain the original template.
	collection.deployment = d.DeepCopy()
	c.imageQueue.Add(collection)
}
func (x *imageCollection) track(rs *appsv1.ReplicaSet) {
	owner := metav1.GetControllerOf(rs)
	revision, _ := strconv.ParseInt(rs.Annotations[revisionAnnotation], 10, 64)
	if rs.Namespace == x.deployment.Namespace && owner != nil && owner.Kind == "Deployment" && owner.UID == x.deployment.UID && revision == x.state.revision {
		x.replicaSets[string(rs.UID)] = true
	}
}
func (x *imageCollection) observe(p *corev1.Pod) bool {
	owner := metav1.GetControllerOf(p)
	if p.Namespace != x.deployment.Namespace || owner == nil || owner.Kind != "ReplicaSet" || !x.replicaSets[string(owner.UID)] || (!x.cutoff.IsZero() && p.CreationTimestamp.Time.After(x.cutoff)) {
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
	selector, err := metav1.LabelSelectorAsSelector(x.deployment.Spec.Selector)
	if err != nil {
		complete = false
	}
	readyPods := 0
	if err == nil && c.opts.client != nil {
		sets, listErr := c.opts.client.AppsV1().ReplicaSets(x.deployment.Namespace).List(readCtx, metav1.ListOptions{LabelSelector: selector.String()})
		if listErr != nil {
			complete = false
		} else {
			for i := range sets.Items {
				x.track(&sets.Items[i])
			}
		}
		pods, listErr := c.opts.client.CoreV1().Pods(x.deployment.Namespace).List(readCtx, metav1.ListOptions{LabelSelector: selector.String()})
		if listErr != nil {
			complete = false
		} else {
			for i := range pods.Items {
				p := &pods.Items[i]
				owner := metav1.GetControllerOf(p)
				if owner == nil || !x.replicaSets[string(owner.UID)] || p.CreationTimestamp.Time.After(x.cutoff) || p.DeletionTimestamp != nil || p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded {
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
	desired := int32(1)
	if x.deployment.Spec.Replicas != nil {
		desired = *x.deployment.Spec.Replicas
	}
	if desired == 0 || readyPods < int(desired) {
		complete = false
	}
	payload := imagesEvent{Name: x.deployment.Name, UID: string(x.deployment.UID), RolloutID: stableID(c.opts.clusterUID, string(x.deployment.UID), fmt.Sprint(x.state.revision)), Revision: x.state.revision, ObservedAt: x.cutoff.UTC().Format(time.RFC3339Nano), Complete: complete, Containers: []resolvedContainer{}}
	add := func(containers []corev1.Container, init bool) {
		for _, container := range containers {
			ids := []string{}
			for id := range x.ids[containerKey(container.Name, init)] {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			if len(ids) == 0 {
				payload.Complete = false
			}
			payload.Containers = append(payload.Containers, resolvedContainer{Name: container.Name, Init: init, Image: container.Image, RuntimeImageIDs: ids})
		}
	}
	add(x.state.template.Spec.Containers, false)
	add(x.state.template.Spec.InitContainers, true)
	c.enqueuePayload(x.deployment, rolloutImagesResolved, payload.RolloutID, x.cutoff, payload)
}

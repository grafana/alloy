package k8s_workloads

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
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

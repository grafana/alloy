package sdlc

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// controllerRevision is what the index retains of a ControllerRevision.
type controllerRevision struct {
	revision int64
	hash     string // controller-revision-hash label
}

// StatefulSets and DaemonSets record their revisions as ControllerRevisions;
// unlike Deployments, the workload itself carries no ordered revision number.
func (c *controller) controllerRevisionInformer() (cache.SharedIndexInformer, error) {
	api := c.opts.client.AppsV1().ControllerRevisions(metav1.NamespaceAll)
	informer := newPagedInformer(c.opts.client, &appsv1.ControllerRevision{}, func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
		return api.List(ctx, options)
	}, api.Watch, imageWatchTransform)
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { c.observeControllerRevision(obj.(*appsv1.ControllerRevision)) },
		UpdateFunc: func(_, obj any) { c.observeControllerRevision(obj.(*appsv1.ControllerRevision)) },
		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			if cr, ok := obj.(*appsv1.ControllerRevision); ok {
				c.mu.Lock()
				delete(c.revisions[controllerUID(cr, "")], cr.Name)
				c.mu.Unlock()
			}
		},
	})
	return informer, err
}

// A workload update and its new ControllerRevision arrive on separate watches
// in either order, so a revision also re-evaluates its owner. Rollbacks reuse a
// ControllerRevision and raise its revision number.
func (c *controller) observeControllerRevision(cr *appsv1.ControllerRevision) {
	c.mu.Lock()
	defer c.mu.Unlock()
	owner := controllerUID(cr, "")
	if owner == "" {
		return
	}
	if c.revisions[owner] == nil {
		c.revisions[owner] = map[string]controllerRevision{}
	}
	c.revisions[owner][cr.Name] = controllerRevision{revision: cr.Revision, hash: cr.Labels[controllerRevisionHashLabel]}
	if obj := c.workloads[owner]; obj != nil {
		c.observeRollout(c.workload(obj), false)
	}
}

// workload is obj, a Deployment, StatefulSet or DaemonSet, with its revision
// resolved. Callers hold c.mu.
func (c *controller) workload(obj runtime.Object) workload {
	switch value := obj.(type) {
	case *appsv1.Deployment:
		return deploymentWorkload{value}
	case *appsv1.StatefulSet:
		return statefulSetWorkload{StatefulSet: value, revisioned: revisioned{c.revisions[string(value.UID)][value.Status.UpdateRevision]}}
	case *appsv1.DaemonSet:
		return daemonSetWorkload{DaemonSet: value, revisioned: revisioned{newestRevision(c.revisions[string(value.UID)])}}
	}
	return nil
}

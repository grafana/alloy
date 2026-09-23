package kubernetes_rollouts

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"

type controllerOptions struct {
	logger      *slog.Logger
	client      kubernetes.Interface
	clusterName string
	clusterUID  string
	emit        func(context.Context, func() eventBatch) error
}

type controller struct {
	opts controllerOptions

	deployments appslisters.DeploymentLister
	replicaSets appslisters.ReplicaSetLister
	pods        corelisters.PodLister
	queue       workqueue.TypedRateLimitingInterface[string]

	stateMu sync.Mutex
	states  map[types.UID]rolloutState
	keys    map[string]types.UID
}

type rolloutState struct {
	generation int64
	revision   string
	phase      rolloutPhase
	digests    map[string]struct{}
	images     []imageData
	deployment *appsv1.Deployment
	replicaSet *appsv1.ReplicaSet
}

func newController(opts controllerOptions) *controller {
	return &controller{
		opts:   opts,
		queue:  workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		states: make(map[types.UID]rolloutState),
		keys:   make(map[string]types.UID),
	}
}

func (c *controller) run(ctx context.Context) error {
	clusterUID := c.opts.clusterUID
	if clusterUID == "" {
		ns, err := c.opts.client.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("discovering k8s.cluster.uid from the kube-system namespace: %w", err)
		}
		clusterUID = string(ns.UID)
	}

	factory := informers.NewSharedInformerFactory(c.opts.client, 0)
	deploymentInformer := factory.Apps().V1().Deployments()
	replicaSetInformer := factory.Apps().V1().ReplicaSets()
	podInformer := factory.Core().V1().Pods()
	c.deployments = deploymentInformer.Lister()
	c.replicaSets = replicaSetInformer.Lister()
	c.pods = podInformer.Lister()

	if _, err := deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueObject, UpdateFunc: func(_, obj any) { c.enqueueObject(obj) }, DeleteFunc: c.enqueueObject,
	}); err != nil {
		return fmt.Errorf("adding Deployment event handler: %w", err)
	}
	if _, err := replicaSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueReplicaSet, UpdateFunc: func(_, obj any) { c.enqueueReplicaSet(obj) }, DeleteFunc: c.enqueueReplicaSet,
	}); err != nil {
		return fmt.Errorf("adding ReplicaSet event handler: %w", err)
	}
	if _, err := podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueuePod, UpdateFunc: func(_, obj any) { c.enqueuePod(obj) }, DeleteFunc: c.enqueuePod,
	}); err != nil {
		return fmt.Errorf("adding Pod event handler: %w", err)
	}

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), deploymentInformer.Informer().HasSynced, replicaSetInformer.Informer().HasSynced, podInformer.Informer().HasSynced) {
		return fmt.Errorf("informer cache sync failed")
	}

	defer c.queue.ShutDown()
	go func() {
		<-ctx.Done()
		c.queue.ShutDown()
	}()

	for {
		key, shutdown := c.queue.Get()
		if shutdown {
			return nil
		}
		err := c.reconcile(ctx, clusterUID, key)
		c.queue.Done(key)
		if err != nil {
			c.opts.logger.Error("failed to reconcile Kubernetes Deployment rollout", "key", key, "err", err)
			c.queue.AddRateLimited(key)
		} else {
			c.queue.Forget(key)
		}
	}
}

func (c *controller) enqueueObject(obj any) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err == nil {
		c.queue.Add(key)
	}
}

func (c *controller) enqueueReplicaSet(obj any) {
	rs, ok := objectFromEvent[*appsv1.ReplicaSet](obj)
	if !ok {
		return
	}
	if owner := metav1.GetControllerOf(rs); owner != nil && owner.Kind == "Deployment" {
		c.queue.Add(rs.Namespace + "/" + owner.Name)
	}
}

func (c *controller) enqueuePod(obj any) {
	pod, ok := objectFromEvent[*corev1.Pod](obj)
	if !ok {
		return
	}
	owner := metav1.GetControllerOf(pod)
	if owner == nil || owner.Kind != "ReplicaSet" {
		return
	}
	rs, err := c.replicaSets.ReplicaSets(pod.Namespace).Get(owner.Name)
	if err == nil {
		c.enqueueReplicaSet(rs)
	}
}

func objectFromEvent[T any](obj any) (T, bool) {
	value, ok := obj.(T)
	if ok {
		return value, true
	}
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		value, ok = tombstone.Obj.(T)
		return value, ok
	}
	var zero T
	return zero, false
}

func (c *controller) reconcile(ctx context.Context, clusterUID, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	deployment, err := c.deployments.Deployments(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.forgetDeployment(key)
			return nil
		}

		return err
	}

	c.stateMu.Lock()
	if oldUID, ok := c.keys[key]; ok && oldUID != deployment.UID {
		delete(c.states, oldUID)
	}
	c.keys[key] = deployment.UID
	c.stateMu.Unlock()

	replicaSets, err := c.replicaSets.ReplicaSets(namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	ownedReplicaSets := make([]*appsv1.ReplicaSet, 0)
	for _, rs := range replicaSets {
		owner := metav1.GetControllerOf(rs)
		if owner != nil && owner.UID == deployment.UID {
			ownedReplicaSets = append(ownedReplicaSets, rs)
		}
	}

	revision, activeRS := selectReplicaSet(deployment, ownedReplicaSets)
	if activeRS != nil && !podSpecsUseSameImages(deployment.Spec.Template.Spec, activeRS.Spec.Template.Spec) {
		revision = ""
		activeRS = nil
	}
	images := collectImages(deployment.Spec.Template.Spec, activeRS, c.pods)
	phase := deploymentPhase(deployment)

	c.stateMu.Lock()
	previous, seen := c.states[deployment.UID]
	current := rolloutState{
		generation: deployment.Generation,
		revision:   revision,
		phase:      phase,
		digests:    digestKeys(images),
		images:     append([]imageData(nil), images...),
		deployment: deployment.DeepCopy(),
	}
	if activeRS != nil {
		current.replicaSet = activeRS.DeepCopy()
	}
	c.stateMu.Unlock()

	if seen && previous.generation != deployment.Generation && previous.phase == phaseStarted {
		if err := c.opts.emit(ctx, func() eventBatch {
			return buildEventBatch(eventData{
				clusterUID: clusterUID, clusterName: c.opts.clusterName, deployment: previous.deployment,
				replicaSet: previous.replicaSet, generation: previous.generation, revision: previous.revision,
				phase: phaseSuperseded, images: previous.images,
			})
		}); err != nil {
			return err
		}
	}

	if !seen || previous.generation != deployment.Generation || previous.phase != phase {
		if err := c.opts.emit(ctx, func() eventBatch {
			return buildEventBatch(eventData{
				clusterUID: clusterUID, clusterName: c.opts.clusterName, deployment: deployment,
				replicaSet: activeRS, revision: revision, phase: phase, images: images,
			})
		}); err != nil {
			return err
		}
	}

	for _, image := range images {
		if image.digest == "" {
			continue
		}
		digestKey := image.container + "\x00" + image.digest
		if seen && previous.generation == deployment.Generation {
			if _, ok := previous.digests[digestKey]; ok {
				continue
			}
		}
		resolvedImage := image
		if err := c.opts.emit(ctx, func() eventBatch {
			return buildEventBatch(eventData{
				clusterUID: clusterUID, clusterName: c.opts.clusterName, deployment: deployment,
				replicaSet: activeRS, revision: revision, phase: phaseImageResolved, images: []imageData{resolvedImage},
			})
		}); err != nil {
			return err
		}
	}
	c.stateMu.Lock()
	c.states[deployment.UID] = current
	c.stateMu.Unlock()
	return nil
}

func (c *controller) forgetDeployment(key string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if uid, ok := c.keys[key]; ok {
		delete(c.states, uid)
		delete(c.keys, key)
	}
}

func selectReplicaSet(deployment *appsv1.Deployment, replicaSets []*appsv1.ReplicaSet) (string, *appsv1.ReplicaSet) {
	wanted := deployment.Annotations[deploymentRevisionAnnotation]
	var selected *appsv1.ReplicaSet
	selectedRevision := int64(-1)
	for _, rs := range replicaSets {
		revision := rs.Annotations[deploymentRevisionAnnotation]
		if wanted != "" && revision == wanted {
			return wanted, rs
		}
		n, err := strconv.ParseInt(revision, 10, 64)
		if err == nil && n > selectedRevision {
			selected, selectedRevision = rs, n
		}
	}
	if wanted == "" {
		wanted = strconv.FormatInt(deployment.Generation, 10)
	}
	if selected != nil && selected.Annotations[deploymentRevisionAnnotation] != "" {
		wanted = selected.Annotations[deploymentRevisionAnnotation]
	}
	return wanted, selected
}

func podSpecsUseSameImages(left, right corev1.PodSpec) bool {
	return containerImages(left.Containers, left.InitContainers) == containerImages(right.Containers, right.InitContainers)
}

func containerImages(containers, initContainers []corev1.Container) string {
	images := make([]string, 0, len(containers)+len(initContainers))
	for _, container := range containers {
		images = append(images, "container\x00"+container.Name+"\x00"+container.Image)
	}
	for _, container := range initContainers {
		images = append(images, "init\x00"+container.Name+"\x00"+container.Image)
	}
	sort.Strings(images)
	return strings.Join(images, "\x00")
}

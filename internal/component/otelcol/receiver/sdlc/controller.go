package sdlc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const revisionAnnotation = "deployment.kubernetes.io/revision"

type controllerOptions struct {
	logger                  *slog.Logger
	client                  kubernetes.Interface
	metadata                metadata.Interface
	clusterName, clusterUID string
	configMapContent        string // empty disables ConfigMap tracking
	secrets                 bool
	statefulSets            bool
	daemonSets              bool
	exclude                 excluder
	extract                 extractor
	emit                    func(context.Context, func() eventBatch) error
	now                     func() time.Time
}

type rolloutState struct {
	revision int64
	// previousRevision is the revision this rollout replaced, or 0 if unknown.
	previousRevision int64
	// startedAt is when the rollout was observed; zero for rollouts already in
	// flight when the component started.
	startedAt time.Time
	template  corev1.PodTemplateSpec
	// metadata is the workload metadata snapshots show, as observed with the
	// revision.
	metadata metav1.ObjectMeta
	status   string
	stalled  bool
}

type controller struct {
	mu                      sync.Mutex
	podsByOwner             map[string]map[string]*corev1.Pod
	replicaSetsByDeployment map[string]map[string]*appsv1.ReplicaSet
	pods                    map[string]*corev1.Pod
	replicaSets             map[string]*appsv1.ReplicaSet
	collections             map[string]*imageCollection
	imageQueue              workqueue.TypedRateLimitingInterface[*imageCollection]
	opts                    controllerOptions
	rollouts                map[string]rolloutState
	// revisions indexes ControllerRevisions by owner UID and name; workloads
	// holds the latest object of each tracked workload, so StatefulSets and
	// DaemonSets are re-evaluated when their ControllerRevisions change.
	revisions map[string]map[string]controllerRevision
	workloads map[string]runtime.Object
	// excluded holds the UIDs of workloads that exclude rules currently match.
	excluded map[string]bool
	queue    workqueue.TypedRateLimitingInterface[*eventBatch]
	now      func() time.Time
}

func newController(opts controllerOptions) *controller {
	if opts.now == nil {
		opts.now = time.Now
	}
	if opts.logger == nil {
		opts.logger = slog.Default()
	}
	return &controller{podsByOwner: map[string]map[string]*corev1.Pod{}, replicaSetsByDeployment: map[string]map[string]*appsv1.ReplicaSet{}, pods: map[string]*corev1.Pod{}, replicaSets: map[string]*appsv1.ReplicaSet{}, collections: map[string]*imageCollection{}, imageQueue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[*imageCollection]()), opts: opts, rollouts: map[string]rolloutState{}, revisions: map[string]map[string]controllerRevision{}, workloads: map[string]runtime.Object{}, excluded: map[string]bool{}, now: opts.now, queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[*eventBatch]())}
}

func (c *controller) run(ctx context.Context) error {
	defer c.queue.ShutDown()
	defer c.imageQueue.ShutDown()
	if c.opts.clusterUID == "" {
		ns, err := c.opts.client.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("discover cluster UID: %w", err)
		}
		c.opts.clusterUID = string(ns.UID)
	}
	factory := informers.NewSharedInformerFactoryWithOptions(c.opts.client, 0, informers.WithTransform(imageWatchTransform))
	pods := factory.InformerFor(&corev1.Pod{}, func(client kubernetes.Interface, _ time.Duration) cache.SharedIndexInformer {
		api := client.CoreV1().Pods(metav1.NamespaceAll)
		return newImageInformer(client, &corev1.Pod{}, func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return api.List(ctx, options)
		}, api.Watch)
	})
	replicaSets := factory.InformerFor(&appsv1.ReplicaSet{}, func(client kubernetes.Interface, _ time.Duration) cache.SharedIndexInformer {
		api := client.AppsV1().ReplicaSets(metav1.NamespaceAll)
		return newImageInformer(client, &appsv1.ReplicaSet{}, func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return api.List(ctx, options)
		}, api.Watch)
	})
	if err := c.watchImages(pods, replicaSets); err != nil {
		return err
	}
	configs, err := c.configInformers()
	if err != nil {
		return err
	}
	deployments := factory.Apps().V1().Deployments().Informer()
	if err := c.watchWorkloads(deployments); err != nil {
		return err
	}
	synced := []cache.InformerSynced{deployments.HasSynced, pods.HasSynced, replicaSets.HasSynced}
	informersCtx, stopInformers := context.WithCancel(ctx)
	var informersDone sync.WaitGroup
	defer func() { stopInformers(); informersDone.Wait() }()
	runInformer := func(informer cache.SharedIndexInformer) {
		informersDone.Add(1)
		go func() { defer informersDone.Done(); informer.Run(informersCtx.Done()) }()
		synced = append(synced, informer.HasSynced)
	}
	if c.opts.statefulSets || c.opts.daemonSets {
		revisions, err := c.controllerRevisionInformer()
		if err != nil {
			return err
		}
		// Index all revisions before the initial workload listing establishes
		// baselines, so existing rollouts aren't mistaken for new ones.
		runInformer(revisions)
		if !cache.WaitForCacheSync(ctx.Done(), revisions.HasSynced) {
			return fmt.Errorf("sync ControllerRevision watch")
		}
	}
	if c.opts.statefulSets {
		if err := c.watchWorkloads(factory.Apps().V1().StatefulSets().Informer()); err != nil {
			return err
		}
	}
	if c.opts.daemonSets {
		if err := c.watchWorkloads(factory.Apps().V1().DaemonSets().Informer()); err != nil {
			return err
		}
	}
	factory.Start(ctx.Done())
	defer factory.Shutdown()
	for _, informer := range configs {
		runInformer(informer)
	}
	if !cache.WaitForCacheSync(ctx.Done(), synced...) {
		return fmt.Errorf("sync Kubernetes watches")
	}
	go func() { <-ctx.Done(); c.queue.ShutDown(); c.imageQueue.ShutDown() }()
	var workers sync.WaitGroup
	workers.Add(1)
	go func() { defer workers.Done(); c.resolveImages(ctx) }()
	defer workers.Wait()
	c.opts.logger.Info("Kubernetes change watcher ready")
	for {
		event, shutdown := c.queue.Get()
		if shutdown {
			return nil
		}
		if c.deliver(ctx, event) {
			c.queue.Forget(event)
		} else if ctx.Err() == nil {
			c.queue.AddRateLimited(event)
		}
		c.queue.Done(event)
	}
}

// observation is how an informer delivered a workload.
type observation int

const (
	// observedInitial is an object of the initial listing, a baseline.
	observedInitial observation = iota
	// observedAdded is an object added after the initial listing.
	observedAdded
	observedUpdated
)

func (c *controller) watchWorkloads(informer cache.SharedIndexInformer) error {
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(obj any, initial bool) {
			if initial {
				c.observe(obj.(runtime.Object), observedInitial)
			} else {
				c.observe(obj.(runtime.Object), observedAdded)
			}
		},
		UpdateFunc: func(_, obj any) { c.observe(obj.(runtime.Object), observedUpdated) },
		DeleteFunc: c.remove,
	})
	return err
}

// observe is the single entry point for workload changes. It keeps the latest
// object for re-evaluation when ControllerRevisions change, and reports a
// workload added after the initial listing as created. Informer handlers
// serialize state updates.
func (c *controller) observe(obj runtime.Object, how observation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.workload(obj)
	if w == nil {
		return
	}
	tracked, baseline := c.scope(w, how == observedInitial)
	if !tracked {
		return
	}
	c.workloads[string(w.GetUID())] = obj
	if how == observedAdded && !baseline {
		c.enqueueCreated(w)
	}
	c.observeRollout(w, baseline)
}

// scope reports whether w is tracked, and whether to observe it as a baseline.
// A workload that exclude rules match is forgotten without events; one that
// leaves their scope is baselined like the initial listing. Callers hold c.mu.
func (c *controller) scope(w workload, initial bool) (tracked, baseline bool) {
	uid := string(w.GetUID())
	if c.opts.exclude.excludes(w.kind().resourceType, w) {
		c.excluded[uid] = true
		// ControllerRevisions stay indexed, so a returning workload baselines
		// on its current revision.
		delete(c.rollouts, uid)
		delete(c.collections, uid)
		delete(c.workloads, uid)
		return false, false
	}
	if c.excluded[uid] {
		// Annotation changes bump a Deployment's generation, so wait until the
		// controller has observed the change that returned w to scope.
		// Otherwise its settled rollout would be baselined as in flight.
		if !w.settled() {
			return false, false
		}
		delete(c.excluded, uid)
		return true, true
	}
	return true, initial
}

// observeRollout advances the rollout state machine of any workload kind.
// Callers hold c.mu.
func (c *controller) observeRollout(w workload, initial bool) {
	uid := string(w.GetUID())
	state, exists := c.rollouts[uid]
	revision, template := w.revision(), w.template()
	if initial {
		status := w.rolloutStatus()
		c.rollouts[uid] = rolloutState{revision: revision, template: *template.DeepCopy(), metadata: snapshotMetadata(w), status: status, stalled: status == rolloutStalled}
		return
	}
	if !w.settled() || revision <= 0 {
		return
	}
	// Ignore stale revision observations, including relists after watch reconnects.
	if exists && revision < state.revision {
		return
	}
	if !exists || revision != state.revision {
		if exists && state.revision > 0 && state.status != rolloutSucceeded {
			c.enqueueSuperseded(w, state, revision)
			c.closeImages(w, false)
		}
		next := rolloutState{revision: revision, previousRevision: state.revision, startedAt: c.now(), template: *template.DeepCopy(), metadata: snapshotMetadata(w), status: rolloutStarted}
		var replaced *rolloutState
		if exists {
			replaced = &state
		}
		c.enqueueStarted(w, next, replaced)
		c.openImages(w, next)
		state = next
	}
	status := w.rolloutStatus()
	// Success closes a rollout permanently; later availability/scaling changes
	// aren't new rollout outcomes. A stalled rollout can recover or be superseded.
	if state.status != rolloutSucceeded {
		switch status {
		case rolloutSucceeded:
			c.enqueueOutcome(w, state, rolloutSucceeded)
			c.closeImages(w, true)
			state.status = status
		case rolloutStalled:
			if !state.stalled {
				c.enqueueOutcome(w, state, rolloutStalled)
				state.stalled = true
			}
			state.status = status
		}
	}
	c.rollouts[uid] = state
}

// Retry the original event, preserving its ID and timestamp, and give consumers
// a copy so their mutations cannot affect a later retry.
func (c *controller) deliver(ctx context.Context, event *eventBatch) bool {
	err := c.opts.emit(ctx, func() eventBatch {
		copy := plog.NewLogs()
		event.logs.CopyTo(copy)
		return eventBatch{logs: copy, id: event.id}
	})
	if err != nil {
		c.opts.logger.Error("Unable to deliver SDLC change event; retrying", "event_id", event.id, "err", err)
		return false
	}
	return true
}

// remove clears local tracking without emitting an SDLC event.
// UID distinguishes deletion/recreation under the same namespace and name.
func (c *controller) remove(obj any) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	m, err := meta.Accessor(obj)
	if err != nil || m.GetUID() == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.rollouts, string(m.GetUID()))
	delete(c.collections, string(m.GetUID()))
	delete(c.workloads, string(m.GetUID()))
	delete(c.revisions, string(m.GetUID()))
	delete(c.excluded, string(m.GetUID()))
}

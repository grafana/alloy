package k8s_workloads

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const revisionAnnotation = "deployment.kubernetes.io/revision"

type controllerOptions struct {
	logger                  *slog.Logger
	client                  kubernetes.Interface
	clusterName, clusterUID string
	maxEventBytes           int
	emit                    func(context.Context, func() eventBatch) error
	now                     func() time.Time
}

type rolloutState struct {
	revision int64
	template corev1.PodTemplateSpec
	status   string
	stalled  bool
}

type controller struct {
	opts     controllerOptions
	rollouts map[string]rolloutState
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
	if opts.maxEventBytes == 0 {
		opts.maxEventBytes = 512 * 1024
	}
	return &controller{opts: opts, rollouts: map[string]rolloutState{}, now: opts.now, queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[*eventBatch]())}
}

func (c *controller) run(ctx context.Context) error {
	defer c.queue.ShutDown()
	if c.opts.clusterUID == "" {
		ns, err := c.opts.client.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("discover cluster UID: %w", err)
		}
		c.opts.clusterUID = string(ns.UID)
	}
	factory := informers.NewSharedInformerFactory(c.opts.client, 0)
	deployments := factory.Apps().V1().Deployments().Informer()
	_, err := deployments.AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
		AddFunc:    func(obj any, initial bool) { c.observe(obj.(*appsv1.Deployment), initial) },
		UpdateFunc: func(_, obj any) { c.observe(obj.(*appsv1.Deployment), false) },
		DeleteFunc: func(obj any) {
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			if d, ok := obj.(*appsv1.Deployment); ok {
				delete(c.rollouts, string(d.UID))
			}
		},
	})
	if err != nil {
		return err
	}
	factory.Start(ctx.Done())
	defer factory.Shutdown()
	if !cache.WaitForCacheSync(ctx.Done(), deployments.HasSynced) {
		return fmt.Errorf("sync Deployment watch")
	}
	go func() { <-ctx.Done(); c.queue.ShutDown() }()
	c.opts.logger.Info("Kubernetes Deployment rollout watcher ready")
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

// Informer handlers serialize state updates. Revisions are assigned by the
// Deployment controller, unlike generation which also changes during scaling.
func (c *controller) observe(d *appsv1.Deployment, initial bool) {
	uid := string(d.UID)
	previous, exists := c.rollouts[uid]
	revision, _ := strconv.ParseInt(d.Annotations[revisionAnnotation], 10, 64)
	if initial {
		c.rollouts[uid] = rolloutState{revision: revision, template: *d.Spec.Template.DeepCopy(), status: rolloutStatus(d), stalled: rolloutStatus(d) == rolloutStalled}
		return
	}
	if d.DeletionTimestamp != nil || d.Spec.Paused || d.Status.ObservedGeneration < d.Generation || revision <= 0 {
		return
	}
	// Ignore stale revision observations, including relists after watch reconnects.
	if exists && revision < previous.revision {
		return
	}
	if !exists || revision != previous.revision {
		if exists && previous.revision > 0 && previous.status != rolloutSucceeded {
			c.enqueue(d, previous, rolloutSuperseded, revision, nil, nil, false)
		}
		kinds, changes := []string{}, []templateChange{}
		if exists {
			kinds, changes = templateChanges(previous.template, d.Spec.Template)
		}
		previous = rolloutState{revision: revision, template: *d.Spec.Template.DeepCopy(), status: rolloutStarted}
		c.enqueue(d, previous, rolloutStarted, 0, kinds, changes, exists)
	}
	status := rolloutStatus(d)
	// Success closes a rollout permanently; later availability/scaling changes
	// aren't new rollout outcomes. A stalled rollout can recover or be superseded.
	if previous.status != rolloutSucceeded {
		switch status {
		case rolloutSucceeded:
			c.enqueue(d, previous, rolloutSucceeded, 0, nil, nil, false)
			previous.status = status
		case rolloutStalled:
			if !previous.stalled {
				c.enqueue(d, previous, rolloutStalled, 0, nil, nil, false)
				previous.stalled = true
			}
			previous.status = status
		}
	}
	c.rollouts[uid] = previous
}

func rolloutStatus(d *appsv1.Deployment) string {
	if d.Spec.Paused || d.DeletionTimestamp != nil || d.Status.ObservedGeneration < d.Generation {
		return rolloutStarted
	}
	for _, condition := range d.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded" {
			return rolloutStalled
		}
	}
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	// Require the controller's completion condition as well as replica counts.
	for _, condition := range d.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionTrue && condition.Reason == "NewReplicaSetAvailable" && d.Status.UpdatedReplicas == desired && d.Status.Replicas == desired && d.Status.AvailableReplicas == desired && d.Status.UnavailableReplicas == 0 {
			return rolloutSucceeded
		}
	}
	return rolloutStarted
}

// Retry the original event, preserving its ID and timestamp, and give consumers
// a copy so their mutations cannot affect a later retry.
func (c *controller) deliver(ctx context.Context, event *eventBatch) bool {
	jsonData, err := (&plog.JSONMarshaler{}).MarshalLogs(event.logs)
	if err == nil {
		var protoData []byte
		protoData, err = (&plog.ProtoMarshaler{}).MarshalLogs(event.logs)
		if len(jsonData) >= c.opts.maxEventBytes || len(protoData) >= c.opts.maxEventBytes {
			c.opts.logger.Error("Dropping oversized rollout event", "event_id", event.id, "max_event_bytes", c.opts.maxEventBytes)
			return true
		}
	}
	if err == nil {
		err = c.opts.emit(ctx, func() eventBatch {
			copy := plog.NewLogs()
			event.logs.CopyTo(copy)
			return eventBatch{logs: copy, id: event.id}
		})
	}
	if err != nil {
		c.opts.logger.Error("Unable to deliver rollout event; retrying", "event_id", event.id, "err", err)
		return false
	}
	return true
}

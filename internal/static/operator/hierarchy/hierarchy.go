// Package hierarchy provides tools to discover a resource hierarchy. A
// resource hierarchy is made when a resource has a set of rules to discover
// other resources.
package hierarchy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Notifier can be attached to a controller and generate reconciles when
// objects inside of a resource hierarchy change.
type Notifier struct {
	log    log.Logger
	client client.Client

	watchersMut sync.RWMutex
	watchers    map[schema.GroupVersionKind][]Watcher
}

// Watcher is something watching for changes to a resource.
type Watcher struct {
	Object   client.Object    // Object to watch for events against.
	Owner    client.ObjectKey // Owner to receive a reconcile for.
	Selector Selector         // Selector to use to match changed objects.
}

// NewNotifier creates a new Notifier which uses the provided client for
// performing hierarchy lookups.
func NewNotifier(l log.Logger, cli client.Client) *Notifier {
	return &Notifier{
		log:      l,
		client:   cli,
		watchers: make(map[schema.GroupVersionKind][]Watcher),
	}
}

// EventHandler returns an event handler that can be given to
// controller.Watches.
//
// controller.Watches should be called once per type in the resource hierarchy.
// Each call to controller.Watches should use the same Notifier.
func (n *Notifier) EventHandler() handler.EventHandler {
	// TODO(rfratto): It's possible to create a custom implementation of
	// source.Source so we wouldn't have to call controller.Watches a bunch of
	// times. I played around a little with an implementation but it was going to
	// be a lot of work to dynamically spin up/down informers, so I put it aside
	// for now. Maybe it's an improvement for the future.
	return &notifierEventHandler{Notifier: n}
}

// Notify configures reconciles to be generated for a set of watchers when
// watched resources change.
//
// Notify appends to the list of watchers. To remove out notifications for a
// specific owner, call StopNotify.
func (n *Notifier) Notify(watchers ...Watcher) error {
	n.watchersMut.Lock()
	defer n.watchersMut.Unlock()

	for _, w := range watchers {
		gvk, err := apiutil.GVKForObject(w.Object, n.client.Scheme())
		if err != nil {
			return fmt.Errorf("could not get GVK: %w", err)
		}

		n.watchers[gvk] = append(n.watchers[gvk], w)
	}

	return nil
}

// StopNotify removes all watches for a specific owner.
func (n *Notifier) StopNotify(owner client.ObjectKey) {
	n.watchersMut.Lock()
	defer n.watchersMut.Unlock()

	for key, watchers := range n.watchers {
		rem := make([]Watcher, 0, len(watchers))
		for _, w := range watchers {
			if w.Owner != owner {
				rem = append(rem, w)
			}
		}
		n.watchers[key] = rem
	}
}

type notifierEventHandler struct {
	*Notifier
}

var _ handler.EventHandler = (*notifierEventHandler)(nil)

func (h *notifierEventHandler) Create(ctx context.Context, ev event.CreateEvent, q workqueue.RateLimitingInterface) {
	h.handleEvent(ev.Object, q)
}

func (h *notifierEventHandler) Update(ctx context.Context, ev event.UpdateEvent, q workqueue.RateLimitingInterface) {
	h.handleEvent(ev.ObjectOld, q)
	h.handleEvent(ev.ObjectNew, q)
}

func (h *notifierEventHandler) Delete(ctx context.Context, ev event.DeleteEvent, q workqueue.RateLimitingInterface) {
	h.handleEvent(ev.Object, q)
}

func (h *notifierEventHandler) Generic(ctx context.Context, ev event.GenericEvent, q workqueue.RateLimitingInterface) {
	h.handleEvent(ev.Object, q)
}

func (h *notifierEventHandler) handleEvent(obj client.Object, q workqueue.RateLimitingInterface) {
	h.watchersMut.RLock()
	defer h.watchersMut.RUnlock()

	gvk, err := apiutil.GVKForObject(obj, h.client.Scheme())
	if err != nil {
		level.Error(h.log).Log("msg", "failed to get gvk for object", "err", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Iterate through all of the watchers for the gvk and check to see if we
	// should trigger a reconcile.
	for _, watcher := range h.watchers[gvk] {
		matches, err := watcher.Selector.Matches(ctx, h.client, obj)
		if err != nil {
			level.Error(h.log).Log("msg", "failed to handle notifier event", "err", err)
			return
		}
		if matches {
			q.Add(reconcile.Request{NamespacedName: watcher.Owner})
		}
	}
}

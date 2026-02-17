package podlogs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	monitoringv1alpha2 "github.com/grafana/alloy/internal/component/loki/source/podlogs/internal/apis/monitoring/v1alpha2"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type controller struct {
	log        log.Logger
	reconciler *reconciler

	mut       sync.RWMutex
	informers cache.Informers
	client    client.Client
	reloadCh  chan struct{} // Written to when informers or client changes

	reconcileCh chan struct{}
	doneCh      chan struct{}
}

// Generous timeout period for configuring all informers
const informerSyncTimeout = 10 * time.Minute

// newController creates a new, unstarted controller. The controller will
// request a reconcile when the state of Kubernetes changes.
func newController(l log.Logger, reconciler *reconciler) *controller {
	return &controller{
		log:        l,
		reconciler: reconciler,
		reloadCh:   make(chan struct{}, 1),

		reconcileCh: make(chan struct{}, 1),
		doneCh:      make(chan struct{}),
	}
}

func (ctrl *controller) UpdateConfig(cfg *rest.Config) error {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		monitoringv1alpha2.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			return fmt.Errorf("unable to register scheme: %w", err)
		}
	}

	cache, err := cache.New(cfg, cache.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	cli, err := client.New(cfg, client.Options{
		Scheme: scheme,
		Cache: &client.CacheOptions{
			Reader: cache,
		},
	})
	if err != nil {
		return err
	}

	// Update the stored informers and client and schedule a reload.
	ctrl.mut.Lock()
	ctrl.informers = cache
	ctrl.client = cli
	ctrl.mut.Unlock()

	select {
	case ctrl.reloadCh <- struct{}{}:
	default:
		// Reload is already scheduled
	}
	return nil
}

// Run the controller.
func (ctrl *controller) Run(ctx context.Context) error {
	var (
		cancel    context.CancelFunc
		informers cache.Informers
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ctrl.reloadCh:
			ctrl.mut.RLock()
			var (
				newInformers = ctrl.informers
				newClient    = ctrl.client
			)
			ctrl.mut.RUnlock()

			// Stop old informers.
			if informers != nil {
				cancel()
			}

			informerContext, informerCancel := context.WithCancel(ctx)

			go func() {
				if err := ctrl.run(informerContext, newInformers, newClient); err != nil {
					level.Error(ctrl.log).Log("msg", "failed to run controller", "err", err)
				}
			}()

			cancel = informerCancel
			informers = newInformers
		}
	}
}

func (ctrl *controller) run(ctx context.Context, informers cache.Informers, client client.Client) error {
	level.Info(ctrl.log).Log("msg", "starting controller")
	defer level.Info(ctrl.log).Log("msg", "controller exiting")

	go func() {
		err := informers.Start(ctx)
		if err != nil && ctx.Err() != nil {
			level.Error(ctrl.log).Log("msg", "failed to start informers", "err", err)
		}
	}()

	if !informers.WaitForCacheSync(ctx) {
		return fmt.Errorf("informer caches failed to sync")
	}

	if err := ctrl.configureInformers(ctx, informers); err != nil {
		return fmt.Errorf("failed to configure informers: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ctrl.reconcileCh:
			if err := ctrl.reconciler.Reconcile(ctx, client); err != nil {
				level.Error(ctrl.log).Log("msg", "reconcile failed", "err", err)
			}
		}
	}
}

// configureInformers starts the informers used by this controller to perform reconciles.
func (ctrl *controller) configureInformers(ctx context.Context, informers cache.Informers) error {
	// We want to re-reconcile the set of PodLogs whenever namespaces, pods, or
	// PodLogs changes. Reconciling on namespaces and pods is important so that
	// we can reevaluate selectors defined in PodLogs.
	types := []client.Object{
		&corev1.Namespace{},
		&corev1.Pod{},
		&monitoringv1alpha2.PodLogs{},
	}

	informerCtx, cancel := context.WithTimeout(ctx, informerSyncTimeout)
	defer cancel()

	// Register field index for spec.nodeName only if node filtering is enabled
	if ctrl.reconciler.nodeFilterEnabled && ctrl.reconciler.getNodeFilterName() != "" {
		if err := informers.IndexField(informerCtx, &corev1.Pod{}, "spec.nodeName", func(obj client.Object) []string {
			pod := obj.(*corev1.Pod)
			return []string{pod.Spec.NodeName}
		}); err != nil {
			return fmt.Errorf("failed to register field index for spec.nodeName: %w", err)
		}
		level.Debug(ctrl.log).Log("msg", "registered field index for spec.nodeName")
	}

	for _, ty := range types {
		informer, err := informers.GetInformer(informerCtx, ty)
		if err != nil {
			if errors.Is(informerCtx.Err(), context.DeadlineExceeded) { // Check the context to prevent GetInformer returning a fake timeout
				return fmt.Errorf("timeout exceeded while configuring informers. Check the connection"+
					" to the Kubernetes API is stable and that Alloy has appropriate RBAC permissions for %v", ty)
			}

			return err
		}
		_, err = informer.AddEventHandler(onChangeEventHandler{ChangeFunc: ctrl.RequestReconcile})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctrl *controller) RequestReconcile() {
	select {
	case ctrl.reconcileCh <- struct{}{}:
	default:
		// Reconcile is already queued; do nothing.
	}
}

// onChangeEventHandler implements [toolscache.ResourceEventHandler], calling
// ChangeFunc when any change occurs. Objects are not sent to the handler.
type onChangeEventHandler struct {
	ChangeFunc func()
}

var _ toolscache.ResourceEventHandler = onChangeEventHandler{}

func (h onChangeEventHandler) OnAdd(_ any, _ bool) { h.ChangeFunc() }
func (h onChangeEventHandler) OnUpdate(_, _ any)   { h.ChangeFunc() }
func (h onChangeEventHandler) OnDelete(_ any)      { h.ChangeFunc() }

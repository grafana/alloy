package k8s_workloads

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type controllerOptions struct {
	logger                  *slog.Logger
	client                  kubernetes.Interface
	clusterName, clusterUID string
	snapshots               SnapshotArguments
	emit                    func(context.Context, func() eventBatch) error
	now                     func() time.Time
	metrics                 *reportingMetrics
}
type reportingMetrics struct {
	failures    *prometheus.CounterVec
	lastSuccess *prometheus.GaugeVec
}

func newReportingMetrics(reg prometheus.Registerer) *reportingMetrics {
	m := &reportingMetrics{
		failures:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "k8s_workloads_reporting_errors_total", Help: "Failed workload event reporting attempts."}, []string{"operation", "reason"}),
		lastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "k8s_workloads_snapshot_last_success_timestamp_seconds", Help: "Last snapshot accepted by the downstream pipeline, by scope."}, []string{"kind", "namespace"}),
	}
	if reg != nil {
		reg.MustRegister(m.failures, m.lastSuccess)
	}
	return m
}

type controller struct {
	opts      controllerOptions
	queue     workqueue.TypedRateLimitingInterface[*eventBatch]
	now       func() time.Time
	lastError map[string]time.Time
}

func newController(opts controllerOptions) *controller {
	if opts.now == nil {
		opts.now = time.Now
	}
	if opts.logger == nil {
		opts.logger = slog.Default()
	}
	if opts.snapshots.Interval == 0 {
		opts.snapshots.SetToDefault()
	}
	if opts.metrics == nil {
		opts.metrics = newReportingMetrics(nil)
	}
	return &controller{opts: opts, now: opts.now, lastError: map[string]time.Time{}, queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[*eventBatch]())}
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
	namespaces := factory.Core().V1().Namespaces().Informer()
	deployments := factory.Apps().V1().Deployments().Informer()
	for _, binding := range []struct {
		informer cache.SharedIndexInformer
		kind     string
	}{{namespaces, "namespace"}, {deployments, "deployment"}} {
		kind := binding.kind
		_, err := binding.informer.AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
			AddFunc: func(obj any, initial bool) {
				if !initial {
					c.notify(kind, "created", obj)
				}
			},
			DeleteFunc: func(obj any) { c.notify(kind, "deleted", obj) },
		})
		if err != nil {
			return err
		}
	}
	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), namespaces.HasSynced, deployments.HasSynced) {
		return fmt.Errorf("sync namespace and Deployment watches")
	}
	// A single worker serializes scan collection. Ticks coalesce while a scan is
	// running; they never create overlapping or queued catch-up collections.
	scan := &eventBatch{scan: true}
	c.queue.Add(scan)
	go func() {
		ticker := time.NewTicker(c.opts.snapshots.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				c.queue.ShutDown()
				return
			case <-ticker.C:
				c.queue.Add(scan)
			}
		}
	}()
	for {
		item, shutdown := c.queue.Get()
		if shutdown {
			return nil
		}
		if item.scan {
			c.collect(ctx)
			c.queue.Forget(item)
		} else if err := c.deliver(ctx, item); err != nil {
			if !item.reported {
				c.report(ctx, item, "delivery_failed", "Downstream rejected an event; retrying the original event", 0, 0)
				item.reported = true
			}
			c.queue.AddRateLimited(item)
		} else {
			c.queue.Forget(item)
		}
		c.queue.Done(item)
	}
}

func (c *controller) notify(kind, operation string, obj any) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	var name, namespace, namespaceUID, uid string
	switch value := obj.(type) {
	case *corev1.Namespace:
		name = value.Name
		namespace = value.Name
		uid = string(value.UID)
		namespaceUID = uid
	case *appsv1.Deployment:
		name = value.Name
		namespace = value.Namespace
		uid = string(value.UID)
	default:
		return
	}
	now := c.now()
	event := makeEvent(c.opts.clusterUID, c.opts.clusterName, namespace, namespaceUID, uid, operation, kind, now, map[string]any{"name": name, "uid": uid, "collected_at": now.UTC().Format(time.RFC3339Nano)})
	c.queue.Add(event)
}

func (c *controller) collect(ctx context.Context) {
	// Prototype: timestamps order scans from one active collector. The scan
	// interval is expected to comfortably exceed typical clock skew across nodes.
	// Rollback after rescheduling can temporarily cause fresh scans to be rejected.
	// A durable monotonic epoch/sequence would give stronger failover guarantees.
	collected := c.now()
	attempt := makeEvent(c.opts.clusterUID, c.opts.clusterName, "", "", "", "snapshot", "namespace", collected, nil)
	namespaces, err := c.opts.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.report(ctx, attempt, "collection_failed", "Could not list namespaces", 0, 0)
		return
	}
	entries := make([]namespaceSnapshot, 0, len(namespaces.Items))
	sort.Slice(namespaces.Items, func(i, j int) bool { return namespaces.Items[i].Name < namespaces.Items[j].Name })
	for i := range namespaces.Items {
		entries = append(entries, describeNamespace(&namespaces.Items[i]))
	}
	c.publish(ctx, makeEvent(c.opts.clusterUID, c.opts.clusterName, "", "", "", "snapshot", "namespace", collected, map[string]any{
		"collected_at": collected.UTC().Format(time.RFC3339Nano), "complete": true, "resource_version": namespaces.ResourceVersion, "namespaces": entries,
	}))
	for i := range namespaces.Items {
		if ctx.Err() != nil {
			return
		}
		c.collectDeployments(ctx, &namespaces.Items[i])
	}
}
func (c *controller) collectDeployments(ctx context.Context, ns *corev1.Namespace) {
	collected := c.now()
	attempt := makeEvent(c.opts.clusterUID, c.opts.clusterName, ns.Name, string(ns.UID), "", "snapshot", "deployment", collected, nil)
	// Use API lists instead of stale watch caches for authoritative membership.
	// Enrichment lists are separate reads, not an atomic cross-resource snapshot.
	deployments, err := c.opts.client.AppsV1().Deployments(ns.Name).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.report(ctx, attempt, "collection_failed", "Could not list Deployments", 0, 0)
		return
	}
	sets, err := c.opts.client.AppsV1().ReplicaSets(ns.Name).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.report(ctx, attempt, "collection_failed", "Could not list ReplicaSets", 0, 0)
		return
	}
	pods, err := c.opts.client.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
	if err != nil {
		c.report(ctx, attempt, "collection_failed", "Could not list Pods", 0, 0)
		return
	}
	current, err := c.opts.client.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err != nil || current.UID != ns.UID {
		c.report(ctx, attempt, "collection_failed", "Namespace disappeared or was recreated during collection", 0, 0)
		return
	}
	entries := make([]deploymentSnapshot, 0, len(deployments.Items))
	sort.Slice(deployments.Items, func(i, j int) bool { return deployments.Items[i].Name < deployments.Items[j].Name })
	for i := range deployments.Items {
		entries = append(entries, describeDeployment(&deployments.Items[i], sets.Items, pods.Items))
	}
	c.publish(ctx, makeEvent(c.opts.clusterUID, c.opts.clusterName, ns.Name, string(ns.UID), "", "snapshot", "deployment", collected, map[string]any{
		"collected_at": collected.UTC().Format(time.RFC3339Nano), "complete": true, "resource_version": deployments.ResourceVersion, "deployments": entries,
	}))
}
func (c *controller) publish(ctx context.Context, event *eventBatch) {
	if err := c.deliver(ctx, event); err != nil {
		c.report(ctx, event, "delivery_failed", "Downstream rejected an event; retrying the original event", 0, 0)
		event.reported = true
		c.queue.AddRateLimited(event)
	}
}
func (c *controller) deliver(ctx context.Context, event *eventBatch) error {
	// Check both supported OTLP encodings, including their one-record envelopes.
	// The ingester independently checks its final Kafka JSON serialization.
	jsonBytes, err := (&plog.JSONMarshaler{}).MarshalLogs(event.logs)
	if err != nil {
		return err
	}
	protoBytes, err := (&plog.ProtoMarshaler{}).MarshalLogs(event.logs)
	if err != nil {
		return err
	}
	measured := max(len(jsonBytes), len(protoBytes))
	if measured >= c.opts.snapshots.MaxSizeBytes {
		c.report(ctx, event, "payload_too_large", "Serialized event reached the configured byte limit", measured, c.opts.snapshots.MaxSizeBytes)
		return nil // Never retry an unchanged oversized event; the next scan retries collection.
	}
	if err := c.send(ctx, event); err != nil {
		return err
	}
	if event.operation == "snapshot" {
		c.opts.metrics.lastSuccess.WithLabelValues(event.kind, event.namespace).Set(float64(c.now().Unix()))
	}
	return nil
}
func (c *controller) send(ctx context.Context, event *eventBatch) error {
	// Some downstream components mutate pdata; retries must retain the payload.
	return c.opts.emit(ctx, func() eventBatch { logs := plog.NewLogs(); event.logs.CopyTo(logs); return eventBatch{logs: logs} })
}
func (c *controller) report(ctx context.Context, failed *eventBatch, reason, message string, measured, limit int) {
	c.opts.metrics.failures.WithLabelValues(failed.operation, reason).Inc()
	key := failed.namespace + "/" + failed.operation + "/" + reason
	now := c.now()
	if last, ok := c.lastError[key]; !ok || now.Sub(last) >= time.Minute {
		c.opts.logger.Error(message, "reason", reason, "namespace", failed.namespace, "attempt_id", failed.id, "measured_bytes", measured, "limit_bytes", limit)
		c.lastError[key] = now
	}
	body := map[string]any{"operation": failed.operation, "entity_kind": failed.kind, "attempt_id": failed.id, "reason": reason, "message": message, "failed_collected_at": failed.collected.UTC().Format(time.RFC3339Nano)}
	if measured > 0 {
		body["measured_bytes"] = measured
		body["limit_bytes"] = limit
	}
	event := makeEvent(c.opts.clusterUID, c.opts.clusterName, failed.namespace, failed.namespaceUID, failed.entityUID, "error", failed.kind, now, body)
	record := event.logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	record.SetEventName(errorEventName)
	record.SetSeverityNumber(plog.SeverityNumberError)
	// Errors contain bounded diagnostic fields, never the original payload. Do not
	// recursively report a reporting failure. They use a separate small budget.
	payload, err := (&plog.JSONMarshaler{}).MarshalLogs(event.logs)
	if err != nil || len(payload) > 8192 {
		c.opts.logger.Error("could not encode bounded reporting error")
		return
	}
	if err := c.send(ctx, event); err != nil {
		c.opts.logger.Error("could not deliver reporting error", "err", err)
	}
}

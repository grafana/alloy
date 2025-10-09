package kubernetes_events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	cachetools "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/loki/pkg/push"
)

const (
	logFormatJson = "json"
	logFormatFmt  = "logfmt"
)

type eventControllerTask struct {
	Log          log.Logger
	Config       *rest.Config // Config to connect to Kubernetes.
	Namespace    string       // Namespace to watch for events in.
	JobName      string       // Label value to use for job.
	InstanceName string       // Label value to use for instance.
	Receiver     loki.LogsReceiver
	Positions    positions.Positions
	LogFormat    string
}

// Hash implements [runner.Task].
func (t eventControllerTask) Hash() uint64 {
	return xxhash.Sum64String(t.Namespace)
}

// Equals implements [runner.Task].
func (t eventControllerTask) Equals(other runner.Task) bool {
	// We can do a direct comparison since the two task types are comparable.
	return t == other.(eventControllerTask)
}

type eventController struct {
	log     log.Logger
	task    eventControllerTask
	handler loki.EntryHandler

	positionsKey  string
	initTimestamp time.Time
}

func newEventController(task eventControllerTask) *eventController {
	var key string
	if task.Namespace == "" {
		key = positions.CursorKey("events")
	} else {
		key = positions.CursorKey("events-" + task.Namespace)
	}

	lastTimestamp, _ := task.Positions.Get(key, "")

	return &eventController{
		log:           task.Log,
		task:          task,
		handler:       loki.NewEntryHandler(task.Receiver.Chan(), func() {}),
		positionsKey:  key,
		initTimestamp: time.UnixMicro(lastTimestamp),
	}
}

func (ctrl *eventController) Run(ctx context.Context) {
	defer ctrl.handler.Stop()

	level.Info(ctrl.log).Log("msg", "watching events for namespace", "namespace", ctrl.task.Namespace)
	defer level.Info(ctrl.log).Log("msg", "stopping watcher for events", "namespace", ctrl.task.Namespace)

	if err := ctrl.runError(ctx); err != nil {
		level.Error(ctrl.log).Log("msg", "event watcher exited with error", "err", err)
	}
}

func (ctrl *eventController) runError(ctx context.Context) error {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("adding core to scheme: %w", err)
	}

	defaultNamespaces := map[string]cache.Config{}
	if ctrl.task.Namespace != "" {
		defaultNamespaces[ctrl.task.Namespace] = cache.Config{}
	}
	opts := cache.Options{
		Scheme:            scheme,
		DefaultNamespaces: defaultNamespaces,
	}
	informers, err := cache.New(ctrl.task.Config, opts)
	if err != nil {
		return fmt.Errorf("creating informers cache: %w", err)
	}

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

	<-ctx.Done()
	return nil
}

func (ctrl *eventController) configureInformers(ctx context.Context, informers cache.Informers) error {
	types := []client.Object{
		&corev1.Event{},
	}

	informerCtx, cancel := context.WithTimeout(ctx, informerSyncTimeout)
	defer cancel()

	for _, ty := range types {
		informer, err := informers.GetInformer(informerCtx, ty)
		if err != nil {
			if errors.Is(informerCtx.Err(), context.DeadlineExceeded) { // Check the context to prevent GetInformer returning a fake timeout
				return fmt.Errorf("timeout exceeded while configuring informers. Check the connection"+
					" to the Kubernetes API is stable and that Alloy has appropriate RBAC permissions for %v", ty)
			}
			return err
		}

		_, err = informer.AddEventHandler(cachetools.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.onAdd(ctx, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.onUpdate(ctx, oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { ctrl.onDelete(ctx, obj) },
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctrl *eventController) onAdd(ctx context.Context, obj interface{}) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		level.Warn(ctrl.log).Log("msg", "received an event for a non-Event Kind", "type", fmt.Sprintf("%T", obj))
		return
	}
	err := ctrl.handleEvent(ctx, event)
	if err != nil {
		level.Error(ctrl.log).Log("msg", "error handling event", "err", err)
	}
}

func (ctrl *eventController) onUpdate(ctx context.Context, oldObj, newObj interface{}) {
	oldEvent, ok := oldObj.(*corev1.Event)
	if !ok {
		level.Warn(ctrl.log).Log("msg", "received an event for a non-Event Kind", "type", fmt.Sprintf("%T", oldObj))
		return
	}
	newEvent, ok := newObj.(*corev1.Event)
	if !ok {
		level.Warn(ctrl.log).Log("msg", "received an event for a non-Event Kind", "type", fmt.Sprintf("%T", newObj))
		return
	}

	if oldEvent.GetResourceVersion() == newEvent.GetResourceVersion() {
		level.Debug(ctrl.log).Log("msg", "resource version didn't change, ignoring call to onUpdate", "resource version", newEvent.GetResourceVersion())
		return
	}

	err := ctrl.handleEvent(ctx, newEvent)
	if err != nil {
		level.Error(ctrl.log).Log("msg", "error handling event", "err", err)
	}
}

func (ctrl *eventController) onDelete(ctx context.Context, obj interface{}) {
	// no-op: the event got deleted from Kubernetes, but there's nothing to log
	// when this happens.
}

func (ctrl *eventController) handleEvent(ctx context.Context, event *corev1.Event) error {
	eventTs := eventTimestamp(event)

	// Events don't have any ordering guarantees, so we can't rely on comparing
	// the timestamp of this event to any other event received.
	//
	// We use a best-effort attempt to not re-deliver any events we've already
	// logged by checking the timestamp from when the worker started. This may
	// still cause us to drop some events in between recreating workers, but it
	// minimizes risk.
	//
	// TODO(rfratto): a longer term solution would be to track timestamps for
	// each involved object (or something similar), but that solution would need
	// to make sure to not leak those timestamps, and find a way to recognize
	// that involved objects have been deleted.
	if !eventTs.After(ctrl.initTimestamp) {
		return nil
	}

	lset, msg, err := ctrl.parseEvent(event)
	if err != nil {
		return err
	}

	entry := loki.Entry{
		Entry: push.Entry{
			Timestamp: eventTs,
			Line:      msg,
		},
		Labels: lset,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ctrl.handler.Chan() <- entry:
		// Update position offset only after it's been sent to the next set of
		// components.
		ctrl.task.Positions.Put(ctrl.positionsKey, "", eventTs.UnixMicro())
		return nil
	}
}

func (ctrl *eventController) parseEvent(event *corev1.Event) (model.LabelSet, string, error) {
	var (
		msg      strings.Builder
		lset     = make(model.LabelSet)
		fields   = make(map[string]any)
		appender = appendTextMsg
	)

	obj := event.InvolvedObject
	if obj.Name == "" {
		return nil, "", fmt.Errorf("no involved object for event")
	}

	lset[model.LabelName("namespace")] = model.LabelValue(obj.Namespace)
	lset[model.LabelName("job")] = model.LabelValue(ctrl.task.JobName)
	lset[model.LabelName("instance")] = model.LabelValue(ctrl.task.InstanceName)

	if ctrl.task.LogFormat == logFormatJson {
		appender = appendJsonMsg
	}

	appender(&msg, fields, "name", obj.Name, "%s")
	if obj.Kind != "" {
		appender(&msg, fields, "kind", obj.Kind, "%s")
	}
	if event.Action != "" {
		appender(&msg, fields, "action", event.Action, "%s")
	}
	if obj.APIVersion != "" {
		appender(&msg, fields, "objectAPIversion", obj.APIVersion, "%s")
	}
	if obj.ResourceVersion != "" {
		appender(&msg, fields, "objectRV", obj.ResourceVersion, "%s")
	}
	if event.ResourceVersion != "" {
		appender(&msg, fields, "eventRV", event.ResourceVersion, "%s")
	}
	if event.ReportingInstance != "" {
		appender(&msg, fields, "reportinginstance", event.ReportingInstance, "%s")
	}
	if event.ReportingController != "" {
		appender(&msg, fields, "reportingcontroller", event.ReportingController, "%s")
	}
	if event.Source.Component != "" {
		appender(&msg, fields, "sourcecomponent", event.Source.Component, "%s")
	}
	if event.Source.Host != "" {
		appender(&msg, fields, "sourcehost", event.Source.Host, "%s")
	}
	if event.Reason != "" {
		appender(&msg, fields, "reason", event.Reason, "%s")
	}
	if event.Type != "" {
		appender(&msg, fields, "type", event.Type, "%s")
	}
	if event.Count != 0 {
		appender(&msg, fields, "count", event.Count, "%d")
	}

	appender(&msg, fields, "msg", event.Message, "%q")

	if ctrl.task.LogFormat == logFormatJson {
		bb, err := json.Marshal(fields)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal Event to JSON: %w", err)
		}
		msg.WriteString(string(bb))
	}

	return lset, msg.String(), nil
}

// Appends the "fields" map with an entry for the provided event field
// Signatures of "appendJsonMsg" and "appendTextMsg" must match
func appendJsonMsg(msg *strings.Builder, fields map[string]any, key string, value any, format string) {
	fields[key] = value
}

// Appends the message builder with the provided event field
// Signatures of "appendJsonMsg" and "appendTextMsg" must match
func appendTextMsg(msg *strings.Builder, fields map[string]any, key string, value any, format string) {
	msg.WriteString(key)
	msg.WriteByte('=')
	fmt.Fprintf(msg, format, value)
	msg.WriteByte(' ')
}

func eventTimestamp(event *corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	return event.EventTime.Time
}

func (ctrl *eventController) DebugInfo() controllerInfo {
	ts, _ := ctrl.task.Positions.Get(ctrl.positionsKey, "")

	return controllerInfo{
		Namespace:     ctrl.task.Namespace,
		LastTimestamp: time.UnixMicro(ts),
	}
}

type controllerInfo struct {
	Namespace     string    `alloy:"namespace,attr"`
	LastTimestamp time.Time `alloy:"last_event_timestamp,attr"`
}

// Package eventhandler watches for Kubernetes Event objects and hands them off to
// Agent's Logs subsystem (embedded promtail)
package eventhandler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	cacheFileMode = 0600
	logFormatJson = "json"
	logFormatFmt  = "logfmt"
)

// EventHandler watches for Kubernetes Event objects and hands them off to
// Agent's logs subsystem (embedded promtail).
type EventHandler struct {
	LogsClient    *logs.Logs
	LogsInstance  string
	Log           log.Logger
	CachePath     string
	LastEvent     *ShippedEvents
	InitEvent     *ShippedEvents
	EventInformer cache.SharedIndexInformer
	SendTimeout   time.Duration
	ticker        *time.Ticker
	instance      string
	extraLabels   labels.Labels
	logFormat     string
	sync.Mutex
}

// ShippedEvents stores a timestamp and map of event ResourceVersions shipped for that timestamp.
// Used to avoid double-shipping events upon restart.
type ShippedEvents struct {
	// shipped event's timestamp
	Timestamp time.Time `json:"ts"`
	// map of event RVs (resource versions) already "shipped" (handed off) for this timestamp.
	// this is to handle the case of a timestamp having multiple events,
	// which happens quite frequently.
	RvMap map[string]struct{} `json:"resourceVersion"`
}

func newEventHandler(l log.Logger, globals integrations.Globals, c *Config) (integrations.Integration, error) {
	var (
		config  *rest.Config
		err     error
		factory informers.SharedInformerFactory
		id      string
	)

	// Try using KubeconfigPath or inClusterConfig
	config, err = clientcmd.BuildConfigFromFlags("", c.KubeconfigPath)
	if err != nil {
		level.Error(l).Log("msg", "Loading from KubeconfigPath or inClusterConfig failed", "err", err)
		// Trying default home location
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath := filepath.Join(home, ".kube", "config")
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				level.Error(l).Log("msg", "Could not load a kubeconfig", "err", err)
				return nil, err
			}
		} else {
			err = fmt.Errorf("could not load a kubeconfig")
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// get an informer
	if c.Namespace == "" {
		factory = informers.NewSharedInformerFactory(clientset, time.Duration(c.InformerResync)*time.Second)
	} else {
		factory = informers.NewSharedInformerFactoryWithOptions(clientset, time.Duration(c.InformerResync)*time.Second, informers.WithNamespace(c.Namespace))
	}

	eventInformer := factory.Core().V1().Events().Informer()
	id, _ = c.Identifier(globals)

	eh := &EventHandler{
		LogsClient:    globals.Logs,
		LogsInstance:  c.LogsInstance,
		Log:           l,
		CachePath:     c.CachePath,
		EventInformer: eventInformer,
		SendTimeout:   time.Duration(c.SendTimeout) * time.Second,
		instance:      id,
		extraLabels:   c.ExtraLabels,
		logFormat:     c.LogFormat,
	}
	// set the resource handler fns
	if err := eh.initInformer(eventInformer); err != nil {
		return nil, err
	}
	eh.ticker = time.NewTicker(time.Duration(c.FlushInterval) * time.Second)
	return eh, nil
}

// Initialize informer by setting event handler fns
func (eh *EventHandler) initInformer(eventsInformer cache.SharedIndexInformer) error {
	_, err := eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    eh.addEvent,
		UpdateFunc: eh.updateEvent,
		DeleteFunc: eh.deleteEvent,
	})
	return err
}

// Handles new event objects
func (eh *EventHandler) addEvent(obj interface{}) {
	event, _ := obj.(*v1.Event)

	err := eh.handleEvent(event)
	if err != nil {
		level.Error(eh.Log).Log("msg", "Error handling event", "err", err, "event", event)
	}
}

// Handles event object updates. Note that this get triggered on informer resyncs and also
// events occurring more than once (in which case .count is incremented)
func (eh *EventHandler) updateEvent(objOld interface{}, objNew interface{}) {
	eOld, _ := objOld.(*v1.Event)
	eNew, _ := objNew.(*v1.Event)

	if eOld.GetResourceVersion() == eNew.GetResourceVersion() {
		// ignore resync updates
		level.Debug(eh.Log).Log("msg", "Event RV didn't change, ignoring", "eRV", eNew.ResourceVersion)
		return
	}

	err := eh.handleEvent(eNew)
	if err != nil {
		level.Error(eh.Log).Log("msg", "Error handling event", "err", err, "event", eNew)
	}
}

func (eh *EventHandler) handleEvent(event *v1.Event) error {
	eventTs := getTimestamp(event)

	// if event is older than the one stored in cache on startup, we've shipped it
	if eventTs.Before(eh.InitEvent.Timestamp) {
		return nil
	}
	// if event is equal and is in map, we've shipped it
	if eventTs.Equal(eh.InitEvent.Timestamp) {
		if _, ok := eh.InitEvent.RvMap[event.ResourceVersion]; ok {
			return nil
		}
	}

	labels, msg, err := eh.extractEvent(event)
	if err != nil {
		return err
	}

	entry := newEntry(msg, eventTs, labels)
	ok := eh.LogsClient.Instance(eh.LogsInstance).SendEntry(entry, eh.SendTimeout)
	if !ok {
		err = fmt.Errorf("msg=%s entry=%s", "error handing entry off to promtail", entry)
		return err
	}

	// update cache with new "last" event
	err = eh.updateLastEvent(event, eventTs)
	if err != nil {
		return err
	}
	return nil
}

// Called when event objects are removed from etcd, can safely ignore this
func (eh *EventHandler) deleteEvent(obj interface{}) {
}

// extract data from event fields and create labels, etc.
// TODO: ship JSON blobs and allow users to configure using pipelines etc.
// instead of hardcoding labels here
func (eh *EventHandler) extractEvent(event *v1.Event) (model.LabelSet, string, error) {
	var (
		msg      strings.Builder
		fields   = make(map[string]any)
		labels   = make(model.LabelSet)
		appender = appendTextMsg
	)

	if eh.logFormat == logFormatJson {
		appender = appendJsonMsg
	}

	obj := event.InvolvedObject
	if obj.Name == "" {
		return nil, "", fmt.Errorf("no involved object for event")
	}
	appender(&msg, fields, "name", obj.Name, "%s")

	labels[model.LabelName("namespace")] = model.LabelValue(obj.Namespace)
	// TODO(hjet) omit "kubernetes"
	labels[model.LabelName("job")] = model.LabelValue("integrations/kubernetes/eventhandler")
	labels[model.LabelName("instance")] = model.LabelValue(eh.instance)
	labels[model.LabelName("agent_hostname")] = model.LabelValue(eh.instance)
	for _, lbl := range eh.extraLabels {
		labels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	}

	// we add these fields to the log line to reduce label bloat and cardinality
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

	if eh.logFormat == logFormatJson {
		bb, err := json.Marshal(fields)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal Event to JSON: %w", err)
		}
		msg.WriteString(string(bb))
	}

	return labels, strings.TrimSpace(msg.String()), nil
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
	msg.WriteString(fmt.Sprintf(format, value))
	msg.WriteByte(' ')
}

func getTimestamp(event *v1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	return event.EventTime.Time
}

func newEntry(msg string, ts time.Time, labels model.LabelSet) api.Entry {
	entry := logproto.Entry{Timestamp: ts, Line: msg}
	return api.Entry{Labels: labels, Entry: entry}
}

// maintain "last event" state
func (eh *EventHandler) updateLastEvent(e *v1.Event, eventTs time.Time) error {
	eh.Lock()
	defer eh.Unlock()

	eventRv := e.ResourceVersion

	if eh.LastEvent == nil {
		// startup
		eh.LastEvent = &ShippedEvents{Timestamp: eventTs, RvMap: make(map[string]struct{})}
		eh.LastEvent.RvMap[eventRv] = struct{}{}
		return nil
	}

	// if timestamp is the same, add to map
	if eh.LastEvent != nil && eventTs.Equal(eh.LastEvent.Timestamp) {
		eh.LastEvent.RvMap[eventRv] = struct{}{}
		return nil
	}

	// if timestamp is different, create a new ShippedEvents struct
	eh.LastEvent = &ShippedEvents{Timestamp: eventTs, RvMap: make(map[string]struct{})}
	eh.LastEvent.RvMap[eventRv] = struct{}{}
	return nil
}

func (eh *EventHandler) writeOutLastEvent() error {
	level.Info(eh.Log).Log("msg", "Flushing last event to disk")

	eh.Lock()
	defer eh.Unlock()

	if eh.LastEvent == nil {
		level.Info(eh.Log).Log("msg", "No last event to flush, returning")
		return nil
	}

	temp := eh.CachePath + "-new"
	buf, err := json.Marshal(&eh.LastEvent)
	if err != nil {
		return err
	}

	err = os.WriteFile(temp, buf, os.FileMode(cacheFileMode))
	if err != nil {
		return err
	}

	if err = os.Rename(temp, eh.CachePath); err != nil {
		return err
	}
	level.Info(eh.Log).Log("msg", "Flushed last event to disk")
	return nil
}

// RunIntegration runs the eventhandler integration
func (eh *EventHandler) RunIntegration(ctx context.Context) error {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Quick check to make sure logs instance exists
	if i := eh.LogsClient.Instance(eh.LogsInstance); i == nil {
		level.Error(eh.Log).Log("msg", "Logs instance not configured", "instance", eh.LogsInstance)
		cancel()
	}

	cacheDir := filepath.Dir(eh.CachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		level.Error(eh.Log).Log("msg", "Failed to create cache dir", "err", err)
		cancel()
	}

	// cache file to store events shipped (prevents double shipping on restart)
	cacheFile, err := os.OpenFile(eh.CachePath, os.O_RDWR|os.O_CREATE, cacheFileMode)
	if err != nil {
		level.Error(eh.Log).Log("msg", "Failed to open or create cache file", "err", err)
		cancel()
	}

	// attempt to read last timestamp from cache file into a ShippedEvents struct
	initEvent, err := readInitEvent(cacheFile, eh.Log)
	if err != nil {
		level.Error(eh.Log).Log("msg", "Failed to read last event from cache file", "err", err)
		cancel()
	}
	eh.InitEvent = initEvent

	if err = cacheFile.Close(); err != nil {
		level.Error(eh.Log).Log("msg", "Failed to close cache file", "err", err)
		cancel()
	}

	go func() {
		level.Info(eh.Log).Log("msg", "Waiting for cache to sync (initial List of events)")
		isSynced := cache.WaitForCacheSync(ctx.Done(), eh.EventInformer.HasSynced)
		if !isSynced {
			level.Error(eh.Log).Log("msg", "Failed to sync informer cache")
			// maybe want to bail here
			return
		}
		level.Info(eh.Log).Log("msg", "Informer cache synced")
	}()

	// start the informer
	// technically we should prob use the factory here, but since we
	// only have one informer atm, this likely doesn't matter
	go eh.EventInformer.Run(ctx.Done())

	// wait for last event to flush before returning
	wg.Add(1)
	go func() {
		defer wg.Done()
		eh.runTicker(ctx.Done())
	}()
	wg.Wait()

	return nil
}

// write out last event every FlushInterval
func (eh *EventHandler) runTicker(stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			if err := eh.writeOutLastEvent(); err != nil {
				level.Error(eh.Log).Log("msg", "Failed to flush last event", "err", err)
			}
			return
		case <-eh.ticker.C:
			if err := eh.writeOutLastEvent(); err != nil {
				level.Error(eh.Log).Log("msg", "Failed to flush last event", "err", err)
			}
		}
	}
}

func readInitEvent(file *os.File, logger log.Logger) (*ShippedEvents, error) {
	var (
		initEvent = new(ShippedEvents)
	)

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		level.Info(logger).Log("msg", "Cache file empty, setting zero-valued initEvent")
		return initEvent, nil
	}

	dec := json.NewDecoder(file)
	err = dec.Decode(&initEvent)
	if err != nil {
		err = fmt.Errorf("could not read init event from cache: %s. Please delete the cache file", err)
		return nil, err
	}
	level.Info(logger).Log("msg", "Loaded init event from cache file", "initEventTime", initEvent.Timestamp)
	return initEvent, nil
}

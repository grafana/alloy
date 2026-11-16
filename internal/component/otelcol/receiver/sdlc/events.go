package sdlc

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/alloy/internal/build"
)

const (
	rolloutStarted    = "started"
	rolloutSucceeded  = "succeeded"
	rolloutStalled    = "stalled"
	rolloutSuperseded = "superseded"
)

const (
	eventName     = "grafana.sdlc.change"
	attrPrefix    = "grafana.sdlc."
	schemaVersion = "1"

	changeTypeIntent = "intent"
	changeTypeEffect = "effect"

	// Resource change types; rollout events use "rollout_" + their kind.
	changeCreated = "created"
	changeUpdated = "updated"

	// Cap the summary so a workload with many containers can't bloat a record.
	maxSummaryLength = 1024
)

type eventBatch struct {
	logs plog.Logs
	id   string
}

// changeEvent is one grafana.sdlc.change record, before encoding.
type changeEvent struct {
	resourceType, resourceID string
	id                       string
	// traceKey groups related events into one trace; empty leaves it unset.
	traceKey                    string
	versionBefore, versionAfter string
	start, end                  time.Time
	changeType                  string
	// resourceChangeType is the provider-defined provider.resource.change.type.
	resourceChangeType string
	summary            string
	// context adds the event's change.context keys.
	context func(pcommon.Map)
	// object is the changed object; its labels and annotations are extracted.
	object    metav1.Object
	snapshots snapshots
}

// snapshots are the change.diff of an event.
type snapshots struct {
	// before and after are redacted snapshots of the object, nil when absent.
	before, after map[string]any
	// beforeKnown, when set, reports whether a previous state was available.
	beforeKnown *bool
}

var rolloutSummaries = map[string]string{
	rolloutStarted:   "Rollout started",
	rolloutSucceeded: "Rollout succeeded",
	rolloutStalled:   "Rollout stalled: ProgressDeadlineExceeded",
}

func (c *controller) rolloutID(w workload, revision int64) string {
	return stableID(c.opts.clusterUID, string(w.GetUID()), strconv.FormatInt(revision, 10))
}

// rolloutChange is a rollout event of state with the common rollout context.
func (c *controller) rolloutChange(w workload, state rolloutState, status string) changeEvent {
	rolloutID := c.rolloutID(w, state.revision)
	event := rolloutEvent(w, state, status, rolloutID, c.now())
	event.summary = rolloutSummaries[status]
	// Terminal events span the whole rollout; a rollout first seen mid-flight has
	// no recorded start and is reported as a point in time.
	if status != rolloutStarted && !state.startedAt.IsZero() {
		event.start = state.startedAt
	}
	event.context = func(ctx pcommon.Map) { c.putRolloutContext(ctx, w, state, rolloutID) }
	return event
}

// enqueueStarted reports a new rollout. previous is the rollout it replaced, or
// nil when unknown.
func (c *controller) enqueueStarted(w workload, state rolloutState, previous *rolloutState) {
	event := c.rolloutChange(w, state, rolloutStarted)
	event.changeType = changeTypeIntent
	known := previous != nil
	event.snapshots = snapshots{beforeKnown: &known, after: workloadSnapshot(w.kind(), &state.metadata, &state.template)}
	if known {
		event.snapshots.before = workloadSnapshot(w.kind(), &previous.metadata, &previous.template)
	}
	c.enqueueChange(event)
}

// enqueueOutcome reports that a rollout succeeded or stalled.
func (c *controller) enqueueOutcome(w workload, state rolloutState, status string) {
	c.enqueueChange(c.rolloutChange(w, state, status))
}

// enqueueSuperseded reports that the replacement revision replaced an
// unfinished rollout.
func (c *controller) enqueueSuperseded(w workload, state rolloutState, replacement int64) {
	event := c.rolloutChange(w, state, rolloutSuperseded)
	event.summary = "Rollout superseded by revision " + strconv.FormatInt(replacement, 10)
	putContext := event.context
	event.context = func(ctx pcommon.Map) {
		putContext(ctx)
		ctx.PutStr("superseded_by", c.rolloutID(w, replacement))
	}
	c.enqueueChange(event)
}

// rolloutEvent is a point-in-time effect of the rollout. kind distinguishes
// events of one rollout and is part of the event ID.
func rolloutEvent(w workload, state rolloutState, kind, rolloutID string, at time.Time) changeEvent {
	event := changeEvent{
		resourceType: w.kind().resourceType,
		resourceID:   w.GetNamespace() + "/" + w.GetName(),
		id:           stableID(rolloutID, kind),
		traceKey:     rolloutID,
		versionAfter: strconv.FormatInt(state.revision, 10),
		start:        at,
		end:          at,
		changeType:   changeTypeEffect,
		// Rollout kinds are part of event IDs, so they keep their names here.
		resourceChangeType: "rollout_" + kind,
		object:             w,
	}
	if state.previousRevision > 0 {
		event.versionBefore = strconv.FormatInt(state.previousRevision, 10)
	}
	return event
}

// enqueueCreated reports a workload added after the initial listing. Its first
// rollout is reported separately.
func (c *controller) enqueueCreated(w workload) {
	now := c.now()
	metadata, template := snapshotMetadata(w), w.template()
	c.enqueueChange(changeEvent{
		resourceType:       w.kind().resourceType,
		resourceID:         w.GetNamespace() + "/" + w.GetName(),
		id:                 stableID(c.opts.clusterUID, string(w.GetUID()), changeCreated),
		versionAfter:       strconv.FormatInt(w.GetGeneration(), 10),
		start:              now,
		end:                now,
		changeType:         changeTypeEffect,
		resourceChangeType: changeCreated,
		summary:            "Created",
		object:             w,
		snapshots:          snapshots{after: workloadSnapshot(w.kind(), &metadata, &template)},
	})
}

func (c *controller) putRolloutContext(ctx pcommon.Map, w workload, state rolloutState, rolloutID string) {
	ctx.PutInt("revision", state.revision)
	ctx.PutStr("rollout_id", rolloutID)
	ctx.PutStr("namespace", w.GetNamespace())
	if c.opts.clusterName != "" {
		ctx.PutStr("cluster_name", c.opts.clusterName)
	}
	containers := ctx.PutEmptySlice("containers")
	add := func(name, image string, init bool) {
		m := containers.AppendEmpty().SetEmptyMap()
		m.PutStr("name", name)
		m.PutBool("init", init)
		m.PutStr("image", image)
	}
	for _, container := range state.template.Spec.Containers {
		add(container.Name, container.Image, false)
	}
	for _, container := range state.template.Spec.InitContainers {
		add(container.Name, container.Image, true)
	}
	w.putContext(ctx)
}

// putSnapshots adds the before_known, before and after keys of s to diff.
func (c *controller) putSnapshots(diff pcommon.Map, s snapshots, eventID string) {
	if s.beforeKnown != nil {
		diff.PutBool("before_known", *s.beforeKnown)
	}
	put := func(key string, value map[string]any) {
		if value == nil {
			return
		}
		// Unstructured objects hold only types FromRaw accepts.
		if err := diff.PutEmptyMap(key).FromRaw(value); err != nil {
			diff.Remove(key)
			c.opts.logger.Debug("Unable to encode snapshot", "event_id", eventID, "snapshot", key, "err", err)
		}
	}
	put("before", s.before)
	put("after", s.after)
}

// enqueueChange encodes e and queues it for delivery. Exporters reject
// oversized records, which would otherwise retry forever: an event that would
// exceed maxEventBytes drops its snapshots and sets snapshots_omitted, and one
// that's still too large is dropped.
func (c *controller) enqueueChange(e changeEvent) {
	logs := c.changeLogs(e)
	if !fitsEvent(logs) && (!omitSnapshots(logs) || !fitsEvent(logs)) {
		c.opts.logger.Warn("Dropping oversized SDLC change event", "event_id", e.id, "resource_type", e.resourceType, "resource_id", e.resourceID)
		return
	}
	c.queue.Add(&eventBatch{logs: logs, id: e.id})
}

// omitSnapshots removes the before and after snapshots from the record in
// logs. It reports whether there were any.
func omitSnapshots(logs plog.Logs) bool {
	attrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	value, ok := attrs.Get(attrPrefix + "change.diff")
	if !ok {
		return false
	}
	diff := value.Map()
	removedBefore, removedAfter := diff.Remove("before"), diff.Remove("after")
	if !removedBefore && !removedAfter {
		return false
	}
	diff.PutStr("snapshots_omitted", "size")
	return true
}

func (c *controller) changeLogs(e changeEvent) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	res := rl.Resource().Attributes()
	providerID := c.opts.clusterName
	if providerID == "" {
		providerID = c.opts.clusterUID
	}
	res.PutStr(attrPrefix+"provider.type", "k8s")
	res.PutStr(attrPrefix+"provider.id", providerID)
	res.PutStr(attrPrefix+"provider.resource.type", e.resourceType)
	res.PutStr(attrPrefix+"provider.resource.id", e.resourceID)
	res.PutStr(attrPrefix+"provider.resource.change.type", e.resourceChangeType)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.sdlc")
	record := sl.LogRecords().AppendEmpty()
	record.SetEventName(eventName)
	record.SetTimestamp(pcommon.NewTimestampFromTime(e.end))
	record.SetObservedTimestamp(pcommon.NewTimestampFromTime(e.end))
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	// One trace per rollout. Derived from the rollout ID so retries are identical.
	if e.traceKey != "" {
		record.SetTraceID(traceID(e.traceKey))
	}

	attrs := record.Attributes()
	attrs.PutStr(attrPrefix+"schema.version", schemaVersion)
	attrs.PutStr(attrPrefix+"provider.version", build.Version)
	attrs.PutStr(attrPrefix+"event.id", e.id)
	attrs.PutStr(attrPrefix+"status", "UNSET")
	attrs.PutInt(attrPrefix+"start_time_unix_nano", e.start.UnixNano())
	attrs.PutStr(attrPrefix+"type", e.changeType)
	attrs.PutStr(attrPrefix+"change.summary", truncate(e.summary, maxSummaryLength))
	if e.versionBefore != "" {
		attrs.PutStr(attrPrefix+"change.version.before", e.versionBefore)
	}
	attrs.PutStr(attrPrefix+"change.version.after", e.versionAfter)
	if e.object != nil {
		if labels := c.opts.extract.labels(e.object); len(labels) > 0 {
			out := attrs.PutEmptyMap(attrPrefix + "labels")
			for name, value := range labels {
				out.PutStr(name, value)
			}
		}
	}
	ctx, diff := pcommon.NewMap(), pcommon.NewMap()
	if e.context != nil {
		e.context(ctx)
	}
	c.putSnapshots(diff, e.snapshots, e.id)
	if ctx.Len() > 0 {
		ctx.MoveTo(attrs.PutEmptyMap(attrPrefix + "change.context"))
	}
	if diff.Len() > 0 {
		diff.MoveTo(attrs.PutEmptyMap(attrPrefix + "change.diff"))
	}
	return logs
}

// Bound the full OTLP JSON record so exporters don't reject it and retry forever.
const maxEventBytes = 2*1024*1024 - 1024

func wireSize(logs plog.Logs) (int, error) {
	wire, err := (&plog.JSONMarshaler{}).MarshalLogs(logs)
	return len(wire), err
}

// jsonExpansion bounds how much larger OTLP JSON is than protobuf: about 6
// times for escaped control characters or tiny attributes.
const jsonExpansion = 8

func fitsEvent(logs plog.Logs) bool {
	// Most records are far below the limit and skip JSON encoding.
	if (&plog.ProtoMarshaler{}).LogsSize(logs)*jsonExpansion <= maxEventBytes {
		return true
	}
	size, err := wireSize(logs)
	return err == nil && size <= maxEventBytes
}

func traceID(key string) pcommon.TraceID {
	sum := sha256.Sum256([]byte(key))
	var id pcommon.TraceID
	copy(id[:], sum[:16])
	return id
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return strings.ToValidUTF8(s[:limit], "")
}

func stableID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

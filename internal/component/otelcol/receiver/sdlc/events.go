package sdlc

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"

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

	// Cap the summary so a Deployment with many containers can't bloat a record.
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
	summary                     string
	diff                        func(pcommon.Map)
}

func (c *controller) rolloutID(d *appsv1.Deployment, revision int64) string {
	return stableID(c.opts.clusterUID, string(d.UID), strconv.FormatInt(revision, 10))
}

func (c *controller) enqueue(d *appsv1.Deployment, state rolloutState, status string, replacement int64, changes []fieldChange, known bool) {
	now := c.now()
	rolloutID := c.rolloutID(d, state.revision)
	event := rolloutEvent(d, state, status, rolloutID, now)
	switch status {
	case rolloutStarted:
		event.changeType = changeTypeIntent
		event.summary = "Rollout started"
	case rolloutSucceeded:
		event.summary = "Rollout succeeded"
	case rolloutStalled:
		event.summary = "Rollout stalled: ProgressDeadlineExceeded"
	case rolloutSuperseded:
		event.summary = "Rollout superseded by revision " + strconv.FormatInt(replacement, 10)
	}
	// Terminal events span the whole rollout; a rollout first seen mid-flight has
	// no recorded start and is reported as a point in time.
	if status != rolloutStarted && !state.startedAt.IsZero() {
		event.start = state.startedAt
	}
	event.diff = func(diff pcommon.Map) {
		c.putRolloutDiff(diff, d, state, rolloutID)
		switch status {
		case rolloutStarted:
			diff.PutBool("changes_known", known)
			putChanges(diff, changes)
		case rolloutSuperseded:
			diff.PutStr("superseded_by", c.rolloutID(d, replacement))
		}
	}
	c.enqueueChange(event)
}

// rolloutEvent is a point-in-time effect of the rollout. kind distinguishes
// events of one rollout and is part of the event ID.
func rolloutEvent(d *appsv1.Deployment, state rolloutState, kind, rolloutID string, at time.Time) changeEvent {
	event := changeEvent{
		resourceType: "deployment",
		resourceID:   d.Namespace + "/" + d.Name,
		id:           stableID(rolloutID, kind),
		traceKey:     rolloutID,
		versionAfter: strconv.FormatInt(state.revision, 10),
		start:        at,
		end:          at,
		changeType:   changeTypeEffect,
	}
	if state.previousRevision > 0 {
		event.versionBefore = strconv.FormatInt(state.previousRevision, 10)
	}
	return event
}

func (c *controller) putRolloutDiff(diff pcommon.Map, d *appsv1.Deployment, state rolloutState, rolloutID string) {
	diff.PutInt("revision", state.revision)
	diff.PutStr("rollout_id", rolloutID)
	diff.PutStr("namespace", d.Namespace)
	if c.opts.clusterName != "" {
		diff.PutStr("cluster_name", c.opts.clusterName)
	}
	containers := diff.PutEmptySlice("containers")
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
}

// putChanges encodes changes as a "changes" map keyed by field path, omitted
// when empty. Values are present only for fields whose diff rule reports them.
func putChanges(diff pcommon.Map, changes []fieldChange) {
	if len(changes) == 0 {
		return
	}
	out := diff.PutEmptyMap("changes")
	for _, change := range changes {
		entry := out.PutEmptyMap(change.Path)
		entry.PutStr("operation", change.Operation)
		if change.Before != nil {
			entry.PutStr("before", *change.Before)
		}
		if change.After != nil {
			entry.PutStr("after", *change.After)
		}
	}
}

func (c *controller) enqueueChange(e changeEvent) {
	c.queue.Add(&eventBatch{logs: c.changeLogs(e), id: e.id})
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
	res.PutStr(attrPrefix+"resource.type", e.resourceType)
	res.PutStr(attrPrefix+"resource.id", e.resourceID)
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
	if e.diff != nil {
		diff := pcommon.NewMap()
		e.diff(diff)
		if diff.Len() > 0 {
			diff.MoveTo(attrs.PutEmptyMap(attrPrefix + "change.diff"))
		}
	}
	return logs
}

// Bound the full OTLP JSON record so exporters don't reject it and retry forever.
const maxEventBytes = 2*1024*1024 - 1024

func wireSize(logs plog.Logs) (int, error) {
	wire, err := (&plog.JSONMarshaler{}).MarshalLogs(logs)
	return len(wire), err
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

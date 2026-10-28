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
	deployment *appsv1.Deployment
	// kind distinguishes events of one rollout and is part of the event ID.
	kind             string
	rolloutID        string
	revision         int64
	previousRevision int64
	start, end       time.Time
	changeType       string
	summary          string
	diff             func(pcommon.Map)
}

func (c *controller) rolloutID(d *appsv1.Deployment, revision int64) string {
	return stableID(c.opts.clusterUID, string(d.UID), strconv.FormatInt(revision, 10))
}

func (c *controller) enqueue(d *appsv1.Deployment, state rolloutState, status string, replacement int64, changes []templateChange, known bool) {
	now := c.now()
	rolloutID := c.rolloutID(d, state.revision)
	event := changeEvent{
		deployment:       d,
		kind:             status,
		rolloutID:        rolloutID,
		revision:         state.revision,
		previousRevision: state.previousRevision,
		start:            now,
		end:              now,
		changeType:       changeTypeEffect,
	}
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
			putChanges(diff.PutEmptyMap("changes"), changes)
		case rolloutSuperseded:
			diff.PutStr("superseded_by", c.rolloutID(d, replacement))
		}
	}
	c.enqueueChange(event)
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

// putChanges encodes changes as a map keyed by field path. Values are present
// only for fields templateChanges allowlists.
func putChanges(out pcommon.Map, changes []templateChange) {
	for _, change := range changes {
		entry := out.PutEmptyMap(change.path())
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
	d := e.deployment
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	res := rl.Resource().Attributes()
	providerID := c.opts.clusterName
	if providerID == "" {
		providerID = c.opts.clusterUID
	}
	res.PutStr(attrPrefix+"provider.type", "k8s")
	res.PutStr(attrPrefix+"provider.id", providerID)
	res.PutStr(attrPrefix+"resource.type", "deployment")
	res.PutStr(attrPrefix+"resource.id", d.Namespace+"/"+d.Name)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.sdlc")
	record := sl.LogRecords().AppendEmpty()
	record.SetEventName(eventName)
	record.SetTimestamp(pcommon.NewTimestampFromTime(e.end))
	record.SetObservedTimestamp(pcommon.NewTimestampFromTime(e.end))
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	// One trace per rollout. Derived from the rollout ID so retries are identical.
	record.SetTraceID(traceID(e.rolloutID))

	id := stableID(e.rolloutID, e.kind)
	attrs := record.Attributes()
	attrs.PutStr(attrPrefix+"schema.version", schemaVersion)
	attrs.PutStr(attrPrefix+"provider.version", build.Version)
	attrs.PutStr(attrPrefix+"event.id", id)
	attrs.PutStr(attrPrefix+"status", "UNSET")
	attrs.PutInt(attrPrefix+"start_time_unix_nano", e.start.UnixNano())
	attrs.PutStr(attrPrefix+"type", e.changeType)
	attrs.PutStr(attrPrefix+"change.summary", truncate(e.summary, maxSummaryLength))
	if e.previousRevision > 0 {
		attrs.PutStr(attrPrefix+"change.version.before", strconv.FormatInt(e.previousRevision, 10))
	}
	attrs.PutStr(attrPrefix+"change.version.after", strconv.FormatInt(e.revision, 10))
	if e.diff != nil {
		e.diff(attrs.PutEmptyMap(attrPrefix + "change.diff"))
	}
	c.queue.Add(&eventBatch{logs: logs, id: id})
}

func traceID(rolloutID string) pcommon.TraceID {
	sum := sha256.Sum256([]byte(rolloutID))
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

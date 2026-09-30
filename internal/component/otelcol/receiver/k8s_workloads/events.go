package k8s_workloads

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	rolloutStarted    = "started"
	rolloutSucceeded  = "succeeded"
	rolloutStalled    = "stalled"
	rolloutSuperseded = "superseded"
)

const eventPrefix = "grafana.sdlc.k8s.deployment.rollout."

type eventBatch struct {
	logs plog.Logs
	id   string
}
type rolloutContainer struct {
	Name  string `json:"name"`
	Init  bool   `json:"init"`
	Image string `json:"image"`
}
type rolloutEvent struct {
	Name         string             `json:"name"`
	UID          string             `json:"uid"`
	RolloutID    string             `json:"rollout_id"`
	Revision     int64              `json:"revision"`
	Status       string             `json:"status"`
	ObservedAt   string             `json:"observed_at"`
	SupersededBy string             `json:"superseded_by,omitempty"`
	Containers   []rolloutContainer `json:"containers"`
	ChangesKnown *bool              `json:"changes_known,omitempty"`
	ChangeTypes  []string           `json:"change_types,omitempty"`
	Changes      []templateChange   `json:"changes,omitempty"`
}

func (c *controller) enqueue(d *appsv1.Deployment, state rolloutState, status string, replacement int64, kinds []string, changes []templateChange, known bool) {
	now := c.now()
	rolloutID := func(revision int64) string {
		return stableID(c.opts.clusterUID, string(d.UID), strconv.FormatInt(revision, 10))
	}
	payload := rolloutEvent{Name: d.Name, UID: string(d.UID), RolloutID: rolloutID(state.revision), Revision: state.revision, Status: status, ObservedAt: now.UTC().Format(time.RFC3339Nano), Containers: []rolloutContainer{}}
	for _, container := range state.template.Spec.Containers {
		payload.Containers = append(payload.Containers, rolloutContainer{Name: container.Name, Image: container.Image})
	}
	for _, container := range state.template.Spec.InitContainers {
		payload.Containers = append(payload.Containers, rolloutContainer{Name: container.Name, Init: true, Image: container.Image})
	}
	if replacement > 0 {
		payload.SupersededBy = rolloutID(replacement)
	}
	if status == rolloutStarted {
		payload.ChangesKnown = &known
		payload.ChangeTypes = kinds
		payload.Changes = changes
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	attrs := rl.Resource().Attributes()
	attrs.PutStr("k8s.cluster.uid", c.opts.clusterUID)
	if c.opts.clusterName != "" {
		attrs.PutStr("k8s.cluster.name", c.opts.clusterName)
	}
	attrs.PutStr("k8s.namespace.name", d.Namespace)
	attrs.PutStr("k8s.deployment.uid", string(d.UID))
	attrs.PutStr("k8s.deployment.name", d.Name)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.k8s_workloads")
	record := sl.LogRecords().AppendEmpty()
	record.SetEventName(eventPrefix + status)
	record.SetTimestamp(pcommon.NewTimestampFromTime(now))
	record.SetObservedTimestamp(record.Timestamp())
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	id := stableID(payload.RolloutID, status)
	record.Attributes().PutStr("grafana.sdlc.event.id", id)
	record.Attributes().PutInt("grafana.sdlc.schema.version", 1)
	record.Body().SetStr(string(encoded))
	c.queue.Add(&eventBatch{logs: logs, id: id})
}

func stableID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

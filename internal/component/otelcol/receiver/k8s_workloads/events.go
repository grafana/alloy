package k8s_workloads

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const eventPrefix = "grafana.sdlc.k8s."
const errorEventName = "grafana.sdlc.reporting.error"

// A snapshot is one log record, not a collection of log records. Its JSON body
// remains atomic when an OTLP gateway splits requests into individual records.
type eventBatch struct {
	logs                                                    plog.Logs
	operation, kind, namespace, namespaceUID, entityUID, id string
	collected                                               time.Time
	scan                                                    bool
	reported                                                bool
}

type imageData struct {
	container, reference, imageName, tag, digest string
	init                                         bool
}

type containerSnapshot struct {
	Name      string `json:"name"`
	Init      bool   `json:"init"`
	Image     string `json:"image"`
	ImageName string `json:"image_name"`
	Tag       string `json:"tag,omitempty"`
	// Multiple images may be running concurrently during a rollout.
	Resolved []resolvedImage `json:"resolved"`
}
type resolvedImage struct {
	ID            string `json:"id"`
	Digest        string `json:"digest,omitempty"`
	ReplicaSetUID string `json:"replicaset_uid"`
}

type deploymentSnapshot struct {
	UID                string                       `json:"uid"`
	Name               string                       `json:"name"`
	Generation         int64                        `json:"generation"`
	ObservedGeneration int64                        `json:"observed_generation"`
	ResourceVersion    string                       `json:"resource_version"`
	CreatedAt          metav1.Time                  `json:"created_at"`
	DeletionTimestamp  *metav1.Time                 `json:"deletion_timestamp,omitempty"`
	Labels             map[string]string            `json:"labels"`
	Paused             bool                         `json:"paused"`
	Status             string                       `json:"status"`
	Desired            int32                        `json:"desired_replicas"`
	Replicas           int32                        `json:"replicas"`
	Updated            int32                        `json:"updated_replicas"`
	Ready              int32                        `json:"ready_replicas"`
	Available          int32                        `json:"available_replicas"`
	Unavailable        int32                        `json:"unavailable_replicas"`
	Conditions         []appsv1.DeploymentCondition `json:"conditions"`
	Containers         []containerSnapshot          `json:"containers"`
}
type namespaceSnapshot struct {
	UID               string                `json:"uid"`
	Name              string                `json:"name"`
	Phase             corev1.NamespacePhase `json:"phase"`
	Labels            map[string]string     `json:"labels"`
	CreatedAt         metav1.Time           `json:"created_at"`
	DeletionTimestamp *metav1.Time          `json:"deletion_timestamp,omitempty"`
}

func describeNamespace(ns *corev1.Namespace) namespaceSnapshot {
	return namespaceSnapshot{UID: string(ns.UID), Name: ns.Name, Phase: ns.Status.Phase, Labels: ns.Labels, CreatedAt: ns.CreationTimestamp, DeletionTimestamp: ns.DeletionTimestamp}
}

func deploymentStatus(d *appsv1.Deployment) string {
	if d.DeletionTimestamp != nil {
		return "terminating"
	}
	if d.Spec.Paused {
		return "paused"
	}
	// Conditions from a previous generation must not label a new rollout stalled.
	if d.Status.ObservedGeneration < d.Generation {
		return "progressing"
	}
	for _, condition := range d.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded" {
			return "stalled"
		}
		if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
			return "degraded"
		}
	}
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	if d.Status.UpdatedReplicas == desired && d.Status.Replicas == desired && d.Status.AvailableReplicas == desired && d.Status.UnavailableReplicas == 0 {
		return "healthy"
	}
	return "progressing"
}

func describeDeployment(d *appsv1.Deployment, sets []appsv1.ReplicaSet, pods []corev1.Pod) deploymentSnapshot {
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	conditions := append([]appsv1.DeploymentCondition{}, d.Status.Conditions...)
	out := deploymentSnapshot{UID: string(d.UID), Name: d.Name, Generation: d.Generation, ObservedGeneration: d.Status.ObservedGeneration,
		ResourceVersion: d.ResourceVersion, CreatedAt: d.CreationTimestamp, DeletionTimestamp: d.DeletionTimestamp, Labels: d.Labels, Paused: d.Spec.Paused,
		Status: deploymentStatus(d), Desired: desired, Replicas: d.Status.Replicas, Updated: d.Status.UpdatedReplicas, Ready: d.Status.ReadyReplicas,
		Available: d.Status.AvailableReplicas, Unavailable: d.Status.UnavailableReplicas, Conditions: conditions, Containers: []containerSnapshot{}}
	owned := map[string]bool{}
	for _, rs := range sets {
		if owner := metav1.GetControllerOf(&rs); owner != nil && owner.UID == d.UID {
			owned[string(rs.UID)] = true
		}
	}
	appendContainer := func(container corev1.Container, init bool) {
		image := newImageData(container.Name, container.Image, init)
		entry := containerSnapshot{Name: container.Name, Init: init, Image: image.reference, ImageName: image.imageName, Tag: image.tag, Resolved: []resolvedImage{}}
		seen := map[string]bool{}
		for _, pod := range pods {
			owner := metav1.GetControllerOf(&pod)
			if owner == nil || !owned[string(owner.UID)] {
				continue
			}
			statuses := pod.Status.ContainerStatuses
			if init {
				statuses = pod.Status.InitContainerStatuses
			}
			for _, status := range statuses {
				if status.Name != container.Name || status.ImageID == "" {
					continue
				}
				key := string(owner.UID) + "/" + status.ImageID
				if seen[key] {
					continue
				}
				seen[key] = true
				entry.Resolved = append(entry.Resolved, resolvedImage{ID: status.ImageID, Digest: imageDigest(status.ImageID), ReplicaSetUID: string(owner.UID)})
			}
		}
		sort.Slice(entry.Resolved, func(i, j int) bool {
			return entry.Resolved[i].ReplicaSetUID+entry.Resolved[i].ID < entry.Resolved[j].ReplicaSetUID+entry.Resolved[j].ID
		})
		out.Containers = append(out.Containers, entry)
	}
	for _, container := range d.Spec.Template.Spec.Containers {
		appendContainer(container, false)
	}
	for _, container := range d.Spec.Template.Spec.InitContainers {
		appendContainer(container, true)
	}
	return out
}

func makeEvent(clusterUID, clusterName, namespace, namespaceUID, entityUID, operation, kind string, collected time.Time, body any) *eventBatch {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	attrs := rl.Resource().Attributes()
	attrs.PutStr("k8s.cluster.uid", clusterUID)
	if clusterName != "" {
		attrs.PutStr("k8s.cluster.name", clusterName)
	}
	if namespace != "" {
		attrs.PutStr("k8s.namespace.name", namespace)
	}
	if namespaceUID != "" {
		attrs.PutStr("k8s.namespace.uid", namespaceUID)
	}
	if entityUID != "" && kind == "deployment" {
		attrs.PutStr("k8s.deployment.uid", entityUID)
	}
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.k8s_workloads")
	record := sl.LogRecords().AppendEmpty()
	record.SetEventName(eventPrefix + kind + "." + operation)
	record.SetTimestamp(pcommon.NewTimestampFromTime(collected))
	record.SetObservedTimestamp(record.Timestamp())
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	id := stableID(clusterUID, namespaceUID, namespace, entityUID, operation, kind, collected.UTC().Format(time.RFC3339Nano))
	record.Attributes().PutStr("grafana.sdlc.event.id", id)
	record.Attributes().PutInt("grafana.sdlc.schema.version", 1)
	encoded, err := json.Marshal(body)
	if err != nil {
		panic(fmt.Sprintf("invalid internal event body: %v", err))
	}
	record.Body().SetStr(string(encoded))
	return &eventBatch{logs: logs, operation: operation, kind: kind, namespace: namespace, namespaceUID: namespaceUID, entityUID: entityUID, id: id, collected: collected}
}

func stableID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}
func newImageData(container, imageReference string, init bool) imageData {
	image := imageData{container: container, reference: imageReference, imageName: imageReference, init: init}
	nameAndTag := imageReference
	if at := strings.LastIndex(nameAndTag, "@"); at >= 0 {
		image.digest = nameAndTag[at+1:]
		nameAndTag = nameAndTag[:at]
	}
	image.imageName = nameAndTag
	if colon := strings.LastIndex(nameAndTag, ":"); colon > strings.LastIndex(nameAndTag, "/") {
		image.imageName = nameAndTag[:colon]
		image.tag = nameAndTag[colon+1:]
	}
	return image
}

func imageDigest(imageID string) string {
	if idx := strings.LastIndex(imageID, "sha256:"); idx >= 0 {
		return imageID[idx:]
	}
	return ""
}

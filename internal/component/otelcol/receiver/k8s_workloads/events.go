package k8s_workloads

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
)

type rolloutPhase string

const (
	phaseStarted    rolloutPhase = "started"
	phaseSucceeded  rolloutPhase = "succeeded"
	phaseStalled    rolloutPhase = "stalled"
	phaseSuperseded rolloutPhase = "superseded"
)

const imageResolvedEventName = "grafana.sdlc.k8s.deployment.rollout.container.image_resolved"

type imageData struct {
	container string
	reference string
	imageName string
	tag       string
	imageID   string
	digest    string
	init      bool
}

type eventData struct {
	clusterUID  string
	clusterName string
	deployment  *appsv1.Deployment
	replicaSet  *appsv1.ReplicaSet
	generation  int64
	revision    string
	phase       rolloutPhase
	images      []imageData
}

type eventBatch struct{ logs plog.Logs }

type inventoryEvent string

const (
	inventoryObserved inventoryEvent = "observed"
	inventoryDeleted  inventoryEvent = "deleted"
)

func deploymentPhase(deployment *appsv1.Deployment) rolloutPhase {
	for _, condition := range deployment.Status.Conditions {
		progressDeadlineExceeded := condition.Type == appsv1.DeploymentProgressing &&
			condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded"
		replicaFailure := condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue
		if progressDeadlineExceeded || replicaFailure || condition.Reason == "ReplicaSetCreateError" {
			return phaseStalled
		}
	}

	desired := desiredReplicas(deployment)
	if deployment.Status.ObservedGeneration >= deployment.Generation &&
		deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.Replicas == desired &&
		deployment.Status.AvailableReplicas == desired &&
		deployment.Status.UnavailableReplicas == 0 {

		return phaseSucceeded
	}
	return phaseStarted
}

func collectImages(podSpec corev1.PodSpec, replicaSet *appsv1.ReplicaSet, pods corelisters.PodLister) []imageData {
	configured := make([]imageData, 0, len(podSpec.Containers)+len(podSpec.InitContainers))
	for _, container := range podSpec.Containers {
		configured = append(configured, newImageData(container.Name, container.Image, false))
	}
	for _, container := range podSpec.InitContainers {
		configured = append(configured, newImageData(container.Name, container.Image, true))
	}
	if replicaSet == nil {
		return configured
	}

	matchingPods, err := pods.Pods(replicaSet.Namespace).List(labelsForReplicaSet(replicaSet))
	if err != nil {
		return configured
	}
	resolved := make(map[string]map[string]string)
	for _, pod := range matchingPods {
		owner := metav1.GetControllerOf(pod)
		if owner == nil || owner.UID != replicaSet.UID {
			continue
		}
		statuses := append(append([]corev1.ContainerStatus(nil), pod.Status.ContainerStatuses...), pod.Status.InitContainerStatuses...)
		for _, status := range statuses {
			if status.ImageID == "" {
				continue
			}
			if resolved[status.Name] == nil {
				resolved[status.Name] = make(map[string]string)
			}
			resolved[status.Name][status.ImageID] = imageDigest(status.ImageID)
		}
	}

	result := make([]imageData, 0, len(configured))
	for _, image := range configured {
		imageIDs := make([]string, 0, len(resolved[image.container]))
		for imageID := range resolved[image.container] {
			imageIDs = append(imageIDs, imageID)
		}
		sort.Strings(imageIDs)
		if len(imageIDs) == 0 {
			result = append(result, image)
			continue
		}
		for _, imageID := range imageIDs {
			resolvedImage := image
			resolvedImage.imageID = imageID
			if digest := resolved[image.container][imageID]; digest != "" {
				resolvedImage.digest = digest
			}
			result = append(result, resolvedImage)
		}
	}
	return result
}

func labelsForReplicaSet(replicaSet *appsv1.ReplicaSet) labels.Selector {
	selector, err := metav1.LabelSelectorAsSelector(replicaSet.Spec.Selector)
	if err != nil {
		return labels.Nothing()
	}
	return selector
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

func digestKeys(images []imageData) map[string]struct{} {
	result := make(map[string]struct{})
	for _, image := range images {
		if image.digest != "" {
			result[image.container+"\x00"+image.digest] = struct{}{}
		}
	}
	return result
}

func buildEventBatch(data eventData) eventBatch {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	putDeploymentResource(resourceLogs.Resource().Attributes(), data)

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.k8s_workloads")
	appendEventRecord(scopeLogs.LogRecords(), data)
	return eventBatch{logs: logs}
}

func buildImageResolvedEventBatch(data eventData) eventBatch {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	putDeploymentResource(resourceLogs.Resource().Attributes(), data)

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.k8s_workloads")
	for i := range data.images {
		appendImageResolvedEventRecord(scopeLogs.LogRecords(), data, &data.images[i])
	}
	return eventBatch{logs: logs}
}

func buildInventoryEventBatch(data eventData, event inventoryEvent, fingerprint string) eventBatch {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	putDeploymentResource(resourceLogs.Resource().Attributes(), data)

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/grafana/alloy/otelcol.receiver.k8s_workloads")
	record := scopeLogs.LogRecords().AppendEmpty()
	record.SetEventName("grafana.sdlc.k8s.deployment." + string(event))
	now := pcommon.NewTimestampFromTime(time.Now())
	record.SetTimestamp(now)
	record.SetObservedTimestamp(now)
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	record.Body().SetStr(fmt.Sprintf("Kubernetes Deployment %s", event))

	attrs := record.Attributes()
	eventIDParts := []string{data.clusterUID, data.deployment.Namespace, string(data.deployment.UID), string(event)}
	if event == inventoryObserved {
		eventIDParts = append(eventIDParts, fingerprint)
	}
	attrs.PutStr("grafana.sdlc.event.id", stableID(eventIDParts...))
	attrs.PutInt("grafana.sdlc.deployment.generation", data.deployment.Generation)
	attrs.PutStr("grafana.sdlc.deployment.revision", data.revision)

	if event == inventoryObserved {
		attrs.PutStr("deployment.status", string(data.phase))
		attrs.PutStr("grafana.sdlc.inventory.version", fingerprint)
		attrs.PutInt("grafana.sdlc.rollout.desired_replicas", int64(desiredReplicas(data.deployment)))
		attrs.PutInt("grafana.sdlc.rollout.replicas", int64(data.deployment.Status.Replicas))
		attrs.PutInt("grafana.sdlc.rollout.updated_replicas", int64(data.deployment.Status.UpdatedReplicas))
		attrs.PutInt("grafana.sdlc.rollout.ready_replicas", int64(data.deployment.Status.ReadyReplicas))
		attrs.PutInt("grafana.sdlc.rollout.available_replicas", int64(data.deployment.Status.AvailableReplicas))
		attrs.PutInt("grafana.sdlc.rollout.unavailable_replicas", int64(data.deployment.Status.UnavailableReplicas))
		putStringMap(attrs.PutEmptyMap("grafana.sdlc.k8s.deployment.labels"), data.deployment.Labels)
		putInventoryContainers(attrs.PutEmptySlice("grafana.sdlc.deployment.containers"), data.images)
	}

	return eventBatch{logs: logs}
}

func putDeploymentResource(resource pcommon.Map, data eventData) {
	resource.PutStr("k8s.cluster.uid", data.clusterUID)
	if data.clusterName != "" {
		resource.PutStr("k8s.cluster.name", data.clusterName)
	}
	resource.PutStr("k8s.namespace.name", data.deployment.Namespace)
	resource.PutStr("k8s.deployment.name", data.deployment.Name)
	resource.PutStr("k8s.deployment.uid", string(data.deployment.UID))
}

func putStringMap(destination pcommon.Map, values map[string]string) {
	for key, value := range values {
		destination.PutStr(key, value)
	}
}

func putInventoryContainers(destination pcommon.Slice, images []imageData) {
	for _, image := range images {
		container := destination.AppendEmpty().SetEmptyMap()
		container.PutStr("name", image.container)
		container.PutBool("init", image.init)
		container.PutStr("image.reference", image.reference)
		container.PutStr("image.name", image.imageName)
		if image.tag != "" {
			container.PutEmptySlice("image.tags").AppendEmpty().SetStr(image.tag)
		}
		if image.imageID != "" {
			container.PutStr("image.id", image.imageID)
		}
		if image.digest != "" {
			container.PutEmptySlice("image.repo_digests").AppendEmpty().SetStr(image.imageName + "@" + image.digest)
		}
	}
}

func inventoryFingerprint(data eventData) string {
	parts := []string{
		fmt.Sprint(data.deployment.Generation),
		data.revision,
		string(data.phase),
		fmt.Sprint(desiredReplicas(data.deployment)),
		fmt.Sprint(data.deployment.Status.Replicas),
		fmt.Sprint(data.deployment.Status.UpdatedReplicas),
		fmt.Sprint(data.deployment.Status.ReadyReplicas),
		fmt.Sprint(data.deployment.Status.AvailableReplicas),
		fmt.Sprint(data.deployment.Status.UnavailableReplicas),
	}
	labelKeys := make([]string, 0, len(data.deployment.Labels))
	for key := range data.deployment.Labels {
		labelKeys = append(labelKeys, key)
	}
	sort.Strings(labelKeys)
	for _, key := range labelKeys {
		parts = append(parts, "label", key, data.deployment.Labels[key])
	}
	for _, image := range data.images {
		parts = append(parts, "image", image.container, fmt.Sprint(image.init), image.reference, image.imageID, image.digest)
	}
	return stableID(parts...)
}

func appendEventRecord(records plog.LogRecordSlice, data eventData) {
	generation := data.generation
	if generation == 0 {
		generation = data.deployment.Generation
	}
	rolloutID := stableID(data.clusterUID, data.deployment.Namespace, string(data.deployment.UID), fmt.Sprint(generation))
	eventIDParts := []string{rolloutID, string(data.phase)}
	record := records.AppendEmpty()
	record.SetEventName("grafana.sdlc.k8s.deployment.rollout." + string(data.phase))
	now := pcommon.NewTimestampFromTime(time.Now())
	record.SetTimestamp(now)
	record.SetObservedTimestamp(now)
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	record.Body().SetStr(fmt.Sprintf("Kubernetes Deployment rollout %s", data.phase))

	attrs := record.Attributes()
	attrs.PutStr("deployment.id", rolloutID)
	attrs.PutStr("deployment.name", data.deployment.Name)
	attrs.PutStr("deployment.status", string(data.phase))
	attrs.PutStr("grafana.sdlc.event.id", stableID(eventIDParts...))
	attrs.PutStr("grafana.sdlc.deployment.revision", data.revision)
	attrs.PutInt("grafana.sdlc.deployment.generation", generation)
	attrs.PutStr("grafana.sdlc.rollout.phase", string(data.phase))
	attrs.PutInt("grafana.sdlc.rollout.desired_replicas", int64(desiredReplicas(data.deployment)))
	attrs.PutInt("grafana.sdlc.rollout.updated_replicas", int64(data.deployment.Status.UpdatedReplicas))
	attrs.PutInt("grafana.sdlc.rollout.available_replicas", int64(data.deployment.Status.AvailableReplicas))
	attrs.PutInt("grafana.sdlc.rollout.unavailable_replicas", int64(data.deployment.Status.UnavailableReplicas))
	if data.replicaSet != nil {
		attrs.PutStr("k8s.replicaset.name", data.replicaSet.Name)
		attrs.PutStr("k8s.replicaset.uid", string(data.replicaSet.UID))
	}
	putInventoryContainers(attrs.PutEmptySlice("grafana.sdlc.deployment.containers"), data.images)
}

func appendImageResolvedEventRecord(records plog.LogRecordSlice, data eventData, image *imageData) {
	generation := data.generation
	if generation == 0 {
		generation = data.deployment.Generation
	}
	rolloutID := stableID(data.clusterUID, data.deployment.Namespace, string(data.deployment.UID), fmt.Sprint(generation))
	record := records.AppendEmpty()
	record.SetEventName(imageResolvedEventName)
	now := pcommon.NewTimestampFromTime(time.Now())
	record.SetTimestamp(now)
	record.SetObservedTimestamp(now)
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	record.Body().SetStr("Kubernetes Deployment rollout container image resolved")

	attrs := record.Attributes()
	attrs.PutStr("deployment.id", rolloutID)
	attrs.PutStr("deployment.name", data.deployment.Name)
	attrs.PutStr("grafana.sdlc.event.id", stableID(rolloutID, imageResolvedEventName, image.container, image.digest))
	attrs.PutStr("grafana.sdlc.deployment.revision", data.revision)
	attrs.PutInt("grafana.sdlc.deployment.generation", generation)
	attrs.PutInt("grafana.sdlc.rollout.desired_replicas", int64(desiredReplicas(data.deployment)))
	attrs.PutInt("grafana.sdlc.rollout.updated_replicas", int64(data.deployment.Status.UpdatedReplicas))
	attrs.PutInt("grafana.sdlc.rollout.available_replicas", int64(data.deployment.Status.AvailableReplicas))
	attrs.PutInt("grafana.sdlc.rollout.unavailable_replicas", int64(data.deployment.Status.UnavailableReplicas))
	if data.replicaSet != nil {
		attrs.PutStr("k8s.replicaset.name", data.replicaSet.Name)
		attrs.PutStr("k8s.replicaset.uid", string(data.replicaSet.UID))
	}
	attrs.PutStr("k8s.container.name", image.container)
	attrs.PutBool("grafana.sdlc.container.init", image.init)
	attrs.PutStr("grafana.sdlc.container.image.reference", image.reference)
	attrs.PutStr("container.image.name", image.imageName)
	if image.tag != "" {
		attrs.PutEmptySlice("container.image.tags").AppendEmpty().SetStr(image.tag)
	}
	if image.imageID != "" {
		attrs.PutStr("container.image.id", image.imageID)
	}
	if image.digest != "" {
		attrs.PutEmptySlice("container.image.repo_digests").AppendEmpty().SetStr(image.imageName + "@" + image.digest)
	}
}

func desiredReplicas(deployment *appsv1.Deployment) int32 {
	if deployment.Spec.Replicas == nil {
		return 1
	}
	return *deployment.Spec.Replicas
}

func stableID(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(part))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

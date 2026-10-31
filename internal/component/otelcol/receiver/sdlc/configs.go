package sdlc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

const (
	contentAll      = "all"
	contentKeys     = "keys"
	contentMetadata = "metadata"
)

// ConfigMapsArguments configures ConfigMap change tracking.
type ConfigMapsArguments struct {
	Enabled bool   `alloy:"enabled,attr,optional"`
	Content string `alloy:"content,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *ConfigMapsArguments) SetToDefault() {
	*args = ConfigMapsArguments{Enabled: true, Content: contentKeys}
}

// Validate implements syntax.Validator.
func (args *ConfigMapsArguments) Validate() error {
	switch args.Content {
	case contentAll, contentKeys, contentMetadata:
		return nil
	}
	return fmt.Errorf("content must be one of %q, %q, or %q", contentAll, contentKeys, contentMetadata)
}

// SecretsArguments configures Secret change tracking. Secret content is never read.
type SecretsArguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *SecretsArguments) SetToDefault() {
	*args = SecretsArguments{Enabled: true}
}

type configKind struct {
	name    string // resource type: configmap or secret
	content string
	rule    *diffRule
}

func newConfigKind(name, content string) configKind {
	return configKind{name: name, content: content, rule: configRule(content == contentAll)}
}

// configRule compares ConfigMaps, and Secret and ConfigMap metadata. Data values
// are reported only when values is set; in keys mode the informer cache holds
// hashes, so changed keys are still detected.
func configRule(values bool) *diffRule {
	ignore := &diffRule{mode: diffIgnore}
	value := &diffRule{mode: diffValue}
	operation := &diffRule{mode: diffOperation}
	content := operation
	if values {
		content = value
	}
	return &diffRule{fields: map[string]*diffRule{
		"apiVersion": ignore,
		"kind":       ignore,
		"metadata": {fields: map[string]*diffRule{
			"name":              ignore,
			"namespace":         ignore,
			"uid":               ignore,
			"resourceVersion":   ignore,
			"generation":        ignore,
			"creationTimestamp": ignore,
			"managedFields":     ignore,
			"labels":            {each: value},
			// Annotation values are never exported: last-applied-configuration
			// annotations contain the full manifest, including Secret data.
			"annotations":       {each: operation},
			"ownerReferences":   operation,
			"finalizers":        operation,
			"deletionTimestamp": operation,
		}},
		"data":       {each: content},
		"binaryData": {each: content},
		"immutable":  value,
	}}
}

// Secrets, and ConfigMaps in metadata mode, are read through the metadata API so
// their content never reaches Alloy. Each list page is transformed before the
// reflector retains it; Helm release Secrets are often close to the size limit.
func (c *controller) configInformers() ([]cache.SharedIndexInformer, error) {
	informers := []cache.SharedIndexInformer{}
	add := func(kind configKind, informer cache.SharedIndexInformer) error {
		_, err := informer.AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
			// The initial listing establishes a baseline, like Deployments.
			AddFunc:    func(obj any, initial bool) { c.observeConfig(kind, nil, obj, initial) },
			UpdateFunc: func(old, obj any) { c.observeConfig(kind, old, obj, false) },
			// Deletions only drop the object from the informer cache.
		})
		informers = append(informers, informer)
		return err
	}
	switch content := c.opts.configMapContent; content {
	case "":
	case contentMetadata:
		if err := add(newConfigKind("configmap", content), c.metadataInformer(corev1.SchemeGroupVersion.WithResource("configmaps"))); err != nil {
			return nil, err
		}
	default:
		api := c.opts.client.CoreV1().ConfigMaps(metav1.NamespaceAll)
		informer := newPagedInformer(c.opts.client, &corev1.ConfigMap{}, func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return api.List(ctx, options)
		}, api.Watch, configWatchTransform(content == contentAll))
		if err := add(newConfigKind("configmap", content), informer); err != nil {
			return nil, err
		}
	}
	if c.opts.secrets {
		if err := add(newConfigKind("secret", contentMetadata), c.metadataInformer(corev1.SchemeGroupVersion.WithResource("secrets"))); err != nil {
			return nil, err
		}
	}
	return informers, nil
}

func (c *controller) metadataInformer(resource schema.GroupVersionResource) cache.SharedIndexInformer {
	api := c.opts.metadata.Resource(resource)
	return newPagedInformer(c.opts.metadata, &metav1.PartialObjectMetadata{}, func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
		return api.List(ctx, options)
	}, api.Watch, configWatchTransform(false))
}

// Keep only compared fields. Without values, data and annotations are replaced by
// hashes so changes stay detectable. The transform runs on list pages and again in
// the informer, so hashing must be idempotent.
func configWatchTransform(values bool) cache.TransformFunc {
	metadata := func(m metav1.ObjectMeta) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: m.Name, Namespace: m.Namespace, UID: m.UID,
			ResourceVersion: m.ResourceVersion, CreationTimestamp: m.CreationTimestamp,
			DeletionTimestamp: m.DeletionTimestamp, Labels: m.Labels, Annotations: redactValues(m.Annotations),
			OwnerReferences: m.OwnerReferences, Finalizers: m.Finalizers}
	}
	return func(obj any) (any, error) {
		switch value := obj.(type) {
		case *corev1.ConfigMap:
			out := &corev1.ConfigMap{TypeMeta: value.TypeMeta, ObjectMeta: metadata(value.ObjectMeta), Immutable: value.Immutable, Data: value.Data, BinaryData: value.BinaryData}
			if !values {
				out.Data = redactValues(value.Data)
				out.BinaryData = make(map[string][]byte, len(value.BinaryData))
				for key, b := range value.BinaryData {
					out.BinaryData[key] = []byte(redact(string(b)))
				}
			}
			return out, nil
		case *metav1.PartialObjectMetadata:
			return &metav1.PartialObjectMetadata{TypeMeta: value.TypeMeta, ObjectMeta: metadata(value.ObjectMeta)}, nil
		default:
			return obj, nil
		}
	}
}

const redactedPrefix = "sha256:"

func redact(value string) string {
	if len(value) == len(redactedPrefix)+sha256.Size*2 && strings.HasPrefix(value, redactedPrefix) {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return redactedPrefix + hex.EncodeToString(sum[:])
}

func redactValues(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = redact(value)
	}
	return out
}

func configMeta(obj any) (metav1.ObjectMeta, bool) {
	switch value := obj.(type) {
	case *corev1.ConfigMap:
		return value.ObjectMeta, true
	case *metav1.PartialObjectMetadata:
		return value.ObjectMeta, true
	}
	return metav1.ObjectMeta{}, false
}

// observeConfig is the single entry point for ConfigMap and Secret changes, and
// the place to apply namespace and name exclusions.
func (c *controller) observeConfig(kind configKind, old, obj any, initial bool) {
	if initial {
		return
	}
	after, ok := configMeta(obj)
	if !ok {
		return
	}
	event, before := changeCreated, metav1.ObjectMeta{}
	if old != nil {
		if before, ok = configMeta(old); !ok {
			return
		}
		event = changeUpdated
	}
	changes := diffObjects(kind.rule, old, obj)
	if _, metadataOnly := obj.(*metav1.PartialObjectMetadata); metadataOnly && old != nil && len(changes) == 0 &&
		before.ResourceVersion != "" && before.ResourceVersion != after.ResourceVersion {
		// The metadata API omits data. A new resource version without a tracked
		// metadata change can only come from a content (or managedFields) update.
		changes = append(changes, fieldChange{Path: "data", Operation: operationModified})
	}
	// Creation is reported even when no tracked field is set, such as a Secret
	// without labels or annotations.
	if len(changes) == 0 && old != nil {
		return
	}
	c.enqueueConfig(kind, event, before.ResourceVersion, after, changes)
}

func (c *controller) enqueueConfig(kind configKind, event, previousVersion string, m metav1.ObjectMeta, changes []fieldChange) {
	now := c.now()
	summary := "Created"
	if event == changeUpdated {
		summary = "Updated"
	}
	e := changeEvent{
		resourceType: kind.name,
		resourceID:   m.Namespace + "/" + m.Name,
		// Every write has a unique resource version; repeated observations share an ID.
		id:                 stableID(c.opts.clusterUID, string(m.UID), m.ResourceVersion, event),
		versionBefore:      previousVersion,
		versionAfter:       m.ResourceVersion,
		start:              now,
		end:                now,
		changeType:         changeTypeEffect,
		resourceChangeType: event,
		summary:            summary,
		diff:               func(diff pcommon.Map) { putChanges(diff, changes) },
	}
	logs := c.changeLogs(e)
	// Exporters reject oversized records, which would otherwise retry forever.
	if kind.content == contentAll {
		if size, err := wireSize(logs); err != nil || size > maxEventBytes {
			for i, change := range changes {
				if strings.HasPrefix(change.Path, "data.") || strings.HasPrefix(change.Path, "binaryData.") {
					changes[i].Before, changes[i].After = nil, nil
				}
			}
			logs = c.changeLogs(e)
		}
	}
	c.queue.Add(&eventBatch{logs: logs, id: e.id})
}

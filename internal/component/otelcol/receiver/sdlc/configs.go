package sdlc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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
	resourceKind
	content string
	rule    *snapshotRule
}

func newConfigKind(kind resourceKind, content string) configKind {
	return configKind{resourceKind: kind, content: content, rule: configRule(content == contentAll)}
}

// configRule redacts ConfigMaps, and Secret and ConfigMap metadata, in
// snapshots. Data values are shown only when values is set; in keys mode the
// informer cache holds fingerprints, so placeholders still show changed values.
func configRule(values bool) *snapshotRule {
	content := redactRule
	if values {
		content = keepRule
	}
	return &snapshotRule{fields: map[string]*snapshotRule{
		// typed sets them; informer objects don't carry them reliably.
		"apiVersion": omitRule,
		"kind":       omitRule,
		"metadata":   {fields: map[string]*snapshotRule{"annotations": annotationsRule}},
		"data":       {each: content},
		"binaryData": {each: content},
	}}
}

// snapshot is the snapshot of obj, a cached ConfigMap or PartialObjectMetadata.
func (k configKind) snapshot(obj any) map[string]any {
	var out map[string]any
	switch value := obj.(type) {
	case *corev1.ConfigMap:
		out = snapshot(k.rule, value)
	case *metav1.PartialObjectMetadata:
		out = snapshot(k.rule, value)
	}
	return typed(out, k.resourceKind)
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
		if err := add(newConfigKind(configMapKind, content), c.metadataInformer(corev1.SchemeGroupVersion.WithResource("configmaps"))); err != nil {
			return nil, err
		}
	default:
		api := c.opts.client.CoreV1().ConfigMaps(metav1.NamespaceAll)
		informer := newPagedInformer(c.opts.client, &corev1.ConfigMap{}, func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return api.List(ctx, options)
		}, api.Watch, configWatchTransform(content == contentAll, c.keepAnnotation))
		if err := add(newConfigKind(configMapKind, content), informer); err != nil {
			return nil, err
		}
	}
	if c.opts.secrets {
		if err := add(newConfigKind(secretKind, contentMetadata), c.metadataInformer(corev1.SchemeGroupVersion.WithResource("secrets"))); err != nil {
			return nil, err
		}
	}
	return informers, nil
}

func (c *controller) metadataInformer(resource schema.GroupVersionResource) cache.SharedIndexInformer {
	api := c.opts.metadata.Resource(resource)
	return newPagedInformer(c.opts.metadata, &metav1.PartialObjectMetadata{}, func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
		return api.List(ctx, options)
	}, api.Watch, configWatchTransform(false, c.keepAnnotation))
}

// keepAnnotation keeps annotation values that extraction or exclusion reads.
func (c *controller) keepAnnotation(key string) bool {
	return c.opts.extract.keepAnnotation(key) || c.opts.exclude.keepAnnotation(key)
}

// Keep only the fields snapshots show, with the resource version and deletion
// timestamp. Without values, data is replaced by fingerprints so changes stay
// detectable. Ignored annotations are fingerprinted too, unless keep reports
// true for extraction or exclusion. The transform runs on list pages and again
// in the informer, so fingerprinting must be idempotent.
func configWatchTransform(values bool, keep func(string) bool) cache.TransformFunc {
	metadata := func(m *metav1.ObjectMeta) metav1.ObjectMeta {
		out := snapshotMetadata(m)
		out.ResourceVersion, out.DeletionTimestamp = m.ResourceVersion, m.DeletionTimestamp
		for key, value := range out.Annotations {
			if ignoredAnnotation(key) && !keep(key) {
				out.Annotations[key] = fingerprint(value)
			}
		}
		return out
	}
	return func(obj any) (any, error) {
		switch value := obj.(type) {
		case *corev1.ConfigMap:
			out := &corev1.ConfigMap{TypeMeta: value.TypeMeta, ObjectMeta: metadata(&value.ObjectMeta), Immutable: value.Immutable, Data: value.Data, BinaryData: value.BinaryData}
			if !values {
				out.Data = fingerprintValues(value.Data)
				out.BinaryData = make(map[string][]byte, len(value.BinaryData))
				for key, b := range value.BinaryData {
					out.BinaryData[key] = []byte(fingerprint(string(b)))
				}
			}
			return out, nil
		case *metav1.PartialObjectMetadata:
			return &metav1.PartialObjectMetadata{TypeMeta: value.TypeMeta, ObjectMeta: metadata(&value.ObjectMeta)}, nil
		default:
			return obj, nil
		}
	}
}

// fingerprintPrefix starts the fingerprints that replace values the informer
// cache must not hold. Unlike snapshot placeholders, fingerprints are full
// SHA-256 hashes, so no change is missed, and they never leave the process.
const fingerprintPrefix = "sha256:"

func fingerprint(value string) string {
	if len(value) == len(fingerprintPrefix)+sha256.Size*2 && strings.HasPrefix(value, fingerprintPrefix) {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return fingerprintPrefix + hex.EncodeToString(sum[:])
}

func fingerprintValues(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = fingerprint(value)
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
// applies exclusions. An update that leaves the scope of exclude rules reports
// nothing; later updates compare against it. Deletion isn't reported, including
// the updates that remove finalizers from an object being deleted.
func (c *controller) observeConfig(kind configKind, old, obj any, initial bool) {
	if initial {
		return
	}
	after, ok := configMeta(obj)
	if !ok || after.DeletionTimestamp != nil || c.opts.exclude.excludes(kind.resourceType, &after) {
		return
	}
	afterSnapshot := kind.snapshot(obj)
	if old == nil {
		// Creation is reported even when the object has no other fields, such
		// as a Secret without labels or annotations.
		c.enqueueConfig(kind, changeCreated, "", after, snapshots{after: afterSnapshot})
		return
	}
	before, ok := configMeta(old)
	if !ok || c.opts.exclude.excludes(kind.resourceType, &before) {
		return
	}
	beforeSnapshot := kind.snapshot(old)
	if sameState(beforeSnapshot, afterSnapshot) && !contentChanged(obj, before, after) {
		return
	}
	known := true
	c.enqueueConfig(kind, changeUpdated, before.ResourceVersion, after, snapshots{before: beforeSnapshot, after: afterSnapshot, beforeKnown: &known})
}

// contentChanged reports whether an update of an object read through the
// metadata API, whose snapshots show the same state, changed its content. The
// metadata API omits data, so a new resource version is assumed to come from a
// content update, unless leader election explains it. Client-side kubectl
// apply changes the content and last-applied-configuration together.
func contentChanged(obj any, before, after metav1.ObjectMeta) bool {
	_, metadataOnly := obj.(*metav1.PartialObjectMetadata)
	return metadataOnly && before.ResourceVersion != "" && before.ResourceVersion != after.ResourceVersion &&
		before.Annotations[leaderAnnotation] == after.Annotations[leaderAnnotation]
}

// enqueueConfig reports a change of m.
func (c *controller) enqueueConfig(kind configKind, event, previousVersion string, m metav1.ObjectMeta, s snapshots) {
	now := c.now()
	summary := "Created"
	if event == changeUpdated {
		summary = "Updated"
	}
	c.enqueueChange(changeEvent{
		resourceType: kind.resourceType,
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
		object:             &m,
		snapshots:          s,
	})
}

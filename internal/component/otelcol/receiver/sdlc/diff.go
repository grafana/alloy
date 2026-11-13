package sdlc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// snapshotMode is how a field appears in the snapshots of change.diff. Fields
// keep their values by default; rules redact fields known to hold credentials
// and omit fields that only repeat other data.
type snapshotMode int

const (
	// keepValue keeps the value. Fields without a rule use it.
	keepValue snapshotMode = iota
	// redactValue keeps the shape of the value and replaces every scalar with
	// a redacted placeholder.
	redactValue
	// redactQuery keeps a URL path and replaces its query string, which can
	// hold tokens, with a redacted placeholder.
	redactQuery
	// omitField leaves the field out.
	omitField
)

var (
	keepRule        = &snapshotRule{mode: keepValue}
	redactRule      = &snapshotRule{mode: redactValue}
	redactQueryRule = &snapshotRule{mode: redactQuery}
	omitRule        = &snapshotRule{mode: omitField}
)

// leaderAnnotation is updated by leader election, without a change of the
// object's configuration.
const leaderAnnotation = "control-plane.alpha.kubernetes.io/leader"

// ignoredAnnotations are set by tools or Kubernetes and only repeat other data
// or change without a configuration change. last-applied-configuration holds
// the full manifest, including Secret data.
var ignoredAnnotations = []string{
	lastAppliedAnnotation,
	leaderAnnotation,
	revisionAnnotation,
	"deprecated.daemonset.template.generation",
}

// annotationsRule keeps annotations, except ignored annotations.
var annotationsRule = func() *snapshotRule {
	rule := &snapshotRule{fields: map[string]*snapshotRule{}}
	for _, key := range ignoredAnnotations {
		rule.fields[key] = omitRule
	}
	return rule
}()

func ignoredAnnotation(key string) bool {
	return slices.Contains(ignoredAnnotations, key)
}

// snapshotRule describes how one node of an unstructured object appears in
// snapshots. A rule with fields or each applies to a map, child by child; a
// rule with items applies to every element of a list. Otherwise, mode applies
// to the whole node.
type snapshotRule struct {
	mode snapshotMode
	// fields holds rules for named children; each applies to every other key.
	// Children without a rule use keepRule.
	fields map[string]*snapshotRule
	each   *snapshotRule
	items  *snapshotRule
}

// childRule is the rule of the key child of a map rule.
func (r *snapshotRule) childRule(key string) *snapshotRule {
	if child := r.fields[key]; child != nil {
		return child
	}
	if r.each != nil {
		return r.each
	}
	return keepRule
}

// snapshotMetadata is the metadata snapshots show: identity and descriptive
// fields, without fields that change with every write or are internal to the
// API server. Labels and annotations are copied, so the result doesn't change
// with m.
func snapshotMetadata(m metav1.Object) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: m.GetName(), Namespace: m.GetNamespace(), UID: m.GetUID(),
		CreationTimestamp: m.GetCreationTimestamp(), Labels: maps.Clone(m.GetLabels()), Annotations: maps.Clone(m.GetAnnotations()),
		OwnerReferences: m.GetOwnerReferences(), Finalizers: m.GetFinalizers()}
}

// snapshot is obj, an API object or part of one, redacted according to rule.
// It's nil when obj is nil or has no fields to show.
func snapshot[T any](rule *snapshotRule, obj *T) map[string]any {
	if obj == nil {
		return nil
	}
	// Conversion only fails for types that aren't structs.
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil
	}
	out, _ := snapshotNode(rule, raw).(map[string]any)
	return out
}

func snapshotNode(rule *snapshotRule, value any) any {
	if isEmpty(value) || rule.mode == omitField {
		return nil
	}
	switch {
	case rule.items != nil:
		list, ok := value.([]any)
		if !ok {
			return redactShape(value)
		}
		return nonEmpty(collect(list, func(item any) any { return snapshotNode(rule.items, item) }))
	case rule.fields != nil || rule.each != nil:
		m, ok := value.(map[string]any)
		if !ok {
			return redactShape(value)
		}
		out := make(map[string]any, len(m))
		for key, child := range m {
			if v := snapshotNode(rule.childRule(key), child); v != nil {
				out[key] = v
			}
		}
		return nonEmpty(out)
	}
	switch rule.mode {
	case redactValue:
		return redactShape(value)
	case redactQuery:
		path, ok := value.(string)
		if !ok {
			return redactShape(value)
		}
		if i := strings.IndexByte(path, '?'); i >= 0 {
			return path[:i+1] + redacted(path[i+1:])
		}
		return path
	}
	return value
}

// redactShape keeps map keys and list elements and replaces every scalar.
func redactShape(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if r := redactShape(child); r != nil {
				out[key] = r
			}
		}
		return nonEmpty(out)
	case []any:
		return nonEmpty(collect(v, redactShape))
	}
	if isEmpty(value) {
		return nil
	}
	return redacted(value)
}

// collect maps the elements of list, dropping nil results.
func collect(list []any, f func(any) any) []any {
	out := make([]any, 0, len(list))
	for _, item := range list {
		if v := f(item); v != nil {
			out = append(out, v)
		}
	}
	return out
}

// nonEmpty is v, or nil when v is empty.
func nonEmpty[T map[string]any | []any](v T) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

func isEmpty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	}
	return false
}

// typed sets the apiVersion and kind of a snapshot.
func typed(snapshot map[string]any, kind resourceKind) map[string]any {
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	snapshot["apiVersion"], snapshot["kind"] = kind.apiVersion, kind.kind
	return snapshot
}

// sameState reports whether two snapshots of an object show the same state.
// The resource version changes with every write, so it's not compared.
func sameState(a, b map[string]any) bool {
	return reflect.DeepEqual(withoutVersion(a), withoutVersion(b))
}

func withoutVersion(snapshot map[string]any) map[string]any {
	m, _ := snapshot["metadata"].(map[string]any)
	if _, ok := m["resourceVersion"]; !ok {
		return snapshot
	}
	out, metadata := maps.Clone(snapshot), maps.Clone(m)
	delete(metadata, "resourceVersion")
	out["metadata"] = metadata
	return out
}

// redactedMarker starts the placeholder that replaces redacted values in
// snapshots.
const redactedMarker = "[redacted:"

// redactionKey keys the short hash in placeholders. It's random per process:
// placeholders in the before and after snapshots of an event show whether a
// value changed, but can't be checked against guessed values or matched across
// restarts. Events are built once and retries resend the same record, so a
// retried event keeps its placeholders.
var redactionKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}()

var redactionMACs = sync.Pool{New: func() any { return hmac.New(sha256.New, redactionKey) }}

// redacted is the placeholder for value, such as "[redacted:1a2b3c4d]".
func redacted(value any) string {
	mac := redactionMACs.Get().(hash.Hash)
	defer redactionMACs.Put(mac)
	mac.Reset()
	// Strings, the common case, skip JSON encoding. The prefix keeps a string
	// distinct from another value with the same encoding.
	if s, ok := value.(string); ok {
		_, _ = mac.Write([]byte{'s'})
		_, _ = mac.Write([]byte(s))
	} else {
		// Unstructured values always encode.
		encoded, _ := json.Marshal(value)
		_, _ = mac.Write([]byte{'j'})
		_, _ = mac.Write(encoded)
	}
	var sum [sha256.Size]byte
	return redactedMarker + hex.EncodeToString(mac.Sum(sum[:0])[:4]) + "]"
}

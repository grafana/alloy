package sdlc

import (
	"encoding/json"
	"sort"
	"strconv"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	operationAdded    = "added"
	operationRemoved  = "removed"
	operationModified = "modified"
)

// diffMode is how much of a changed field is reported. Objects can contain
// credentials, so only fields declared as diffValue report before/after.
type diffMode int

const (
	// diffName reports that the field changed, without values. Fields without
	// a rule fall back to this, so new Kubernetes fields never export values.
	diffName diffMode = iota
	// diffOperation reports whether the field was added, removed or modified.
	diffOperation
	// diffValue reports the operation with before/after values.
	diffValue
	// diffIgnore never reports the field.
	diffIgnore
)

// diffRule describes how one node of an unstructured object is compared. A
// rule with fields or each is a branch and is compared child by child;
// otherwise it's a leaf reported according to mode.
type diffRule struct {
	mode diffMode
	// fields holds rules for named children; each applies to every other map
	// key, or to every element of a keyBy list.
	fields map[string]*diffRule
	each   *diffRule
	// keyBy compares a list of objects as a map keyed by this field. Elements
	// that are branches also report their own addition or removal.
	keyBy string
	// order, when set on a keyBy list, is the path reported when the same
	// elements appear in a different order.
	order string
	// as replaces the path of this node and its children.
	as string
	// format renders a leaf value for comparison and reporting.
	format func(any) (string, bool)
}

func (r *diffRule) branch() bool {
	return r.fields != nil || r.each != nil
}

// fieldChange is one entry of a change.diff "changes" map.
type fieldChange struct {
	Path      string  `json:"path"`
	Operation string  `json:"operation"`
	Before    *string `json:"before,omitempty"`
	After     *string `json:"after,omitempty"`
}

// diffObjects compares two API objects according to rule. Either may be nil.
// Changes are sorted by path.
func diffObjects(rule *diffRule, before, after any) []fieldChange {
	changes := []fieldChange{}
	walkDiff(&changes, rule, "", toUnstructured(before), toUnstructured(after))
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func toUnstructured(obj any) any {
	switch obj := obj.(type) {
	case nil:
		return nil
	case map[string]any:
		return obj
	}
	out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil
	}
	return out
}

func walkDiff(changes *[]fieldChange, rule *diffRule, path string, before, after any) {
	if rule.as != "" {
		path = rule.as
	}
	if rule.mode == diffIgnore || sameValue(before, after) {
		return
	}
	switch {
	case rule.keyBy != "":
		walkList(changes, rule, path, before, after)
	case rule.branch():
		a, _ := before.(map[string]any)
		b, _ := after.(map[string]any)
		for _, key := range unionKeys(a, b) {
			child := rule.fields[key]
			if child == nil {
				child = rule.each
			}
			childPath := join(path, key)
			if child == nil {
				if !sameValue(a[key], b[key]) {
					*changes = append(*changes, fieldChange{Path: childPath, Operation: operationModified})
				}
				continue
			}
			walkDiff(changes, child, childPath, a[key], b[key])
		}
	default:
		*changes = append(*changes, leafChange(rule, path, before, after))
	}
}

func walkList(changes *[]fieldChange, rule *diffRule, path string, before, after any) {
	a, aOrder := keyed(before, rule.keyBy)
	b, bOrder := keyed(after, rule.keyBy)
	each := rule.each
	if each == nil {
		each = &diffRule{}
	}
	sameKeys := len(a) == len(b)
	for _, key := range unionKeys(a, b) {
		old, had := a[key]
		next, has := b[key]
		elemPath := join(path, key)
		if had != has {
			sameKeys = false
			if each.branch() {
				operation := operationAdded
				if had {
					operation = operationRemoved
				}
				*changes = append(*changes, fieldChange{Path: elemPath, Operation: operation})
			}
		}
		walkDiff(changes, each, elemPath, old, next)
	}
	if rule.order != "" && sameKeys && !equality.Semantic.DeepEqual(aOrder, bOrder) {
		*changes = append(*changes, fieldChange{Path: rule.order, Operation: operationModified})
	}
}

func leafChange(rule *diffRule, path string, before, after any) fieldChange {
	if rule.mode == diffName {
		return fieldChange{Path: path, Operation: operationModified}
	}
	change := fieldChange{Path: path, Operation: operationModified}
	switch {
	case isEmpty(before):
		change.Operation = operationAdded
	case isEmpty(after):
		change.Operation = operationRemoved
	}
	if rule.mode == diffValue {
		change.Before = renderValue(rule, before)
		change.After = renderValue(rule, after)
	}
	return change
}

func renderValue(rule *diffRule, value any) *string {
	if isEmpty(value) {
		return nil
	}
	if rule.format != nil {
		s, ok := rule.format(value)
		if !ok {
			return nil
		}
		return &s
	}
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case bool:
		s = strconv.FormatBool(v)
	case int64:
		s = strconv.FormatInt(v, 10)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		s = string(encoded)
	}
	return &s
}

// sameValue treats missing, null and empty values as equal, matching
// equality.Semantic on the typed objects.
func sameValue(a, b any) bool {
	if isEmpty(a) || isEmpty(b) {
		return isEmpty(a) && isEmpty(b)
	}
	return equality.Semantic.DeepEqual(a, b)
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

func keyed(list any, key string) (map[string]any, []string) {
	items, _ := list.([]any)
	out := make(map[string]any, len(items))
	order := make([]string, 0, len(items))
	for _, item := range items {
		m, _ := item.(map[string]any)
		name, _ := m[key].(string)
		out[name] = m
		order = append(order, name)
	}
	return out, order
}

func unionKeys(a, b map[string]any) []string {
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

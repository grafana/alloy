package sdlc

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	opEquals       = "equals"
	opNotEquals    = "not-equals"
	opExists       = "exists"
	opDoesNotExist = "does-not-exist"

	excludeField      = "field"
	excludeLabel      = "label"
	excludeAnnotation = "annotation"

	fieldNamespace = "metadata.namespace"
	fieldName      = "metadata.name"
	fieldKind      = "kind"
)

// excludeKinds maps the Kinds that field rules accept to resource types.
var excludeKinds = map[string]string{
	"Deployment":  "deployment",
	"StatefulSet": "statefulset",
	"DaemonSet":   "daemonset",
	"ConfigMap":   "configmap",
	"Secret":      "secret",
}

// ExcludeArguments ignores objects that match all of its rules. Rules follow
// the filter blocks of otelcol.processor.k8sattributes.
type ExcludeArguments struct {
	Fields      []ExcludeRule           `alloy:"field,block,optional"`
	Labels      []ExcludeRule           `alloy:"label,block,optional"`
	Annotations []ExcludeAnnotationRule `alloy:"annotation,block,optional"`
}

// ExcludeRule matches the value of a field (metadata.namespace, metadata.name
// or kind) or a label.
type ExcludeRule struct {
	Key   string `alloy:"key,attr"`
	Value string `alloy:"value,attr"`
	Op    string `alloy:"op,attr,optional"`
}

// ExcludeAnnotationRule matches the value of an annotation, or its presence.
type ExcludeAnnotationRule struct {
	Key   string `alloy:"key,attr"`
	Value string `alloy:"value,attr,optional"`
	Op    string `alloy:"op,attr,optional"`
}

type excludeCondition struct {
	block, key, value, op string
}

// excluder applies exclude rules. The zero value excludes nothing.
type excluder struct {
	rules [][]excludeCondition
}

func newExcluder(args []ExcludeArguments) (excluder, error) {
	var out excluder
	for _, block := range args {
		if len(block.Fields)+len(block.Labels)+len(block.Annotations) == 0 {
			return excluder{}, fmt.Errorf("exclude: at least one field, label, or annotation block is required")
		}
		rule := []excludeCondition{}
		for _, field := range block.Fields {
			if field.Key != fieldNamespace && field.Key != fieldName && field.Key != fieldKind {
				return excluder{}, fmt.Errorf(`exclude: field: key must be "metadata.namespace", "metadata.name", or "kind"`)
			}
			condition, err := newExcludeCondition(excludeField, field.Key, field.Value, field.Op)
			if err != nil {
				return excluder{}, err
			}
			if field.Key == fieldKind {
				kind, ok := excludeKinds[field.Value]
				if !ok {
					return excluder{}, fmt.Errorf("exclude: field: kind must be one of %q", slices.Sorted(maps.Keys(excludeKinds)))
				}
				condition.value = kind
			}
			rule = append(rule, condition)
		}
		for _, label := range block.Labels {
			condition, err := newExcludeCondition(excludeLabel, label.Key, label.Value, label.Op)
			if err != nil {
				return excluder{}, err
			}
			rule = append(rule, condition)
		}
		for _, annotation := range block.Annotations {
			condition, err := newExcludeCondition(excludeAnnotation, annotation.Key, annotation.Value, annotation.Op)
			if err != nil {
				return excluder{}, err
			}
			// Informer caches never keep this value, so only its presence can match.
			if annotation.Key == lastAppliedAnnotation && condition.compares() {
				return excluder{}, fmt.Errorf("exclude: annotation: %q supports only op %q or %q", lastAppliedAnnotation, opExists, opDoesNotExist)
			}
			rule = append(rule, condition)
		}
		out.rules = append(out.rules, rule)
	}
	return out, nil
}

func newExcludeCondition(block, key, value, op string) (excludeCondition, error) {
	condition := excludeCondition{block: block, key: key, value: value, op: cmp.Or(op, opEquals)}
	if key == "" {
		return excludeCondition{}, fmt.Errorf("exclude: %s: key is required", block)
	}
	switch condition.op {
	case opEquals, opNotEquals:
		if value == "" {
			return excludeCondition{}, fmt.Errorf("exclude: %s: value is required with op %q", block, condition.op)
		}
	case opExists, opDoesNotExist:
		if block != excludeAnnotation {
			return excludeCondition{}, fmt.Errorf("exclude: %s: op %q is only supported for annotations", block, condition.op)
		}
		if value != "" {
			return excludeCondition{}, fmt.Errorf("exclude: %s: value can't be set with op %q", block, condition.op)
		}
	default:
		return excludeCondition{}, fmt.Errorf("exclude: %s: op must be one of %q, %q, %q, or %q", block, opEquals, opNotEquals, opExists, opDoesNotExist)
	}
	return condition, nil
}

// compares reports whether the condition reads a value, not just presence.
func (c excludeCondition) compares() bool {
	return c.op == opEquals || c.op == opNotEquals
}

func (c excludeCondition) matches(resourceType string, m metav1.Object) bool {
	var value string
	var ok bool
	switch c.block {
	case excludeField:
		switch c.key {
		case fieldNamespace:
			value, ok = m.GetNamespace(), true
		case fieldName:
			value, ok = m.GetName(), true
		case fieldKind:
			value, ok = resourceType, true
		}
	case excludeLabel:
		value, ok = m.GetLabels()[c.key]
	case excludeAnnotation:
		value, ok = m.GetAnnotations()[c.key]
	}
	switch c.op {
	case opEquals:
		return ok && value == c.value
	case opNotEquals:
		// Like Kubernetes selectors, a missing key isn't equal to any value.
		return !ok || value != c.value
	case opExists:
		return ok
	default:
		return !ok
	}
}

// excludes reports whether any rule matches m, an object of resourceType.
func (e excluder) excludes(resourceType string, m metav1.Object) bool {
	return slices.ContainsFunc(e.rules, func(rule []excludeCondition) bool {
		for _, condition := range rule {
			if !condition.matches(resourceType, m) {
				return false
			}
		}
		return true
	})
}

// keepAnnotation reports whether an annotation value must stay readable in
// informer caches that otherwise hold hashes. Presence checks match hashes too.
func (e excluder) keepAnnotation(key string) bool {
	for _, rule := range e.rules {
		for _, condition := range rule {
			if condition.block == excludeAnnotation && condition.key == key && condition.compares() {
				return true
			}
		}
	}
	return false
}

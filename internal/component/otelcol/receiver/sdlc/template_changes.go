package sdlc

import (
	"reflect"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// Values are allowlisted: configuration can contain credentials, including in
// annotations, commands, probes and environment variables.
type templateChange struct {
	Container string  `json:"container,omitempty"`
	Init      bool    `json:"init,omitempty"`
	Field     string  `json:"field"`
	Operation string  `json:"operation"`
	Before    *string `json:"before,omitempty"`
	After     *string `json:"after,omitempty"`
}

func templateChanges(before, after corev1.PodTemplateSpec) []templateChange {
	changes := []templateChange{}
	add := func(change templateChange) {
		changes = append(changes, change)
	}
	for _, field := range changedFields(before.ObjectMeta, after.ObjectMeta) {
		add(templateChange{Field: "metadata." + field, Operation: "modified"})
	}
	for _, field := range changedFields(before.Spec, after.Spec, "containers", "initContainers") {
		add(templateChange{Field: "spec." + field, Operation: "modified"})
	}
	containers := func(old, next []corev1.Container, init bool) {
		oldByName, nextByName := map[string]corev1.Container{}, map[string]corev1.Container{}
		names := map[string]bool{}
		oldOrder, nextOrder := []string{}, []string{}
		for _, c := range old {
			oldByName[c.Name] = c
			names[c.Name] = true
			oldOrder = append(oldOrder, c.Name)
		}
		for _, c := range next {
			nextByName[c.Name] = c
			names[c.Name] = true
			nextOrder = append(nextOrder, c.Name)
		}
		orderedNames := make([]string, 0, len(names))
		for name := range names {
			orderedNames = append(orderedNames, name)
		}
		sort.Strings(orderedNames)
		sameNames := len(oldByName) == len(nextByName)
		for _, name := range orderedNames {
			previous, existed := oldByName[name]
			current, exists := nextByName[name]
			if !existed || !exists {
				sameNames = false
				operation := "added"
				if !exists {
					operation = "removed"
				}
				add(templateChange{Container: name, Init: init, Field: "container", Operation: operation})
			}
			change := func(field string, before, after *string) {
				operation := "modified"
				if before == nil && after != nil {
					operation = "added"
				}
				if before != nil && after == nil {
					operation = "removed"
				}
				add(templateChange{Container: name, Init: init, Field: field, Operation: operation, Before: before, After: after})
			}
			if previous.Image != current.Image {
				var a, b *string
				if existed {
					a = &previous.Image
				}
				if exists {
					b = &current.Image
				}
				change("image", a, b)
			}
			resourceChanges := func(field string, a, b corev1.ResourceList) {
				keys := map[corev1.ResourceName]bool{}
				for k := range a {
					keys[k] = true
				}
				for k := range b {
					keys[k] = true
				}
				ordered := make([]string, 0, len(keys))
				for k := range keys {
					ordered = append(ordered, string(k))
				}
				sort.Strings(ordered)
				for _, key := range ordered {
					old, had := a[corev1.ResourceName(key)]
					next, has := b[corev1.ResourceName(key)]
					if had == has && old.Cmp(next) == 0 {
						continue
					}
					var oldValue, newValue *string
					if had {
						value := old.String()
						oldValue = &value
					}
					if has {
						value := next.String()
						newValue = &value
					}
					change("resources."+field+"."+key, oldValue, newValue)
				}
			}
			resourceChanges("requests", previous.Resources.Requests, current.Resources.Requests)
			resourceChanges("limits", previous.Resources.Limits, current.Resources.Limits)
			for _, field := range changedFields(previous.Resources, current.Resources, "requests", "limits") {
				change("resources."+field, nil, nil)
			}
			for _, field := range changedFields(previous, current, "name", "image", "resources") {
				change(field, nil, nil)
			}
		}
		if sameNames && !equality.Semantic.DeepEqual(oldOrder, nextOrder) {
			field := "spec.containers.order"
			if init {
				field = "spec.initContainers.order"
			}
			add(templateChange{Field: field, Operation: "modified"})
		}
	}
	containers(before.Spec.Containers, after.Spec.Containers, false)
	containers(before.Spec.InitContainers, after.Spec.InitContainers, true)
	return changes
}

// Report only top-level API field names for redacted structs. This covers new
// Kubernetes fields automatically without accidentally exporting their values.
func changedFields(before, after any, skip ...string) []string {
	a, b := reflect.ValueOf(before), reflect.ValueOf(after)
	ignored := map[string]bool{}
	for _, name := range skip {
		ignored[name] = true
	}
	fields := []string{}
	for i := 0; i < a.NumField(); i++ {
		name := strings.Split(a.Type().Field(i).Tag.Get("json"), ",")[0]
		if name == "" || name == "-" || ignored[name] {
			continue
		}
		if !equality.Semantic.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields
}

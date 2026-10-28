package sdlc

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	operationAdded    = "added"
	operationRemoved  = "removed"
	operationModified = "modified"
)

// Values are allowlisted: configuration can contain credentials, including in
// annotations, commands, probes and environment variables. Only fields that
// can't carry secrets report before/after; environment variables report names.
type templateChange struct {
	Container string  `json:"container,omitempty"`
	Init      bool    `json:"init,omitempty"`
	Field     string  `json:"field"`
	Operation string  `json:"operation"`
	Before    *string `json:"before,omitempty"`
	After     *string `json:"after,omitempty"`
}

// path is the key of the change within a change.diff "changes" map.
func (c templateChange) path() string {
	if c.Container == "" {
		return c.Field
	}
	prefix := "containers."
	if c.Init {
		prefix = "initContainers."
	}
	if c.Field == "container" {
		return prefix + c.Container
	}
	return prefix + c.Container + "." + c.Field
}

func templateChanges(before, after corev1.PodTemplateSpec) []templateChange {
	changes := []templateChange{}
	add := func(change templateChange) {
		changes = append(changes, change)
	}
	// Entries are sorted by key so output is deterministic.
	stringMap := func(prefix string, a, b map[string]string) {
		keys := map[string]bool{}
		for k := range a {
			keys[k] = true
		}
		for k := range b {
			keys[k] = true
		}
		ordered := make([]string, 0, len(keys))
		for k := range keys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			old, had := a[key]
			next, has := b[key]
			if had == has && old == next {
				continue
			}
			add(valueChange("", false, prefix+"."+key, optional(old, had), optional(next, has)))
		}
	}
	for _, field := range changedFields(before.ObjectMeta, after.ObjectMeta, "labels") {
		add(templateChange{Field: "metadata." + field, Operation: operationModified})
	}
	stringMap("metadata.labels", before.Labels, after.Labels)
	for _, field := range changedFields(before.Spec, after.Spec, "containers", "initContainers", "serviceAccountName", "nodeSelector") {
		add(templateChange{Field: "spec." + field, Operation: operationModified})
	}
	if before.Spec.ServiceAccountName != after.Spec.ServiceAccountName {
		add(valueChange("", false, "spec.serviceAccountName", optional(before.Spec.ServiceAccountName, before.Spec.ServiceAccountName != ""), optional(after.Spec.ServiceAccountName, after.Spec.ServiceAccountName != "")))
	}
	stringMap("spec.nodeSelector", before.Spec.NodeSelector, after.Spec.NodeSelector)
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
				operation := operationAdded
				if !exists {
					operation = operationRemoved
				}
				add(templateChange{Container: name, Init: init, Field: "container", Operation: operation})
			}
			change := func(field string, before, after *string) {
				add(valueChange(name, init, field, before, after))
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
			if previous.ImagePullPolicy != current.ImagePullPolicy {
				change("imagePullPolicy", optional(string(previous.ImagePullPolicy), previous.ImagePullPolicy != ""), optional(string(current.ImagePullPolicy), current.ImagePullPolicy != ""))
			}
			if a, b := portsString(previous.Ports), portsString(current.Ports); a != b {
				change("ports", optional(a, a != ""), optional(b, b != ""))
			}
			// Environment values may be credentials: report which variables
			// changed, never what they changed to.
			oldEnv, newEnv := envByName(previous.Env), envByName(current.Env)
			for _, envName := range sortedKeys(oldEnv, newEnv) {
				old, had := oldEnv[envName]
				next, has := newEnv[envName]
				switch {
				case had && has && equality.Semantic.DeepEqual(old, next):
				case had && has:
					add(templateChange{Container: name, Init: init, Field: "env." + envName, Operation: operationModified})
				case has:
					add(templateChange{Container: name, Init: init, Field: "env." + envName, Operation: operationAdded})
				default:
					add(templateChange{Container: name, Init: init, Field: "env." + envName, Operation: operationRemoved})
				}
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
			for _, field := range changedFields(previous, current, "name", "image", "resources", "imagePullPolicy", "ports", "env") {
				change(field, nil, nil)
			}
		}
		if sameNames && !equality.Semantic.DeepEqual(oldOrder, nextOrder) {
			field := "spec.containers.order"
			if init {
				field = "spec.initContainers.order"
			}
			add(templateChange{Field: field, Operation: operationModified})
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

func valueChange(container string, init bool, field string, before, after *string) templateChange {
	operation := operationModified
	if before == nil && after != nil {
		operation = operationAdded
	}
	if before != nil && after == nil {
		operation = operationRemoved
	}
	return templateChange{Container: container, Init: init, Field: field, Operation: operation, Before: before, After: after}
}

func optional(value string, present bool) *string {
	if !present {
		return nil
	}
	return &value
}

func portsString(ports []corev1.ContainerPort) string {
	items := make([]string, 0, len(ports))
	for _, port := range ports {
		protocol := port.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}
		items = append(items, strconv.Itoa(int(port.ContainerPort))+"/"+string(protocol))
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func envByName(vars []corev1.EnvVar) map[string]corev1.EnvVar {
	out := make(map[string]corev1.EnvVar, len(vars))
	for _, v := range vars {
		out[v.Name] = v
	}
	return out
}

func sortedKeys(a, b map[string]corev1.EnvVar) []string {
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

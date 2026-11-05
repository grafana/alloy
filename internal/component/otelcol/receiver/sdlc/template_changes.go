package sdlc

import (
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Values are allowlisted: configuration can contain credentials, including in
// annotations, commands, probes and environment variables. Only fields that
// can't carry secrets report before/after; environment variables report names.
var podTemplateRule = func() *diffRule {
	value := &diffRule{mode: diffValue}
	stringMap := &diffRule{each: value}
	resources := &diffRule{each: value}
	container := &diffRule{fields: map[string]*diffRule{
		"name":            {mode: diffIgnore},
		"image":           value,
		"imagePullPolicy": value,
		"ports":           {mode: diffValue, format: portsString},
		// Environment values may be credentials: report which variables
		// changed, never what they changed to.
		"env": {keyBy: "name", each: &diffRule{mode: diffOperation}},
		"resources": {fields: map[string]*diffRule{
			"requests": resources,
			"limits":   resources,
		}},
	}}
	return &diffRule{fields: map[string]*diffRule{
		"metadata": {fields: map[string]*diffRule{
			"labels": stringMap,
		}},
		"spec": {fields: map[string]*diffRule{
			"serviceAccountName": value,
			"nodeSelector":       stringMap,
			"containers":         {keyBy: "name", as: "containers", order: "spec.containers.order", each: container},
			"initContainers":     {keyBy: "name", as: "initContainers", order: "spec.initContainers.order", each: container},
		}},
	}}
}()

func templateChanges(before, after corev1.PodTemplateSpec) []fieldChange {
	return diffObjects(podTemplateRule, &before, &after)
}

func portsString(value any) (string, bool) {
	ports, _ := value.([]any)
	items := make([]string, 0, len(ports))
	for _, item := range ports {
		port, _ := item.(map[string]any)
		number, _ := port["containerPort"].(int64)
		protocol, _ := port["protocol"].(string)
		if protocol == "" {
			protocol = string(corev1.ProtocolTCP)
		}
		items = append(items, strconv.FormatInt(number, 10)+"/"+protocol)
	}
	sort.Strings(items)
	return strings.Join(items, ","), len(items) > 0
}

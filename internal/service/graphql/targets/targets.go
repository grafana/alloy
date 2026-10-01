package targets

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
)

type target interface {
	ForEachLabel(func(key string, value string) bool) bool
	HashLabelsWithPredicate(func(key string) bool) uint64
	NonMetaLabelsHash() uint64
}

func FromComponent(info *component.Info) []model.Target {
	if info == nil || info.Exports == nil {
		return nil
	}

	var discovered []target
	collectTargets(reflect.ValueOf(info.Exports), &discovered)

	result := make([]model.Target, 0, len(discovered))
	for _, target := range discovered {
		result = append(result, convertTarget(target))
	}
	return result
}

func collectTargets(value reflect.Value, targets *[]target) {
	if !value.IsValid() {
		return
	}

	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}

	if value.CanInterface() {
		if discoveredTarget, ok := value.Interface().(target); ok {
			*targets = append(*targets, discoveredTarget)
			return
		}
	}

	if value.CanAddr() && value.Addr().CanInterface() {
		if discoveredTarget, ok := value.Addr().Interface().(target); ok {
			*targets = append(*targets, discoveredTarget)
			return
		}
	}

	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.CanInterface() {
				collectTargets(field, targets)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			collectTargets(value.Index(i), targets)
		}
	}
}

func convertTarget(target target) model.Target {
	return model.Target{
		Labels:        labelPairs(target, false),
		NonMetaLabels: labelPairs(target, true),
		Hash: strconv.FormatUint(target.HashLabelsWithPredicate(func(string) bool {
			return true
		}), 10),
		NonMetaHash: strconv.FormatUint(target.NonMetaLabelsHash(), 10),
	}
}

func labelPairs(target target, nonMetaOnly bool) []model.LabelPair {
	var labels []model.LabelPair
	target.ForEachLabel(func(name string, value string) bool {
		if nonMetaOnly && strings.HasPrefix(name, "__meta_") {
			return true
		}
		labels = append(labels, model.LabelPair{Name: name, Value: value})
		return true
	})
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
	return labels
}

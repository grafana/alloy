package discovery

import (
	"fmt"
	"slices"
	"strings"

	commonlabels "github.com/prometheus/common/model"
	modellabels "github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/syntax"
)

type Target struct {
	// labels is of a Prometheus-native models.LabelSet type, because most of the time targets are used with
	// Prometheus codebase (even for logs, we have loki.relabel) and this representation helps reduce
	// unnecessary conversions and allocations. We can add another internal representations in the future if needed.
	labels commonlabels.LabelSet
}

var (
	_ syntax.Capsule = Target{}
)

func NewEmptyTarget() Target {
	return Target{}
}

// NewEmptyTargetWithSize creates an empty target, but allocates the allocSize of space for labels. These can be set
// using Set method.
func NewEmptyTargetWithSize(allocSize int) Target {
	return Target{
		labels: make(commonlabels.LabelSet, allocSize),
	}
}

func NewTargetFromLabelSet(targetLabels commonlabels.LabelSet) Target {
	return Target{
		labels: targetLabels,
	}
}

func NewTargetFromModelLabels(labels modellabels.Labels) Target {
	// TODO(thampiotr): save labels as cached value?
	l := make(commonlabels.LabelSet, len(labels))
	for _, v := range labels {
		l[commonlabels.LabelName(v.Name)] = commonlabels.LabelValue(v.Value)
	}
	return Target{
		labels: l,
	}
}

func NewTargetFromOwnAndSharedLabelSets(targetLabels commonlabels.LabelSet, sharedLabels commonlabels.LabelSet) Target {
	return Target{
		labels: targetLabels.Merge(sharedLabels),
	}
}

func NewTargetFromMap(m map[string]string) Target {
	l := make(commonlabels.LabelSet, len(m))
	for k, v := range m {
		l[commonlabels.LabelName(k)] = commonlabels.LabelValue(v)
	}
	return Target{
		labels: l,
	}
}

// TODO(thampiotr): unused?
func NewTargetsFromMaps(maps []map[string]string) []Target {
	targets := make([]Target, len(maps))
	for i, m := range maps {
		targets[i] = NewTargetFromMap(m)
	}
	return targets
}

// AlloyCapsule marks FastTarget as a capsule so Alloy syntax can marshal to or from it.
func (t Target) AlloyCapsule() {}

// ConvertInto is called by Alloy syntax to try convert Target to another type.
func (t Target) ConvertInto(dst interface{}) error {
	switch dst := dst.(type) {
	case *map[string]syntax.Value:
		result := make(map[string]syntax.Value, len(t.labels))
		for k, v := range t.labels {
			result[string(k)] = syntax.ValueFromString(string(v))
		}
		*dst = result
		return nil
		// TODO(thampiotr): Do we need to support other conversions?
	}
	return fmt.Errorf("MapCapsule: conversion to '%T' is not supported", dst)
}

// ConvertFrom is called by Alloy syntax to try convert from another type to Target.
func (t *Target) ConvertFrom(src interface{}) error {
	switch src := src.(type) {
	case map[string]syntax.Value:
		labelSet := make(commonlabels.LabelSet, len(src))
		for k, v := range src {
			if !v.IsString() {
				return fmt.Errorf("target::ConvertFrom: cannot convert non-string values to labels")
			}
			labelSet[commonlabels.LabelName(k)] = commonlabels.LabelValue(v.Text())
		}
		t.labels = labelSet
		return nil
		// TODO(thampiotr): Do we need to support other conversions?
	}

	return fmt.Errorf("target: conversion from '%T' is not supported", src)
}

// Equals should be called to compare two Target objects.
// TODO(thampiotr): make sure this is called when Alloy is deciding whether to propagate updates
func (t Target) Equals(other Target) bool {
	return t.labels.Equal(other.labels)
}

func (t Target) LabelSet() commonlabels.LabelSet {
	return t.labels
}

func (t Target) Labels() modellabels.Labels {
	// TODO(thampiotr): consider using base? cached one? or scratch builder?
	lb := modellabels.NewBuilder(nil)
	for k, v := range t.labels {
		lb.Set(string(k), string(v))
	}
	// TODO(thampiotr): verify this will be sorted!
	// TODO(thampiotr): We can cache this!
	return lb.Labels()
}

func (t Target) NonMetaLabels() modellabels.Labels {
	// TODO(thampiotr): consider using base? cached one? or scratch builder?
	lb := modellabels.NewBuilder(nil)
	for k, v := range t.labels {
		if !strings.HasPrefix(string(k), commonlabels.MetaLabelPrefix) {
			lb.Set(string(k), string(v))
		}
	}
	// TODO(thampiotr): verify this will be sorted!
	// TODO(thampiotr): We can cache this!
	return lb.Labels()
}

func (t Target) NonReservedLabelSet() commonlabels.LabelSet {
	// TODO(thampiotr): is there a more optimal way?
	result := make(commonlabels.LabelSet, len(t.labels))
	for k, v := range t.labels {
		if !strings.HasPrefix(string(k), commonlabels.ReservedLabelPrefix) {
			result[k] = v
		}
	}
	return result
}

func (t Target) SpecificLabels(lbls []string) modellabels.Labels {
	// TODO(thampiotr): consider using base? cached one? or scratch builder?
	lb := modellabels.NewBuilder(nil)
	for k, v := range t.labels {
		if slices.Contains(lbls, string(k)) {
			lb.Set(string(k), string(v))
		}
	}
	// TODO(thampiotr): verify this will be sorted!
	// TODO(thampiotr): We can cache this!
	return lb.Labels()
}

// ForEachLabel runs f over each key value pair in the Target. f must not modify Target while iterating. If f returns
// false, the iteration is interrupted. If f returns true, the iteration continues until the last element. ForEachLabel
// returns true if all the labels were iterated over or false if any call to f has interrupted the iteration.
func (t Target) ForEachLabel(f func(key string, value string) bool) bool {
	for k, v := range t.labels {
		if !f(string(k), string(v)) {
			// f has returned false, interrupt the iteration and return false.
			return false
		}
	}
	// We finished the iteration, return true.
	return true
}

// AsMap returns target's labels as a map of strings.
// Deprecated: this should not be used on any hot path as it leads to more allocation.
func (t Target) AsMap() map[string]string {
	ret := make(map[string]string, len(t.labels))
	for k, v := range t.labels {
		ret[string(k)] = string(v)
	}
	return ret
}

func (t Target) Get(key string) (string, bool) {
	value, ok := t.labels[commonlabels.LabelName(key)]
	return string(value), ok
}

func (t Target) Set(key, value string) {
	t.labels[commonlabels.LabelName(key)] = commonlabels.LabelValue(value)
}

func (t Target) Len() int {
	return len(t.labels)
}

func (t Target) Delete(key string) {
	// TODO(thampiotr): do we even need this method?
	delete(t.labels, commonlabels.LabelName(key))
}

func (t Target) Clone() Target {
	// TODO(thampiotr): Do we even need this method? Is this the best way to do it?
	return Target{
		labels: t.labels.Clone(),
	}
}

func (t Target) String() string {
	return fmt.Sprintf("%s", t.labels)
}

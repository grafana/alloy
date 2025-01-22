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

func NewEmptyTarget() Target {
	return Target{}
}

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
func (p Target) AlloyCapsule() {}

// ConvertInto is called by Alloy syntax to try convert FastTarget to another type.
func (p Target) ConvertInto(dst interface{}) error {
	switch dst := dst.(type) {
	case *map[string]syntax.Value:
		result := make(map[string]syntax.Value, len(p.labels))
		for k, v := range p.labels {
			result[string(k)] = syntax.ValueFromString(string(v))
		}
		*dst = result
		return nil
		// TODO(thampiotr): Do we need to support other conversions?
	}
	return fmt.Errorf("MapCapsule: conversion to '%T' is not supported", dst)
}

// ConvertFrom is called by Alloy syntax to try convert from another type to FastTarget.
func (p *Target) ConvertFrom(src interface{}) error {
	switch src := src.(type) {
	case map[string]syntax.Value:
		labelSet := make(commonlabels.LabelSet, len(src))
		for k, v := range src {
			if !v.IsString() {
				return fmt.Errorf("Target::ConvertFrom: cannot convert non-string values to labels")
			}
			labelSet[commonlabels.LabelName(k)] = commonlabels.LabelValue(v.Text())
		}
		p.labels = labelSet
		return nil
		// TODO(thampiotr): Do we need to support other conversions?
	}

	return fmt.Errorf("FastTarget: conversion from '%T' is not supported", src)
}

// Equals should be called to compare two FastTarget objects.
// TODO(thampiotr): make sure this is called when Alloy is deciding whether to propagate updates
func (p Target) Equals(other Target) bool {
	return p.labels.Equal(other.labels)
}

func (p Target) LabelSet() commonlabels.LabelSet {
	return p.labels
}

func (p Target) Labels() modellabels.Labels {
	// TODO(thampiotr): consider using base? cached one? or scratch builder?
	lb := modellabels.NewBuilder(nil)
	for k, v := range p.labels {
		lb.Set(string(k), string(v))
	}
	// TODO(thampiotr): verify this will be sorted!
	// TODO(thampiotr): We can cache this!
	return lb.Labels()
}

func (p Target) NonMetaLabels() modellabels.Labels {
	// TODO(thampiotr): consider using base? cached one? or scratch builder?
	lb := modellabels.NewBuilder(nil)
	for k, v := range p.labels {
		if !strings.HasPrefix(string(k), commonlabels.MetaLabelPrefix) {
			lb.Set(string(k), string(v))
		}
	}
	// TODO(thampiotr): verify this will be sorted!
	// TODO(thampiotr): We can cache this!
	return lb.Labels()
}

func (p Target) NonReservedLabelSet() commonlabels.LabelSet {
	// TODO(thampiotr): is there a more optimal way?
	result := make(commonlabels.LabelSet, len(p.labels))
	for k, v := range p.labels {
		if !strings.HasPrefix(string(k), commonlabels.ReservedLabelPrefix) {
			result[k] = v
		}
	}
	return result
}

func (p Target) SpecificLabels(lbls []string) modellabels.Labels {
	// TODO(thampiotr): consider using base? cached one? or scratch builder?
	lb := modellabels.NewBuilder(nil)
	for k, v := range p.labels {
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
func (p Target) ForEachLabel(f func(key string, value string) bool) bool {
	for k, v := range p.labels {
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
func (p Target) AsMap() map[string]string {
	ret := make(map[string]string, len(p.labels))
	for k, v := range p.labels {
		ret[string(k)] = string(v)
	}
	return ret
}

func (p Target) Get(key string) (string, bool) {
	value, ok := p.labels[commonlabels.LabelName(key)]
	return string(value), ok
}

func (p Target) Set(key, value string) {
	p.labels[commonlabels.LabelName(key)] = commonlabels.LabelValue(value)
}

func (p Target) Len() int {
	return len(p.labels)
}

func (p Target) Delete(key string) {
	// TODO(thampiotr): do we even need this method?
	delete(p.labels, commonlabels.LabelName(key))
}

func (p Target) Clone() Target {
	// TODO(thampiotr): Do we even need this method? Is this the best way to do it?
	return Target{
		labels: p.labels.Clone(),
	}
}

func (p Target) String() string {
	return fmt.Sprintf("%s", p.labels)
}

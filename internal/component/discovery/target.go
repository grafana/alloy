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
	// labelSet is of a Prometheus-native models.LabelSet type, because most of the time targets are used with
	// Prometheus codebase (even for logs, we have loki.relabel) and this representation helps reduce
	// unnecessary conversions and allocations. We can add another internal representations in the future if needed.
	labelSet commonlabels.LabelSet
	// NOTE: it is essential that equality between targets continues to work as it is used by Alloy runtime to
	// decide whether updates need to be propagated throughout the pipeline. See tests.
}

var (
	_ syntax.Capsule                = Target{}
	_ syntax.ConvertibleIntoCapsule = Target{}
	_ syntax.ConvertibleFromCapsule = &Target{}
)

func NewEmptyTarget() Target {
	return NewTargetFromLabelSet(make(commonlabels.LabelSet))
}

// NewEmptyTargetWithSize creates an empty target, but allocates the allocSize of space for labels. These can be set
// using Set method.
func NewEmptyTargetWithSize(allocSize int) Target {
	return NewTargetFromLabelSet(make(commonlabels.LabelSet, allocSize))
}

func NewTargetFromLabelSet(targetLabels commonlabels.LabelSet) Target {
	return Target{
		labelSet: targetLabels,
	}
}

// TODO(thampiotr): 27% allocs
// TODO(thampiotr): discovery.*
func NewTargetFromSpecificAndBaseLabelSet(specific, base commonlabels.LabelSet) Target {
	merged := make(commonlabels.LabelSet, len(specific)+len(base))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range specific {
		merged[k] = v
	}
	return NewTargetFromLabelSet(merged)
}

// TODO(thampiotr): 27% allocs
// TODO(thampiotr): discovery.relabel
func NewTargetFromModelLabels(labels modellabels.Labels) Target {
	// TODO(thampiotr): save labels as cached value?
	l := make(commonlabels.LabelSet, len(labels))
	for _, v := range labels {
		l[commonlabels.LabelName(v.Name)] = commonlabels.LabelValue(v.Value)
	}
	return Target{
		labelSet: l,
	}
}

func NewTargetFromMap(m map[string]string) Target {
	l := make(commonlabels.LabelSet, len(m))
	for k, v := range m {
		l[commonlabels.LabelName(k)] = commonlabels.LabelValue(v)
	}
	return NewTargetFromLabelSet(l)
}

// TODO(thampiotr): 13% allocs
// TODO(thampiotr): discovery.relabel
func (t Target) Labels() modellabels.Labels {
	// TODO(thampiotr): We can cache this!
	return t.LabelsWithPredicate(nil)
}

// TODO(thampiotr): 13% allocs
// TODO(thampiotr): prometheus.scrape / distributed_targets
func (t Target) NonMetaLabels() modellabels.Labels {
	return t.LabelsWithPredicate(func(key string) bool {
		return !strings.HasPrefix(key, commonlabels.MetaLabelPrefix)
	})
}

// TODO(thampiotr): loki.source.kubernetes / distributed_targets
func (t Target) SpecificLabels(lbls []string) modellabels.Labels {
	// TODO(thampiotr): We can cache this!
	return t.LabelsWithPredicate(func(key string) bool {
		return slices.Contains(lbls, key)
	})
}

func (t Target) NonReservedLabelSet() commonlabels.LabelSet {
	// This may not be the most optimal way, but this method is NOT a known hot spot at the time of this comment.
	result := make(commonlabels.LabelSet, t.Len())
	t.ForEachLabel(func(key string, value string) bool {
		if !strings.HasPrefix(key, commonlabels.ReservedLabelPrefix) {
			result[commonlabels.LabelName(key)] = commonlabels.LabelValue(value)
		}
		return true
	})
	return result
}

func (t Target) LabelsWithPredicate(pred func(key string) bool) modellabels.Labels {
	// This method allocates less than Builder or ScratchBuilder, as proven by benchmarks.
	lb := make([]modellabels.Label, 0, t.Len())
	t.ForEachLabel(func(key string, value string) bool {
		if pred == nil || pred(key) {
			lb = append(lb, modellabels.Label{
				Name:  key,
				Value: value,
			})
		}
		return true
	})
	slices.SortFunc(lb, func(a, b modellabels.Label) int { return strings.Compare(a.Name, b.Name) })
	return lb
}

// ForEachLabel runs f over each key value pair in the Target. f must not modify Target while iterating. If f returns
// false, the iteration is interrupted. If f returns true, the iteration continues until the last element. ForEachLabel
// returns true if all the labels were iterated over or false if any call to f has interrupted the iteration.
func (t Target) ForEachLabel(f func(key string, value string) bool) bool {
	for k, v := range t.labelSet {
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
	ret := make(map[string]string, t.Len())
	t.ForEachLabel(func(key string, value string) bool {
		ret[key] = value
		return true
	})
	return ret
}

func (t Target) Get(key string) (string, bool) {
	value, ok := t.labelSet[commonlabels.LabelName(key)]
	return string(value), ok
}

func (t *Target) Set(key, value string) {
	if t.labelSet == nil {
		t.labelSet = make(commonlabels.LabelSet, 1)
	}
	t.labelSet[commonlabels.LabelName(key)] = commonlabels.LabelValue(value)
}

func (t Target) Delete(key string) {
	delete(t.labelSet, commonlabels.LabelName(key))
}

func (t Target) LabelSet() commonlabels.LabelSet {
	return t.labelSet
}

func (t Target) Len() int {
	return len(t.labelSet)
}

func (t Target) Clone() Target {
	return Target{
		labelSet: t.labelSet.Clone(),
	}
}

// AlloyCapsule marks FastTarget as a capsule so Alloy syntax can marshal to or from it.
func (t Target) AlloyCapsule() {}

// ConvertInto is called by Alloy syntax to try converting Target to another type.
func (t Target) ConvertInto(dst interface{}) error {
	switch dst := dst.(type) {
	case *map[string]syntax.Value:
		result := make(map[string]syntax.Value, t.Len())
		t.ForEachLabel(func(key string, value string) bool {
			result[key] = syntax.ValueFromString(value)
			return true
		})
		*dst = result
		return nil
	}
	return fmt.Errorf("target::ConvertInto: conversion to '%T' is not supported", dst)
}

// ConvertFrom is called by Alloy syntax to try converting from another type to Target.
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
		*t = NewTargetFromLabelSet(labelSet)
		return nil
	}
	return fmt.Errorf("target: conversion from '%T' is not supported", src)
}

func (t Target) String() string {
	return fmt.Sprintf("%s", t.labelSet)
}

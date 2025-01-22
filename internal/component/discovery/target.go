package discovery

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cespare/xxhash/v2"
	commonlabels "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	modellabels "github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/syntax"
)

type Target struct {
	group commonlabels.LabelSet
	own   commonlabels.LabelSet
	size  int
}

var (
	seps = []byte{'\xff'}

	_ syntax.Capsule                = Target{}
	_ syntax.ConvertibleIntoCapsule = Target{}
	_ syntax.ConvertibleFromCapsule = &Target{}
	_ equality.CustomEquality       = Target{}
)

func ComponentTargetsToPromTargetGroups(jobName string, tgs []Target) map[string][]*targetgroup.Group {
	targetsWithCommonGroupLabels := map[uint64][]Target{}
	for _, t := range tgs {
		fp := t.groupLabelsHash() // TODO(thampiotr): could use a cache if it's on exactly the same slice
		targetsWithCommonGroupLabels[fp] = append(targetsWithCommonGroupLabels[fp], t)
	}

	allGroups := make([]*targetgroup.Group, 0, len(targetsWithCommonGroupLabels))

	groupIndex := 0
	for _, targetsInGroup := range targetsWithCommonGroupLabels {
		sharedLabels := targetsInGroup[0].group // all have the same group labels.
		individualLabels := make([]commonlabels.LabelSet, len(targetsInGroup))
		for i, target := range targetsInGroup {
			individualLabels[i] = target.own
		}
		promGroup := &targetgroup.Group{
			Source:  fmt.Sprintf("%s_part_%d", jobName, groupIndex),
			Labels:  sharedLabels,
			Targets: individualLabels,
		}
		allGroups = append(allGroups, promGroup)
	}
	return map[string][]*targetgroup.Group{jobName: allGroups}
}

var EmptyTarget = Target{
	group: commonlabels.LabelSet{},
	own:   commonlabels.LabelSet{},
	size:  0,
}

func NewTargetFromLabelSet(ls commonlabels.LabelSet) Target {
	return NewTargetFromSpecificAndBaseLabelSet(ls, nil)
}

func NewTargetFromSpecificAndBaseLabelSet(own, group commonlabels.LabelSet) Target {
	if group == nil {
		group = commonlabels.LabelSet{}
	}
	if own == nil {
		own = commonlabels.LabelSet{}
	}
	ret := Target{
		group: group,
		own:   own,
	}
	size := 0
	ret.ForEachLabel(func(key string, value string) bool {
		size++
		return true
	})
	ret.size = size
	return ret
}

// NewTargetFromModelLabels creates a target from model Labels.
// NOTE: this is not optimised and should be avoided on a hot path.
func NewTargetFromModelLabels(labels modellabels.Labels) Target {
	l := make(commonlabels.LabelSet, len(labels))
	for _, label := range labels {
		l[commonlabels.LabelName(label.Name)] = commonlabels.LabelValue(label.Value)
	}
	return NewTargetFromLabelSet(l)
}

func NewTargetFromMap(m map[string]string) Target {
	l := make(commonlabels.LabelSet, len(m))
	for k, v := range m {
		l[commonlabels.LabelName(k)] = commonlabels.LabelValue(v)
	}
	return NewTargetFromLabelSet(l)
}

// Labels converts this target into prometheus/prometheus/model/labels.Labels. It is not efficient and should be
// avoided on a hot path.
func (t Target) Labels() modellabels.Labels {
	// This method allocates less than Builder or ScratchBuilder, as proven by benchmarks.
	lb := make([]modellabels.Label, 0, t.Len())
	t.ForEachLabel(func(key string, value string) bool {
		lb = append(lb, modellabels.Label{
			Name:  key,
			Value: value,
		})
		return true
	})
	slices.SortFunc(lb, func(a, b modellabels.Label) int { return strings.Compare(a.Name, b.Name) })
	return lb
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

// ForEachLabel runs f over each key value pair in the Target. f must not modify Target while iterating. If f returns
// false, the iteration is interrupted. If f returns true, the iteration continues until the last element. ForEachLabel
// returns true if all the labels were iterated over or false if any call to f has interrupted the iteration.
func (t Target) ForEachLabel(f func(key string, value string) bool) bool {
	for k, v := range t.own {
		if !f(string(k), string(v)) {
			// f has returned false, interrupt the iteration and return false.
			return false
		}
	}
	// Now go over the group ones only if they were not part of own labels
	for k, v := range t.group {
		if _, ok := t.own[k]; ok {
			continue
		}
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
	lv, ok := t.own[commonlabels.LabelName(key)]
	if ok {
		return string(lv), ok
	}
	lv, ok = t.group[commonlabels.LabelName(key)]
	return string(lv), ok
}

// LabelSet converts this target in to a LabelSet
// Deprecated: this is not optimised and should be avoided if possible.
func (t Target) LabelSet() commonlabels.LabelSet {
	merged := make(commonlabels.LabelSet, t.Len())
	for k, v := range t.group {
		merged[k] = v
	}
	for k, v := range t.own {
		merged[k] = v
	}
	return merged
}

func (t Target) Len() int {
	return t.size
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
	s := make([]string, 0, t.Len())
	t.ForEachLabel(func(key string, value string) bool {
		s = append(s, fmt.Sprintf("%q=%q", key, value))
		return true
	})
	slices.Sort(s)
	return fmt.Sprintf("{%s}", strings.Join(s, ", "))
}

func (t Target) Equals(other any) bool {
	if ot, ok := other.(Target); ok {
		return t.EqualsTarget(ot)
	}
	return false
}

func (t Target) EqualsTarget(other Target) bool {
	if t.Len() != other.Len() {
		return false
	}
	finished := t.ForEachLabel(func(key string, value string) bool {
		otherValue, ok := other.Get(key)
		if !ok || otherValue != value {
			return false
		}
		return true
	})
	return finished
}

func (t Target) NonMetaLabelsHash() uint64 {
	return t.HashLabelsWithPredicate(func(key string) bool {
		return !strings.HasPrefix(key, commonlabels.MetaLabelPrefix)
	})
}

func (t Target) SpecificLabelsHash(labelNames []string) uint64 {
	return t.HashLabelsWithPredicate(func(key string) bool {
		return slices.Contains(labelNames, key)
	})
}

func (t Target) HashLabelsWithPredicate(pred func(key string) bool) uint64 {
	// For hash to be deterministic, we need labels order to be deterministic too. Figure this out first.
	labelsInOrder := make([]string, 0, t.Len()) // TODO(thampiotr): this can go to object pool?
	t.ForEachLabel(func(key string, value string) bool {
		if pred(value) {
			labelsInOrder = append(labelsInOrder, key)
		}
		return true
	})
	slices.Sort(labelsInOrder)
	return t.hashLabelsInOrder(labelsInOrder)
}

func (t Target) groupLabelsHash() uint64 {
	// For hash to be deterministic, we need labels order to be deterministic too. Figure this out first.
	// TODO(thampiotr): We could cache the hash somewhere if it is called often on the same data.
	labelsInOrder := make([]string, 0, len(t.group)) // TODO(thampiotr): this can go to object pool?
	for name := range t.group {
		labelsInOrder = append(labelsInOrder, string(name))
	}
	slices.Sort(labelsInOrder)
	return t.hashLabelsInOrder(labelsInOrder)
}

func (t Target) hashLabelsInOrder(order []string) uint64 {
	// This optimisation is adapted from prometheus/model/labels.
	// Use xxhash.Sum64(b) for fast path as it's faster.
	b := make([]byte, 0, 1024)
	mustGet := func(label string) string {
		val, ok := t.Get(label)
		if !ok {
			panic("label concurrently modified - this is a bug - please report an issue")
		}
		return val
	}

	for i, key := range order {
		value := mustGet(key)
		if len(b)+len(key)+len(value)+2 >= cap(b) {
			// If labels entry is 1KB+ do not allocate whole entry.
			h := xxhash.New()
			_, _ = h.Write(b)
			for _, key := range order[i:] {
				_, _ = h.WriteString(key)
				_, _ = h.Write(seps)
				_, _ = h.WriteString(mustGet(key))
				_, _ = h.Write(seps)
			}
			return h.Sum64()
		}

		b = append(b, key...)
		b = append(b, seps[0])
		b = append(b, value...)
		b = append(b, seps[0])
	}
	return xxhash.Sum64(b)
}

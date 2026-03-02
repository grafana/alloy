package discovery

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
	commonlabels "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	modellabels "github.com/prometheus/prometheus/model/labels"
	"golang.org/x/exp/maps"

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
	// used in tests to simulate hash conflicts
	labelSetEqualsFn  = func(l1, l2 commonlabels.LabelSet) bool { return &l1 == &l2 || l1.Equal(l2) }
	stringSlicesPool  = sync.Pool{New: func() any { return make([]string, 0, 20) }}
	borrowLabelsSlice = func() []string {
		return stringSlicesPool.Get().([]string)
	}
	releaseLabelsSlice = func(labels []string) {
		// We can ignore linter warning here, because slice headers are small and the underlying array will be reused.
		stringSlicesPool.Put(labels[:0]) //nolint:staticcheck // SA6002
	}

	_ syntax.Capsule                = Target{}
	_ syntax.ConvertibleIntoCapsule = Target{}
	_ syntax.ConvertibleFromCapsule = &Target{}
	_ equality.CustomEquality       = Target{}
)

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
	l := make(commonlabels.LabelSet, labels.Len())
	labels.Range(func(label modellabels.Label) {
		l[commonlabels.LabelName(label.Name)] = commonlabels.LabelValue(label.Value)
	})
	return NewTargetFromLabelSet(l)
}

func NewTargetFromMap(m map[string]string) Target {
	l := make(commonlabels.LabelSet, len(m))
	for k, v := range m {
		l[commonlabels.LabelName(k)] = commonlabels.LabelValue(v)
	}
	return NewTargetFromLabelSet(l)
}

// PromLabels converts this target into prometheus/prometheus/model/labels.Labels. It is not efficient and should be
// avoided on a hot path.
func (t Target) PromLabels() modellabels.Labels {
	builder := modellabels.NewScratchBuilder(t.Len())
	t.ForEachLabel(func(key string, value string) bool {
		builder.Add(key, value)
		return true
	})
	builder.Sort()
	return builder.Labels()
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
// ForEachLabel does not guarantee iteration order or sort labels in any way.
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
func (t Target) ConvertInto(dst any) error {
	switch dst := dst.(type) {
	case *map[string]syntax.Value:
		result := make(map[string]syntax.Value, t.Len())
		// NOTE: no need to sort as value_tokens.go in syntax/token/builder package sorts the map's keys.
		t.ForEachLabel(func(key string, value string) bool {
			result[key] = syntax.ValueFromString(value)
			return true
		})
		*dst = result
		return nil
	case *map[string]string:
		result := make(map[string]string, t.Len())
		// NOTE: no need to sort as value_tokens.go in syntax/token/builder package sorts the map's keys.
		t.ForEachLabel(func(key string, value string) bool {
			result[key] = value
			return true
		})
		*dst = result
		return nil
	}

	return fmt.Errorf("target::ConvertInto: conversion to '%T' is not supported", dst)
}

// ConvertFrom is called by Alloy syntax to try converting from another type to Target.
func (t *Target) ConvertFrom(src any) error {
	switch src := src.(type) {
	case map[string]syntax.Value:
		labelSet := make(commonlabels.LabelSet, len(src))
		for k, v := range src {
			var strValue string
			switch {
			case v.IsString():
				strValue = v.Text()
			case v.Reflect().CanInterface():
				strValue = fmt.Sprintf("%v", v.Reflect().Interface())
			default:
				return fmt.Errorf("target::ConvertFrom: cannot convert value that can't be interfaced to (e.g. unexported struct field)")
			}
			labelSet[commonlabels.LabelName(k)] = commonlabels.LabelValue(strValue)
		}
		*t = NewTargetFromLabelSet(labelSet)
		return nil
	default: // handle all other types of maps via reflection as Go generics don't support generics in switch/case.
		rv := reflect.ValueOf(src)
		switch rv.Kind() {
		case reflect.Map:
			labelSet := make(commonlabels.LabelSet, rv.Len())
			for _, key := range rv.MapKeys() {
				value := rv.MapIndex(key)
				if !value.CanInterface() || !key.CanInterface() {
					return fmt.Errorf("target::ConvertFrom: conversion from '%T' is not supported", src)
				}
				labelSet[commonlabels.LabelName(fmt.Sprintf("%v", key.Interface()))] = commonlabels.LabelValue(fmt.Sprintf("%v", value.Interface()))
			}
			*t = NewTargetFromLabelSet(labelSet)
			return nil
		default:
			return fmt.Errorf("target::ConvertFrom: conversion from '%T' is not supported", src)
		}
	}
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

// Equals implements equality.CustomEquality. Works only with pointers.
func (t Target) Equals(other any) bool {
	if ot, ok := other.(*Target); ok {
		return t.EqualsTarget(ot)
	}
	return false
}

func (t Target) EqualsTarget(other *Target) bool {
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
	labelsInOrder := borrowLabelsSlice()
	defer releaseLabelsSlice(labelsInOrder)
	t.ForEachLabel(func(key string, value string) bool {
		if pred(key) {
			labelsInOrder = append(labelsInOrder, key)
		}
		return true
	})
	slices.Sort(labelsInOrder)
	return t.hashLabelsInOrder(labelsInOrder)
}

func (t Target) groupLabelsHash() uint64 {
	// For hash to be deterministic, we need labels order to be deterministic too. Figure this out first.
	labelsInOrder := borrowLabelsSlice()
	defer releaseLabelsSlice(labelsInOrder)

	for name := range t.group {
		labelsInOrder = append(labelsInOrder, string(name))
	}
	slices.Sort(labelsInOrder)
	return t.hashLabelsInOrder(labelsInOrder)
}

// NOTE 1: This function is copied from Prometheus codebase (labels.StableHash()) and adapted to work correctly with Alloy types.
// NOTE 2: It is important to keep the hashing function consistent between Alloy versions in order to have smooth clustering
// rollouts without duplicated or missing scraping of targets. There are tests to verify this behaviour. Do not change it.
func (t Target) hashLabelsInOrder(order []string) uint64 {
	// This optimisation is adapted from prometheus/model/labels.
	// Use xxhash.Sum64(b) for fast path as it's faster.
	b := make([]byte, 0, 1024)
	mustGet := func(label string) string {
		val, _ := t.Get(label)
		// if val is not found it would mean there is a bug and Target is no longer immutable. But we can still provide
		// a consistent hashing behaviour by returning empty string we got from Get.
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

func ComponentTargetsToPromTargetGroups(jobName string, tgs []Target) map[string][]*targetgroup.Group {
	allGroups := ComponentTargetsToPromTargetGroupsForSingleJob(jobName, tgs)

	return map[string][]*targetgroup.Group{jobName: allGroups}
}

func ComponentTargetsToPromTargetGroupsForSingleJob(jobName string, tgs []Target) []*targetgroup.Group {
	targetIndWithCommonGroupLabels := map[uint64][]int{} // target group hash --> index of target in tgs array
	for ind, t := range tgs {
		fp := t.groupLabelsHash()
		targetIndWithCommonGroupLabels[fp] = append(targetIndWithCommonGroupLabels[fp], ind)
	}

	// Sort by hash to get deterministic order
	sortedKeys := maps.Keys(targetIndWithCommonGroupLabels)
	slices.Sort(sortedKeys)

	allGroups := make([]*targetgroup.Group, 0, len(targetIndWithCommonGroupLabels))
	var hashConflicts []commonlabels.LabelSet
	for _, hash := range sortedKeys {
		// targetIndices = indices of all the targets that have the same group labels hash
		targetIndices := targetIndWithCommonGroupLabels[hash]
		// since we grouped them by their group labels hash, their group labels should all be the same (except for hash collision handled below)
		sharedLabels := tgs[targetIndices[0]].group
		individualLabels := make([]commonlabels.LabelSet, 0, len(targetIndices))
		for _, ind := range targetIndices {
			target := tgs[ind]
			// detect hash collisions - we'll append them separately - it's still correct, just may be less efficient
			if !labelSetEqualsFn(sharedLabels, target.group) {
				hashConflicts = append(hashConflicts, target.LabelSet())
				continue
			}
			individualLabels = append(individualLabels, target.own)
		}

		if len(individualLabels) != 0 {
			allGroups = append(allGroups, &targetgroup.Group{
				Source:  fmt.Sprintf("%s_part_%v", jobName, hash),
				Labels:  sharedLabels,
				Targets: individualLabels,
			})
		}
	}

	if len(hashConflicts) > 0 { // these are consolidated already, no common group labels here.
		allGroups = append(allGroups, &targetgroup.Group{
			Source:  fmt.Sprintf("%s_rest", jobName),
			Targets: hashConflicts,
		})
	}
	return allGroups
}

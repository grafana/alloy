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

	"github.com/grafana/alloy/internal/component/discovery/internal/labelpack"
	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/syntax"
)

// groupLabels holds the labels that a Target shares with all the other targets
// that came from the same service discovery target group. It is referenced by
// pointer from every such Target, so the packed labels are stored once per group
// rather than once per target.
//
// A nil *groupLabels means "no group labels" and all its methods are safe to
// call on a nil receiver.
type groupLabels struct {
	packed labelpack.Labels
	n      int

	// hash is the hash of packed on its own, computed eagerly because a group is
	// shared by many targets and each of them needs it in
	// ComponentTargetsToPromTargetGroupsForSingleJob.
	hash uint64

	// ls is the LabelSet view of packed, materialised on first use. It is only
	// needed when converting back into Prometheus target groups, so most groups
	// never build it.
	lsOnce sync.Once
	ls     commonlabels.LabelSet
}

// newGroupLabels packs ls into a groupLabels. It returns nil if ls holds no
// labels that survive normalisation, so that targets without group labels do not
// pay for an allocation.
func newGroupLabels(ls commonlabels.LabelSet) *groupLabels {
	if len(ls) == 0 {
		return nil
	}
	packed, n := labelpack.FromLabelSet(ls)
	return newGroupLabelsFromPacked(packed, n)
}

func newGroupLabelsFromPacked(packed labelpack.Labels, n int) *groupLabels {
	if n == 0 {
		return nil
	}
	return &groupLabels{
		packed: packed,
		n:      n,
		hash:   hashLabels(packed, labelpack.Empty, nil),
	}
}

func (g *groupLabels) labels() labelpack.Labels {
	if g == nil {
		return labelpack.Empty
	}
	return g.packed
}

func (g *groupLabels) len() int {
	if g == nil {
		return 0
	}
	return g.n
}

// labelSet returns the group labels as a LabelSet. The result is cached, and
// must not be mutated by callers.
func (g *groupLabels) labelSet() commonlabels.LabelSet {
	if g == nil {
		return commonlabels.LabelSet{}
	}
	g.lsOnce.Do(func() {
		g.ls = make(commonlabels.LabelSet, g.n)
		g.packed.Range(func(name, value string) bool {
			g.ls[commonlabels.LabelName(name)] = commonlabels.LabelValue(value)
			return true
		})
	})
	return g.ls
}

// withoutEmptyIn returns g with every label removed whose name is mapped to an
// empty value in own. Setting a label to an empty value means deleting it, so
// such a group label must not shine through. g is returned unchanged when there
// is nothing to remove, which is the overwhelmingly common case and keeps the
// group shared between targets.
func (g *groupLabels) withoutEmptyIn(own commonlabels.LabelSet) *groupLabels {
	if g == nil || len(own) == 0 {
		return g
	}

	// Cheap scan first: only build a derived group if a deletion actually
	// applies.
	tombstoned := false
	for name, value := range own {
		if value == "" && g.packed.Has(string(name)) {
			tombstoned = true
			break
		}
	}
	if !tombstoned {
		return g
	}

	pairs := make([]labelpack.Pair, 0, g.n)
	g.packed.Range(func(name, value string) bool {
		if ownValue, ok := own[commonlabels.LabelName(name)]; ok && ownValue == "" {
			return true
		}
		pairs = append(pairs, labelpack.Pair{Name: name, Value: value})
		return true
	})
	return newGroupLabelsFromPacked(labelpack.FromPairs(pairs))
}

// Target is an immutable set of labels describing something to collect from.
//
// Labels are stored in two packed, name-sorted string buffers: group holds the
// labels shared with every other target from the same service discovery target
// group, and own holds the labels specific to this target. A label present in
// own shadows the label of the same name in group.
//
// Labels with an empty name or an empty value are dropped at construction. An
// empty value is indistinguishable from an absent label in this representation,
// which matches how Prometheus and TargetBuilder already behave.
type Target struct {
	// Keep Target non-comparable. Comparing two Targets with == would compile
	// once the label storage is a pointer and a string, but it would be wrong:
	// two Targets holding identical labels can have different group pointers.
	// Callers must use EqualsTarget instead. This field must stay first, so that
	// it does not add trailing padding to the struct.
	_ [0]func()

	group *groupLabels
	own   labelpack.Labels
	size  int32
}

var (
	seps = []byte{'\xff'}
	// used in tests to simulate hash conflicts
	labelSetEqualsFn = func(l1, l2 commonlabels.LabelSet) bool { return &l1 == &l2 || l1.Equal(l2) }

	_ syntax.Capsule                = Target{}
	_ syntax.ConvertibleIntoCapsule = Target{}
	_ syntax.ConvertibleFromCapsule = &Target{}
	_ equality.CustomEquality       = Target{}
)

// EmptyTarget is a Target with no labels. It is equal to the zero value.
var EmptyTarget = Target{}

func NewTargetFromLabelSet(ls commonlabels.LabelSet) Target {
	return NewTargetFromSpecificAndBaseLabelSet(ls, nil)
}

func NewTargetFromSpecificAndBaseLabelSet(own, group commonlabels.LabelSet) Target {
	return newTargetFromGroup(newGroupLabels(group), own)
}

// newTargetFromGroup builds a Target from an already packed group. Callers that
// create many targets sharing the same group labels should pack the group once
// and reuse it here, so that the group labels are stored once rather than once
// per target.
func newTargetFromGroup(group *groupLabels, own commonlabels.LabelSet) Target {
	ownPacked, _ := labelpack.FromLabelSet(own)
	// An empty value in own deletes the label, including one inherited from the
	// group, so the group may need narrowing for this target.
	group = group.withoutEmptyIn(own)
	return newTarget(group, ownPacked)
}

func newTarget(group *groupLabels, own labelpack.Labels) Target {
	return Target{
		group: group,
		own:   own,
		size:  int32(labelpack.MergedLen(group.labels(), own)),
	}
}

// NewTargetFromModelLabels creates a target from model Labels.
func NewTargetFromModelLabels(labels modellabels.Labels) Target {
	pairs := make([]labelpack.Pair, 0, labels.Len())
	labels.Range(func(label modellabels.Label) {
		pairs = append(pairs, labelpack.Pair{Name: label.Name, Value: label.Value})
	})
	packed, n := labelpack.FromPairs(pairs)
	return Target{own: packed, size: int32(n)}
}

func NewTargetFromMap(m map[string]string) Target {
	packed, n := labelpack.FromMap(m)
	return Target{own: packed, size: int32(n)}
}

// PromLabels converts this target into prometheus/prometheus/model/labels.Labels.
func (t Target) PromLabels() modellabels.Labels {
	builder := modellabels.NewScratchBuilder(t.Len())
	// Labels are already in ascending name order, so there is no need to sort.
	t.ForEachLabel(func(key string, value string) bool {
		builder.Add(key, value)
		return true
	})
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
	return labelpack.RangeMerged(t.group.labels(), t.own, f)
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
	if value, ok := t.own.Get(key); ok {
		return value, true
	}
	return t.group.labels().Get(key)
}

// LabelSet converts this target in to a LabelSet
// Deprecated: this is not optimised and should be avoided if possible.
func (t Target) LabelSet() commonlabels.LabelSet {
	merged := make(commonlabels.LabelSet, t.Len())
	t.ForEachLabel(func(key string, value string) bool {
		merged[commonlabels.LabelName(key)] = commonlabels.LabelValue(value)
		return true
	})
	return merged
}

// ownLabelSet returns only the labels specific to this target, excluding the
// ones inherited from its group.
func (t Target) ownLabelSet() commonlabels.LabelSet {
	own := make(commonlabels.LabelSet, t.Len()-t.group.len())
	t.own.Range(func(name, value string) bool {
		own[commonlabels.LabelName(name)] = commonlabels.LabelValue(value)
		return true
	})
	return own
}

func (t Target) Len() int {
	return int(t.size)
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
		pairs := make([]labelpack.Pair, 0, len(src))
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
			pairs = append(pairs, labelpack.Pair{Name: k, Value: strValue})
		}
		packed, n := labelpack.FromPairs(pairs)
		*t = Target{own: packed, size: int32(n)}
		return nil
	default: // handle all other types of maps via reflection as Go generics don't support generics in switch/case.
		rv := reflect.ValueOf(src)
		switch rv.Kind() {
		case reflect.Map:
			pairs := make([]labelpack.Pair, 0, rv.Len())
			for _, key := range rv.MapKeys() {
				value := rv.MapIndex(key)
				if !value.CanInterface() || !key.CanInterface() {
					return fmt.Errorf("target::ConvertFrom: conversion from '%T' is not supported", src)
				}
				pairs = append(pairs, labelpack.Pair{
					Name:  fmt.Sprintf("%v", key.Interface()),
					Value: fmt.Sprintf("%v", value.Interface()),
				})
			}
			packed, n := labelpack.FromPairs(pairs)
			*t = Target{own: packed, size: int32(n)}
			return nil
		default:
			return fmt.Errorf("target::ConvertFrom: conversion from '%T' is not supported", src)
		}
	}
}

func (t Target) String() string {
	// Labels are already in ascending name order, so there is no need to sort.
	sb := strings.Builder{}
	sb.WriteString("{")
	first := true
	t.ForEachLabel(func(key string, value string) bool {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&sb, "%q=%q", key, value)
		return true
	})
	sb.WriteString("}")
	return sb.String()
}

// Equals implements equality.CustomEquality. Works only with pointers.
func (t Target) Equals(other any) bool {
	if ot, ok := other.(*Target); ok {
		return t.EqualsTarget(ot)
	}
	return false
}

func (t Target) EqualsTarget(other *Target) bool {
	// Fast path: targets that share a group pointer and have identical own labels
	// are equal without decoding anything. This is the common case when checking
	// whether a component's exports changed, since a relabel step that changes
	// nothing reuses both the group pointer and the own buffer.
	if t.group == other.group && t.own == other.own {
		return true
	}
	if t.size != other.size {
		return false
	}
	// Otherwise walk both merged views in lockstep. Both are sorted by name, so
	// this is linear rather than a lookup per label.
	return labelpack.EqualMerged(t.group.labels(), t.own, other.group.labels(), other.own)
}

// TargetCacheKey identifies the labels of a Target and can be used as a map key.
// It is only meaningful as an opaque key: use Target.EqualsTarget to compare
// targets for equality.
//
// Two Targets with equal keys always have the same labels, which is what makes
// the key safe to memoise a per-target computation on. The converse does not
// hold: two Targets with the same labels split differently between their group
// and own buffers have different keys, so a cache keyed on this may miss where a
// label comparison would have matched. A miss only costs recomputation.
type TargetCacheKey struct {
	group *groupLabels
	own   labelpack.Labels
}

// CacheKey returns a comparable key identifying this target's labels, for use as
// a map key when memoising work per target.
//
// This is cheap: the group is compared by pointer and the own labels by their
// already-packed string, so no labels are decoded and nothing is allocated.
// Targets discovered together share a group pointer, and a target that came
// through service discovery or relabeling unchanged keeps the same own buffer, so
// unchanged targets produce equal keys.
func (t Target) CacheKey() TargetCacheKey {
	return TargetCacheKey{group: t.group, own: t.own}
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
	return hashLabels(t.group.labels(), t.own, pred)
}

// groupLabelsHash hashes only the labels this target inherits from its group,
// but resolves their values through the target, so that a label overridden in
// own changes the hash. Targets with the same group labels hash can be sent to
// Prometheus as a single target group with shared labels.
func (t Target) groupLabelsHash() uint64 {
	group := t.group.labels()
	if group.IsEmpty() {
		return hashLabels(labelpack.Empty, labelpack.Empty, nil)
	}
	// When own cannot override any group label, the hash is just the group's own
	// hash, which was computed once when the group was packed.
	if !labelpack.Overlaps(group, t.own) {
		return t.group.hash
	}
	// Hash the group's names, taking values from own where it overrides them.
	return hashShadowedLabels(group, t.own)
}

// NOTE 1: This function is copied from Prometheus codebase (labels.StableHash()) and adapted to work correctly with Alloy types.
// NOTE 2: It is important to keep the hashing function consistent between Alloy versions in order to have smooth clustering
// rollouts without duplicated or missing scraping of targets. There are tests to verify this behaviour. Do not change it.
//
// hashLabels hashes the merged group and own labels, in ascending name order,
// keeping only the labels for which pred returns true. A nil pred keeps all of
// them.
func hashLabels(group, own labelpack.Labels, pred func(key string) bool) uint64 {
	// This optimisation is adapted from prometheus/model/labels.
	// Use xxhash.Sum64(b) for fast path as it's faster.
	b := make([]byte, 0, 1024)
	var h *xxhash.Digest

	labelpack.RangeMerged(group, own, func(key, value string) bool {
		if pred != nil && !pred(key) {
			return true
		}
		if h == nil && len(b)+len(key)+len(value)+2 >= cap(b) {
			// If labels entry is 1KB+, switch to the streaming API and copy in
			// everything hashed so far.
			h = xxhash.New()
			_, _ = h.Write(b)
		}
		if h != nil {
			_, _ = h.WriteString(key)
			_, _ = h.Write(seps)
			_, _ = h.WriteString(value)
			_, _ = h.Write(seps)
			return true
		}
		b = append(b, key...)
		b = append(b, seps[0])
		b = append(b, value...)
		b = append(b, seps[0])
		return true
	})

	if h != nil {
		return h.Sum64()
	}
	return xxhash.Sum64(b)
}

// hashShadowedLabels hashes the names of group, in ascending order, taking each
// value from own where own holds the same name. It must produce the same bytes
// as hashLabels for the equivalent label set.
func hashShadowedLabels(group, own labelpack.Labels) uint64 {
	b := make([]byte, 0, 1024)
	var h *xxhash.Digest

	ownIter := own.Iter()
	ownName, ownValue, ownOK := ownIter.Next()

	group.Range(func(key, value string) bool {
		// Advance own until it reaches or passes key. Both sides are sorted by
		// name, so this is a single pass over each.
		for ownOK && ownName < key {
			ownName, ownValue, ownOK = ownIter.Next()
		}
		if ownOK && ownName == key {
			value = ownValue
		}

		if h == nil && len(b)+len(key)+len(value)+2 >= cap(b) {
			h = xxhash.New()
			_, _ = h.Write(b)
		}
		if h != nil {
			_, _ = h.WriteString(key)
			_, _ = h.Write(seps)
			_, _ = h.WriteString(value)
			_, _ = h.Write(seps)
			return true
		}
		b = append(b, key...)
		b = append(b, seps[0])
		b = append(b, value...)
		b = append(b, seps[0])
		return true
	})

	if h != nil {
		return h.Sum64()
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
		sharedLabels := tgs[targetIndices[0]].group.labelSet()
		individualLabels := make([]commonlabels.LabelSet, 0, len(targetIndices))
		for _, ind := range targetIndices {
			target := tgs[ind]
			// detect hash collisions - we'll append them separately - it's still correct, just may be less efficient
			if !labelSetEqualsFn(sharedLabels, target.group.labelSet()) {
				hashConflicts = append(hashConflicts, target.LabelSet())
				continue
			}
			individualLabels = append(individualLabels, target.ownLabelSet())
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

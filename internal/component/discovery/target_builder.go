package discovery

import (
	"slices"

	commonlabels "github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery/internal/labelpack"
)

type TargetBuilder interface {
	relabel.LabelBuilder
	Target() Target
	MergeWith(Target) TargetBuilder
}

// targetBuilder accumulates changes to a Target without touching its packed
// label buffers, then applies them all at once in Target(). It follows the same
// model as prometheus/model/labels.Builder: pending additions and deletions are
// kept in small slices, and the base labels are only re-encoded if something
// actually changed.
//
// The zero value is not usable; use one of the constructors.
type targetBuilder struct {
	group *groupLabels
	own   labelpack.Labels

	// add and del stay nil until something is actually changed, so relabel rules
	// that only inspect labels allocate nothing beyond the builder itself.
	add []labelpack.Pair
	del []string
}

// NewTargetBuilder creates an empty labels builder.
func NewTargetBuilder() TargetBuilder {
	return newTargetBuilder(nil, labelpack.Empty)
}

func NewTargetBuilderFrom(t Target) TargetBuilder {
	return newTargetBuilder(t.group, t.own)
}

func NewTargetBuilderFromLabelSets(group, own commonlabels.LabelSet) TargetBuilder {
	t := NewTargetFromSpecificAndBaseLabelSet(own, group)
	return newTargetBuilder(t.group, t.own)
}

func newTargetBuilder(group *groupLabels, own labelpack.Labels) TargetBuilder {
	return &targetBuilder{
		group: group,
		own:   own,
	}
}

func (t *targetBuilder) Get(label string) string {
	// Del removes entries from add but Set does not remove from del, so add has
	// to be checked first.
	for _, p := range t.add {
		if p.Name == label {
			return p.Value
		}
	}
	if slices.Contains(t.del, label) {
		return ""
	}
	if value, ok := t.own.Get(label); ok {
		return value
	}
	value, _ := t.group.labels().Get(label)
	return value
}

func (t *targetBuilder) Range(f func(label string, value string)) {
	// Take a copy of add and del so that they are unaffected by calls to Set or
	// Del from within f. Relabel rules such as labelmap, labeldrop and labelkeep
	// mutate the builder while ranging over it. Stack-based arrays avoid a heap
	// allocation in the common case.
	var addStack [64]labelpack.Pair
	var delStack [64]string
	origAdd := append(addStack[:0], t.add...)
	origDel := append(delStack[:0], t.del...)

	labelpack.RangeMerged(t.group.labels(), t.own, func(label string, value string) bool {
		if slices.Contains(origDel, label) {
			return true // skip if it's deleted
		}
		if containsName(origAdd, label) {
			return true // skip if it was in add
		}
		f(label, value)
		return true
	})
	for _, p := range origAdd {
		f(p.Name, p.Value)
	}
}

func (t *targetBuilder) Set(label string, val string) {
	if val == "" { // Setting to empty is treated as deleting.
		t.Del(label)
		return
	}
	for i, p := range t.add {
		if p.Name == label {
			t.add[i].Value = val
			return
		}
	}
	t.add = append(t.add, labelpack.Pair{Name: label, Value: val})
}

func (t *targetBuilder) Del(labels ...string) {
	for _, label := range labels {
		// If we were adding one, may need to clean it up too.
		if i := indexOfName(t.add, label); i >= 0 {
			t.add = slices.Delete(t.add, i, i+1)
		}
		if !slices.Contains(t.del, label) {
			t.del = append(t.del, label)
		}
	}
}

func (t *targetBuilder) MergeWith(target Target) TargetBuilder {
	// Not on a hot path, so doesn't really need to be optimised.
	target.ForEachLabel(func(key string, value string) bool {
		t.Set(key, value)
		return true
	})
	return t
}

func (t *targetBuilder) Target() Target {
	if len(t.add) == 0 && len(t.del) == 0 {
		// Nothing changed, so both packed buffers can be reused as they are.
		return newTarget(t.group, t.own)
	}

	// Figure out whether the own labels need re-encoding.
	modifyOwn := len(t.add) > 0 // if there is anything to add
	if !modifyOwn {
		for _, label := range t.del { // if there is anything to delete
			if t.own.Has(label) {
				modifyOwn = true
				break
			}
		}
	}

	// Figure out whether the group labels need re-encoding. Deleting a label the
	// group provides has to narrow the group, otherwise it would shine through.
	modifyGroup := false
	for _, label := range t.del {
		if t.group.labels().Has(label) {
			modifyGroup = true
			break
		}
	}

	var (
		newOwn   = t.own
		newGroup = t.group
	)

	if modifyOwn {
		pairs := make([]labelpack.Pair, 0, t.own.Len()+len(t.add))
		t.own.Range(func(name, value string) bool {
			if slices.Contains(t.del, name) {
				return true
			}
			if containsName(t.add, name) {
				// The value from add wins and is appended below; skip it here so
				// FromPairs does not have to de-duplicate.
				return true
			}
			pairs = append(pairs, labelpack.Pair{Name: name, Value: value})
			return true
		})
		pairs = append(pairs, t.add...)
		newOwn, _ = labelpack.FromPairs(pairs)
	}

	if modifyGroup {
		// TODO(thampiotr): When relabeling a lot of targets that require changes to t.group, we might produce a lot of
		//  				t.groups that will be essentially the same. If this becomes a hot spot, it could be
		//  				remediated with an extra step to consolidate them using perhaps a hash as an ID.
		//  				The packed representation makes this straightforward: the packed string is a natural
		//  				interning key.
		pairs := make([]labelpack.Pair, 0, t.group.len())
		t.group.labels().Range(func(name, value string) bool {
			if slices.Contains(t.del, name) {
				return true
			}
			pairs = append(pairs, labelpack.Pair{Name: name, Value: value})
			return true
		})
		newGroup = newGroupLabelsFromPacked(labelpack.FromPairs(pairs))
	}

	return newTarget(newGroup, newOwn)
}

func containsName(pairs []labelpack.Pair, name string) bool {
	return indexOfName(pairs, name) >= 0
}

func indexOfName(pairs []labelpack.Pair, name string) int {
	for i, p := range pairs {
		if p.Name == name {
			return i
		}
	}
	return -1
}

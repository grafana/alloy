package discovery

import (
	commonlabels "github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/relabel"
)

type TargetBuilder interface {
	relabel.LabelBuilder
	Target() Target
	MergeWith(Target) TargetBuilder
}

type targetBuilder struct {
	group commonlabels.LabelSet
	own   commonlabels.LabelSet

	toAdd map[string]string
	toDel map[string]struct{}
}

// NewTargetBuilder creates an empty labels builder.
func NewTargetBuilder() TargetBuilder {
	return targetBuilder{
		group: nil,
		own:   make(commonlabels.LabelSet),
		toAdd: make(map[string]string),
		toDel: make(map[string]struct{}),
	}
}

func NewTargetBuilderFrom(t Target) TargetBuilder {
	return NewTargetBuilderFromLabelSets(t.group, t.own)
}

func NewTargetBuilderFromLabelSets(group, own commonlabels.LabelSet) TargetBuilder {
	toAdd := make(map[string]string)
	toDel := make(map[string]struct{})

	// if we are given labels that are set to empty value, it should be treated as deleting them
	for name, value := range group {
		if len(value) == 0 { // if group has empty value
			// and own doesn't override it OR overrides it with an empty value
			if ownValue, ok := own[name]; !ok || len(ownValue) == 0 {
				toDel[string(name)] = struct{}{} // mark label as deleted
			}
		}
	}
	for name, value := range own {
		if len(value) == 0 {
			toDel[string(name)] = struct{}{}
		}
	}

	return targetBuilder{
		group: group,
		own:   own,
		toAdd: toAdd,
		toDel: toDel,
	}
}

func (t targetBuilder) Get(label string) string {
	if v, ok := t.toAdd[label]; ok {
		return v
	}
	if _, ok := t.toDel[label]; ok {
		return ""
	}
	lv, ok := t.own[commonlabels.LabelName(label)]
	if ok {
		return string(lv)
	}
	lv = t.group[commonlabels.LabelName(label)]
	return string(lv)
}

func (t targetBuilder) Range(f func(label string, value string)) {
	for k, v := range t.toAdd {
		f(k, v)
	}
	for k, v := range t.own {
		if _, deleted := t.toDel[string(k)]; deleted {
			continue // skip if it's deleted
		}
		if _, added := t.toAdd[string(k)]; added {
			continue // skip if it was in toAdd
		}
		f(string(k), string(v))
	}
	for k, v := range t.group {
		if _, deleted := t.toDel[string(k)]; deleted {
			continue // skip if it's deleted
		}
		if _, added := t.toAdd[string(k)]; added {
			continue // skip if it was in toAdd
		}
		if _, inOwn := t.own[k]; inOwn {
			continue // skip if it was in own
		}
		f(string(k), string(v))
	}
}

func (t targetBuilder) Set(label string, val string) {
	if val == "" { // Setting to empty is treated as deleting.
		t.Del(label)
		return
	}
	t.toAdd[label] = val
}

func (t targetBuilder) Del(labels ...string) {
	for _, label := range labels {
		t.toDel[label] = struct{}{}
		// If we were adding one, may need to clean it up too.
		delete(t.toAdd, label)
	}
}

func (t targetBuilder) MergeWith(target Target) TargetBuilder {
	// Not on a hot path, so doesn't really need to be optimised.
	target.ForEachLabel(func(key string, value string) bool {
		t.Set(key, value)
		return true
	})
	return t
}

func (t targetBuilder) Target() Target {
	if len(t.toAdd) == 0 && len(t.toDel) == 0 {
		return NewTargetFromSpecificAndBaseLabelSet(t.own, t.group)
	}
	// Figure out if we need to modify own set
	modifyOwn := false
	if len(t.toAdd) > 0 { // if there is anything to add
		modifyOwn = true
	} else {
		for label := range t.toDel { // if there is anything to delete
			if _, ok := t.own[commonlabels.LabelName(label)]; ok {
				modifyOwn = true
				break
			}
		}
	}

	modifyGroup := false
	for label := range t.toDel { // if there is anything to delete from group
		if _, ok := t.group[commonlabels.LabelName(label)]; ok {
			modifyGroup = true
			break
		}
	}

	var (
		newOwn   = t.own
		newGroup = t.group
	)

	if modifyOwn {
		newOwn = make(commonlabels.LabelSet, len(t.own)+len(t.toAdd))
		for k, v := range t.own {
			if _, ok := t.toDel[string(k)]; ok {
				continue
			}
			newOwn[k] = v
		}
		for k, v := range t.toAdd {
			newOwn[commonlabels.LabelName(k)] = commonlabels.LabelValue(v)
		}
	}
	if modifyGroup {
		// TODO(thampiotr): When relabeling a lot of targets that require changes to t.group, we might produce a lot of
		//  				t.groups that will be essentially the same. If this becomes a hot spot, it could be
		//  				remediated with an extra step to consolidate them using perhaps a hash as an ID.
		newGroup = make(commonlabels.LabelSet, len(t.group))
		for k, v := range t.group {
			if _, ok := t.toDel[string(k)]; ok {
				continue
			}
			newGroup[k] = v
		}
	}

	return NewTargetFromSpecificAndBaseLabelSet(newOwn, newGroup)
}

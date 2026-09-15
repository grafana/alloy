// Package enrich provides shared target matching for enrichment components.
package enrich

import (
	"sort"

	"github.com/cespare/xxhash/v2"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/discovery"
)

// Matcher looks up discovery targets using matching label values. It is safe for
// concurrent lookups. To change the targets or matching strategy, create a new Matcher.
type Matcher struct {
	labelNames []string
	targets    map[uint64]model.LabelSet
}

// NewMatcher builds a lookup from targets. targetToLabel maps target label names
// to incoming label names. Targets with a missing or empty matching label are
// skipped. If multiple targets have the same matching values, the last one wins.
func NewMatcher(targets []discovery.Target, targetToLabel map[string]string) *Matcher {
	targetNames := make([]string, 0, len(targetToLabel))
	for name := range targetToLabel {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	m := &Matcher{
		labelNames: make([]string, 0, len(targetNames)),
		targets:    make(map[uint64]model.LabelSet),
	}
	for _, name := range targetNames {
		m.labelNames = append(m.labelNames, targetToLabel[name])
	}
	for _, target := range targets {
		h, ok := hashValues(func(name string) string {
			value, _ := target.Get(name)
			return value
		}, targetNames)
		if !ok {
			continue
		}
		labelSet := make(model.LabelSet, target.Len())
		target.ForEachLabel(func(name, value string) bool {
			labelSet[model.LabelName(name)] = model.LabelValue(value)
			return true
		})
		m.targets[h] = labelSet
	}
	return m
}

// Match returns the matching target's labels, or nil if no target matches.
// get must return an empty string for missing labels. The returned label set
// belongs to the Matcher and must not be modified.
func (m *Matcher) Match(get func(string) string) model.LabelSet {
	h, ok := hashValues(get, m.labelNames)
	if !ok {
		return nil
	}
	return m.targets[h]
}

// Len returns the number of distinct matching targets in the lookup.
func (m *Matcher) Len() int {
	return len(m.targets)
}

var sep = []byte{0xff} // separates UTF-8 label values to preserve value boundaries

// hashValues returns false if no names are configured or any value is empty.
func hashValues(get func(string) string, names []string) (uint64, bool) {
	if len(names) == 0 {
		return 0, false
	}
	h := xxhash.New()
	for _, name := range names {
		value := get(name)
		if value == "" {
			return 0, false
		}
		_, _ = h.WriteString(value)
		_, _ = h.Write(sep)
	}
	return h.Sum64(), true
}

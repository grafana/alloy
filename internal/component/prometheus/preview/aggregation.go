package preview

import (
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

// Limit retained group values independently of max_groups. In particular, never
// retain the input label set: it can be arbitrarily large or contain secrets.
const maxGroupValueBytes = 4096

type group struct {
	labels  labels.Labels
	samples [2]uint64 // Float samples, then native histogram samples.
	series  uint64
	limited bool
}

type aggregation struct {
	groupBy   []string
	maxGroups int
	maxSeries int
	total     group
	groups    map[string]*group
	overflow  group
	series    map[uint64]*group
}

func newAggregation(args Arguments) *aggregation {
	return &aggregation{
		groupBy:   args.GroupBy,
		maxGroups: args.MaxGroups,
		maxSeries: args.MaxSeries,
		groups:    make(map[string]*group),
		series:    make(map[uint64]*group),
	}
}

func (a *aggregation) groupFor(ls labels.Labels) *group {
	if len(a.groupBy) == 0 {
		return nil
	}
	var size int
	for _, name := range a.groupBy {
		size += len(ls.Get(name))
		if size > maxGroupValueBytes {
			return &a.overflow
		}
	}
	builder := labels.NewScratchBuilder(len(a.groupBy))
	for _, name := range a.groupBy {
		builder.Add("preview_group_"+name, ls.Get(name))
	}
	builder.Sort()
	groupLabels := builder.Labels()
	return a.admitGroup(groupLabels.String(), groupLabels)
}

func (a *aggregation) admitGroup(key string, ls labels.Labels) *group {
	if g, ok := a.groups[key]; ok {
		return g
	}
	if len(a.groups) >= a.maxGroups {
		return &a.overflow
	}
	// Clone values so that a short group value can't keep a large input string
	// allocation alive. This also works with the different labels build tags.
	builder := labels.NewScratchBuilder(ls.Len())
	ls.Range(func(l labels.Label) { builder.Add(l.Name, strings.Clone(l.Value)) })
	g := &group{labels: builder.Labels()}
	a.groups[key] = g
	return g
}

func (a *aggregation) observe(ls labels.Labels, histogram bool) {
	g := a.groupFor(ls)
	kind := 0
	if histogram {
		kind = 1
	}
	a.total.samples[kind]++
	if g != nil {
		g.samples[kind]++
	}
	a.track(ls.Hash(), g)
}

func (a *aggregation) track(hash uint64, g *group) {
	if previous, ok := a.series[hash]; ok {
		// A batch can overflow its group budget before encountering a group
		// admitted by an earlier batch. We still count the series only once
		// globally; signal the resulting lower bound for the other group.
		if previous != g && g != nil {
			g.limited = true
		}
		return
	}
	if len(a.series) >= a.maxSeries {
		a.total.limited = true
		if g != nil {
			g.limited = true
		}
		return
	}
	a.series[hash] = g
	a.total.series++
	if g != nil {
		g.series++
	}
}

// merge applies a committed transaction without retaining its labels or samples.
// Each transaction and the component independently enforce the same bounds.
func (a *aggregation) merge(batch *aggregation) {
	a.total.samples[0] += batch.total.samples[0]
	a.total.samples[1] += batch.total.samples[1]
	a.total.limited = a.total.limited || batch.total.limited
	mapping := make(map[*group]*group, len(batch.groups)+1)
	mapping[&batch.overflow] = &a.overflow
	for key, bg := range batch.groups {
		mapping[bg] = a.admitGroup(key, bg.labels)
	}
	for bg, g := range mapping {
		g.samples[0] += bg.samples[0]
		g.samples[1] += bg.samples[1]
		g.limited = g.limited || bg.limited
	}
	for hash, bg := range batch.series {
		a.track(hash, mapping[bg])
	}
}

func (a *aggregation) resetWindow() {
	clear(a.series)
	a.total.series, a.total.limited = 0, false
	a.overflow.series, a.overflow.limited = 0, false
	for _, g := range a.groups {
		g.series, g.limited = 0, false
	}
}

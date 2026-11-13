package relabel

import (
	"slices"
	"strings"

	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
)

// rebuildMinSize is the smallest cache the collapse check considers. Below this
// the memory a Go map keeps in its buckets after deletions is not worth the cost
// of copying every live entry into a fresh map.
const rebuildMinSize = 10_000

// ruleSnapshot is a comparable copy of the parts of a relabel rule that affect
// its outcome, used to decide whether cached results are still valid.
//
// The rules cannot be compared directly: alloy_relabel.Config holds a compiled
// *regexp.Regexp, so it is not comparable with ==, and the Config values are
// reachable from the component's arguments rather than owned by the component.
// Snapshotting the source form makes the comparison exact and independent of the
// caller.
type ruleSnapshot struct {
	// sourceLabels is joined with a byte that cannot appear in a label name, so
	// that the snapshot stays comparable while still distinguishing
	// ["a", "b"] from ["a\xffb"].
	sourceLabels string
	separator    string
	regex        string
	modulus      uint64
	targetLabel  string
	replacement  string
	action       alloy_relabel.Action
}

const sourceLabelSep = "\xff"

func snapshotRules(cfgs []*alloy_relabel.Config) []ruleSnapshot {
	if len(cfgs) == 0 {
		return nil
	}
	out := make([]ruleSnapshot, 0, len(cfgs))
	for _, cfg := range cfgs {
		if cfg == nil {
			out = append(out, ruleSnapshot{})
			continue
		}
		out = append(out, ruleSnapshot{
			sourceLabels: strings.Join(cfg.SourceLabels, sourceLabelSep),
			separator:    cfg.Separator,
			regex:        cfg.Regex.String(),
			modulus:      cfg.Modulus,
			targetLabel:  cfg.TargetLabel,
			replacement:  cfg.Replacement,
			action:       cfg.Action,
		})
	}
	return out
}

// cacheEntry is the memoised outcome of relabeling one target. Targets dropped by
// the rules are cached too, with keep false, because deciding to drop a target is
// often the most expensive part of a rule set.
type cacheEntry struct {
	output discovery.Target
	keep   bool

	// generation is the last update in which this entry was used. Entries not used
	// in the current update refer to targets that have gone away.
	generation uint64
}

// targetCache memoises relabeling results for the lifetime of a target.
//
// discovery.relabel is handed a complete snapshot of its input on every update,
// so the cache is bounded by the current number of discovered targets rather than
// by a configured size: anything absent from an update is dropped at the end of
// it. That keeps the cache exactly as large as it is useful and means there is no
// size to tune.
//
// Not safe for concurrent use; the component holds its write lock throughout an
// update.
type targetCache struct {
	entries map[discovery.TargetCacheKey]*cacheEntry

	generation uint64
	// seen counts the entries used in the current generation, so that the common
	// case of nothing having gone away can skip the eviction scan.
	seen int
	// peak is the high-water mark of live entries since the last rebuild, measured
	// after eviction. It deliberately excludes the transient growth while an update
	// is in flight: when every target changes, the map holds the previous and the
	// current generation at once, and treating that as a peak would make every
	// update look like a collapse and rebuild the map needlessly.
	peak int
}

func newTargetCache() *targetCache {
	return &targetCache{entries: map[discovery.TargetCacheKey]*cacheEntry{}}
}

// begin starts a new update.
func (c *targetCache) begin() {
	c.generation++
	c.seen = 0
}

// lookup returns the cached result for a target, if any.
func (c *targetCache) lookup(key discovery.TargetCacheKey) (*cacheEntry, bool) {
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	// Mark the entry as still in use. Entries are stored by pointer so this does
	// not need a second map operation, which matters because hashing the key means
	// hashing the target's packed labels.
	if entry.generation != c.generation {
		entry.generation = c.generation
		c.seen++
	}
	return entry, true
}

// insert records the result of relabeling a target.
func (c *targetCache) insert(key discovery.TargetCacheKey, output discovery.Target, keep bool) *cacheEntry {
	entry := &cacheEntry{output: output, keep: keep, generation: c.generation}
	c.entries[key] = entry
	c.seen++
	return entry
}

// end finishes an update, dropping the entries for targets that were not part of
// it.
func (c *targetCache) end() {
	// Entries touched this generation are the live ones; the rest belong to targets
	// that have gone away.
	live := c.seen
	stale := len(c.entries) - live

	// Deleting in place is cheaper than copying the survivors into a fresh map,
	// even when most of the map is stale: measured on a 20k target set where every
	// target changes each update, rebuilding every time cost ~13% more memory for
	// no gain in time.
	if stale > 0 {
		for key, entry := range c.entries {
			if entry.generation != c.generation {
				delete(c.entries, key)
			}
		}
	}

	// A Go map never gives back bucket memory on delete, so once the live set has
	// collapsed far enough, copy what is left into a right-sized map. This is for
	// the case where targets drain away over several updates; a single update that
	// replaces everything keeps a steady live size and does not trigger it.
	if shouldRebuild(c.peak, live) {
		rebuilt := make(map[discovery.TargetCacheKey]*cacheEntry, live)
		for key, entry := range c.entries {
			rebuilt[key] = entry
		}
		c.entries = rebuilt
		c.peak = live
		return
	}

	if live > c.peak {
		c.peak = live
	}
}

// shouldRebuild reports whether the backing map should be replaced with one sized
// to the live entries, to give back memory after the live set has shrunk.
func shouldRebuild(peak, live int) bool {
	return peak >= rebuildMinSize && live*2 <= peak
}

func (c *targetCache) len() int { return len(c.entries) }

// clear drops every entry, for when the relabeling rules change and no cached
// result is valid any more.
func (c *targetCache) clear() {
	// Release the buckets rather than clear() them: the rules changing says
	// nothing about how many targets there will be, and holding the old footprint
	// is not justified.
	c.entries = map[discovery.TargetCacheKey]*cacheEntry{}
	c.seen = 0
	c.peak = 0
}

func rulesChanged(prev, next []ruleSnapshot) bool {
	return !slices.Equal(prev, next)
}

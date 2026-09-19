//go:build unix

package reporter

import (
	"bytes"

	"github.com/google/pprof/profile"
	"github.com/prometheus/prometheus/model/labels"
)

type builtProfile struct {
	profile *profile.Profile
	labels  labels.Labels
}

type profileGroupKey struct {
	labels     string
	sampleType string
}

type profileGroup struct {
	labels   labels.Labels
	profiles []*profile.Profile
}

type profileGroups map[profileGroupKey]*profileGroup

func (g profileGroups) add(profiles []builtProfile, sampleType string) {
	for _, p := range profiles {
		// A merged profile has no single process ID. Do not let discovery's
		// internal PID label prevent aggregation when pid_label is disabled.
		if p.labels.Has("__process_pid__") {
			builder := labels.NewBuilder(p.labels)
			builder.Del("__process_pid__")
			p.labels = builder.Labels()
		}
		// Use the full label set rather than a hash to avoid merging hash collisions.
		key := profileGroupKey{labels: p.labels.String(), sampleType: sampleType}
		group := g[key]
		if group == nil {
			group = &profileGroup{labels: p.labels}
			g[key] = group
		}
		group.profiles = append(group.profiles, p.profile)
	}
}

func (p *PPROFReporter) encodeGroups(groups profileGroups) []PPROF {
	result := make([]PPROF, 0, len(groups))
	for key, group := range groups {
		// Merge once per group, not repeatedly into a growing accumulator.
		// Merge also normalizes ASLR addresses and sums samples with identical
		// stacks and sample labels, including duplicates within a single profile.
		// Off-CPU builders omit PeriodType. Merge expects a non-nil value
		// (as provided by profile.Parse), so normalize only the headers.
		for i, src := range group.profiles {
			if src.PeriodType == nil {
				header := *src
				header.PeriodType = &profile.ValueType{}
				group.profiles[i] = &header
			}
		}
		merged, err := profile.Merge(group.profiles)
		if err != nil {
			p.log.Error("failed to aggregate profiles", "err", err)
			// Preserve the original profiles if aggregation fails.
			for _, original := range group.profiles {
				result = append(result, p.encodeProfiles([]builtProfile{{profile: original, labels: group.labels}})...)
			}
		} else {
			// Release the input profiles before encoding the aggregate.
			clear(group.profiles)
			result = append(result, p.encodeProfiles([]builtProfile{{profile: merged, labels: group.labels}})...)
		}
		delete(groups, key)
	}
	return result
}

func (p *PPROFReporter) encodeProfiles(profiles []builtProfile) []PPROF {
	result := make([]PPROF, 0, len(profiles))
	for _, built := range profiles {
		var buf bytes.Buffer
		if _, err := writeProfile(&buf, built.profile); err != nil {
			p.log.Error("failed to encode profile", "err", err)
			continue
		}
		result = append(result, PPROF{Raw: buf.Bytes(), Samples: len(built.profile.Sample), Labels: built.labels})
	}
	return result
}

// UpdateProfileOptions applies both options atomically for the next collection.
func (p *PPROFReporter) UpdateProfileOptions(pidLabel, aggregate bool) {
	p.profileOptionsMut.Lock()
	defer p.profileOptionsMut.Unlock()
	p.pidLabel = pidLabel
	p.aggregateProfiles = aggregate
}

func (p *PPROFReporter) profileOptions() (pidLabel, aggregate bool) {
	p.profileOptionsMut.RLock()
	defer p.profileOptionsMut.RUnlock()
	return p.pidLabel, p.aggregateProfiles && !p.pidLabel
}

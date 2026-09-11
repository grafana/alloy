package discovery

import (
	"strings"
	"testing"

	commonlabels "github.com/prometheus/common/model"
	modellabels "github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

// decodeFuzzLabels turns fuzzer bytes into a label map. Splitting on a separator
// byte into alternating names and values lets the fuzzer reach duplicate names,
// empty names and empty values easily.
func decodeFuzzLabels(data []byte) map[string]string {
	parts := strings.Split(string(data), "\x00")
	out := map[string]string{}
	for i := 0; i+1 < len(parts); i += 2 {
		out[parts[i]] = parts[i+1]
	}
	return out
}

// referenceLabels is a naive map-based model of what a Target built from a group
// and an own label set must contain. Group labels are inherited, own labels
// override them, and a label with an empty name or an empty value is absent - so
// an empty value in own deletes an inherited group label.
func referenceLabels(group, own map[string]string) map[string]string {
	ref := map[string]string{}
	for name, value := range group {
		if name == "" || value == "" {
			continue
		}
		ref[name] = value
	}
	for name, value := range own {
		if name == "" {
			continue
		}
		if value == "" {
			delete(ref, name)
			continue
		}
		ref[name] = value
	}
	return ref
}

func labelSet(m map[string]string) commonlabels.LabelSet {
	if m == nil {
		return nil
	}
	ls := make(commonlabels.LabelSet, len(m))
	for name, value := range m {
		ls[commonlabels.LabelName(name)] = commonlabels.LabelValue(value)
	}
	return ls
}

// assertTargetMatches checks every Target accessor against the reference label
// map.
func assertTargetMatches(t *testing.T, target Target, want map[string]string) {
	t.Helper()

	require.Equal(t, len(want), target.Len(), "Len()")

	got := map[string]string{}
	prev := ""
	first := true
	finished := target.ForEachLabel(func(key, value string) bool {
		if !first {
			require.Less(t, prev, key, "labels must be iterated in ascending name order")
		}
		prev, first = key, false
		_, dup := got[key]
		require.False(t, dup, "label %q iterated twice", key)
		got[key] = value
		return true
	})
	require.True(t, finished, "ForEachLabel should report a complete iteration")
	require.Equal(t, want, got, "ForEachLabel")

	require.Equal(t, want, target.AsMap(), "AsMap")
	require.Equal(t, labelSet(want), target.LabelSet(), "LabelSet")

	for name, value := range want {
		gotValue, ok := target.Get(name)
		require.True(t, ok, "Get(%q) must find the label", name)
		require.Equal(t, value, gotValue, "Get(%q)", name)
	}

	// PromLabels must be a faithful, sorted conversion, and the Alloy hash must
	// agree with the Prometheus one. The latter is the clustering compatibility
	// contract.
	promLabels := target.PromLabels()
	require.Equal(t, want, promLabels.Map(), "PromLabels")
	require.NoError(t, promLabels.Validate(func(modellabels.Label) error { return nil }))
	require.Equal(t,
		modellabels.StableHash(promLabels),
		target.HashLabelsWithPredicate(func(string) bool { return true }),
		"alloy and prometheus hashes must match")

	// A target built purely from the merged map must compare equal and hash
	// identically, regardless of how the labels were split between group and own.
	flat := NewTargetFromMap(want)
	require.True(t, target.EqualsTarget(&flat), "target must equal its flattened form")
	require.True(t, flat.EqualsTarget(&target), "equality must be symmetric")
	require.Equal(t, flat.NonMetaLabelsHash(), target.NonMetaLabelsHash(),
		"hash must not depend on the group/own split")
	require.Equal(t, flat.String(), target.String(), "String must not depend on the group/own split")
}

// FuzzTargetFromGroupAndOwn checks Target against a naive map reference for
// arbitrary group/own splits, including empty values that delete inherited
// labels.
func FuzzTargetFromGroupAndOwn(f *testing.F) {
	f.Add([]byte("a\x001\x00b\x002"), []byte("a\x00override"))
	f.Add([]byte("a\x001"), []byte("a\x00"))
	f.Add([]byte("a\x001\x00b\x002"), []byte(""))
	f.Add([]byte(""), []byte("a\x001"))
	f.Add([]byte("\x00v"), []byte("\x00v"))
	f.Add([]byte("a\x00"), []byte("a\x00"))
	f.Add([]byte(strings.Repeat("x", 300)+"\x00v"), []byte("a\x001"))

	f.Fuzz(func(t *testing.T, groupData, ownData []byte) {
		groupMap := decodeFuzzLabels(groupData)
		ownMap := decodeFuzzLabels(ownData)
		want := referenceLabels(groupMap, ownMap)

		target := NewTargetFromSpecificAndBaseLabelSet(labelSet(ownMap), labelSet(groupMap))
		assertTargetMatches(t, target, want)

		// A builder that changes nothing must round trip to an equal target.
		roundTripped := NewTargetBuilderFrom(target).Target()
		assertTargetMatches(t, roundTripped, want)
		require.True(t, target.EqualsTarget(&roundTripped))
	})
}

// FuzzTargetBuilder applies a sequence of Set and Del operations to a builder and
// checks the result against both a naive map reference and the Prometheus
// labels.Builder, which TargetBuilder is required to behave like.
func FuzzTargetBuilder(f *testing.F) {
	f.Add([]byte("a\x001\x00b\x002"), []byte("c\x003"), []byte("a\x00b"))
	f.Add([]byte("a\x001"), []byte("a\x00"), []byte(""))
	f.Add([]byte("a\x001"), []byte(""), []byte("a"))
	f.Add([]byte("g\x001"), []byte("g\x002"), []byte("g"))

	f.Fuzz(func(t *testing.T, groupData, setData, delData []byte) {
		groupMap := decodeFuzzLabels(groupData)
		setMap := decodeFuzzLabels(setData)
		delNames := strings.Split(string(delData), "\x00")

		// Prometheus rejects empty label names, and TargetBuilder drops them, so
		// exclude them from the operations being compared.
		for name := range setMap {
			if name == "" {
				delete(setMap, name)
			}
		}

		apply := func(tb TargetBuilder) {
			for name, value := range setMap {
				tb.Set(name, value)
			}
			for _, name := range delNames {
				if name == "" {
					continue
				}
				tb.Del(name)
			}
		}

		// Reference: start from the normalised group labels, then apply the same
		// operations, where setting an empty value deletes.
		want := referenceLabels(groupMap, nil)
		for name, value := range setMap {
			if value == "" {
				delete(want, name)
				continue
			}
			want[name] = value
		}
		for _, name := range delNames {
			delete(want, name)
		}

		// The group/own split must not affect the outcome, so exercise all of
		// them.
		t.Run("all own", func(t *testing.T) {
			tb := NewTargetBuilderFromLabelSets(nil, labelSet(groupMap))
			apply(tb)
			assertTargetMatches(t, tb.Target(), want)
		})

		t.Run("all group", func(t *testing.T) {
			tb := NewTargetBuilderFromLabelSets(labelSet(groupMap), nil)
			apply(tb)
			assertTargetMatches(t, tb.Target(), want)
		})

		t.Run("prometheus builder", func(t *testing.T) {
			// Cross-check against the Prometheus builder over the same inputs.
			tb := newPromBuilderAdapter(modellabels.FromMap(referenceLabels(groupMap, nil)))
			apply(tb)
			assertTargetMatches(t, tb.Target(), want)
		})
	})
}

// FuzzTargetBuilderRangeMutation covers the relabel rules that mutate a builder
// while ranging over it, such as labelmap, labeldrop and labelkeep. Labels set
// during a Range must not be visited by that same Range.
func FuzzTargetBuilderRangeMutation(f *testing.F) {
	f.Add([]byte("a\x001\x00b\x002"), []byte("c\x003"))
	f.Add([]byte("prefix_a\x001"), []byte(""))
	f.Add([]byte(""), []byte(""))
	f.Add([]byte("a\x001"), []byte("a\x00"))

	f.Fuzz(func(t *testing.T, groupData, ownData []byte) {
		groupMap := decodeFuzzLabels(groupData)
		ownMap := decodeFuzzLabels(ownData)

		build := func() TargetBuilder {
			return NewTargetBuilderFromLabelSets(labelSet(groupMap), labelSet(ownMap))
		}

		// labelmap: copy every label to a prefixed name while ranging.
		alloy := build()
		var visited []string
		alloy.Range(func(label, value string) {
			visited = append(visited, label)
			alloy.Set("mapped_"+label, value)
		})

		base := referenceLabels(groupMap, ownMap)
		wantVisited := make([]string, 0, len(base))
		for name := range base {
			wantVisited = append(wantVisited, name)
		}
		require.ElementsMatch(t, wantVisited, visited,
			"Range must visit exactly the base labels, and must not visit labels Set during the Range")

		want := map[string]string{}
		for name, value := range base {
			want[name] = value
			want["mapped_"+name] = value
		}
		assertTargetMatches(t, alloy.Target(), want)

		// labeldrop: delete every label while ranging.
		dropped := build()
		dropped.Range(func(label, value string) {
			dropped.Del(label)
		})
		assertTargetMatches(t, dropped.Target(), map[string]string{})
	})
}

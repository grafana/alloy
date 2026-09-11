package labelpack

import (
	"maps"
	"slices"
	"strings"
	"testing"

	modellabels "github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

// decodeFuzzLabels turns fuzzer bytes into a label map. The input is split on a
// separator byte into alternating names and values, which lets the fuzzer reach
// duplicate names, empty names and empty values easily.
func decodeFuzzLabels(data []byte) map[string]string {
	parts := strings.Split(string(data), "\x00")
	out := map[string]string{}
	for i := 0; i+1 < len(parts); i += 2 {
		out[parts[i]] = parts[i+1]
	}
	return out
}

// referenceNormalize applies the documented construction rules to a label map:
// drop empty names and empty values.
func referenceNormalize(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for name, value := range m {
		if name == "" || value == "" {
			continue
		}
		out[name] = value
	}
	return out
}

// FuzzRoundTrip checks that packing a label set and reading it back yields
// exactly the normalized input, and that Get agrees with Range for every stored
// name.
func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte("a\x001\x00b\x002"))
	f.Add([]byte("\x001"))
	f.Add([]byte("a\x00"))
	f.Add([]byte("a\x001\x00a\x002"))
	f.Add([]byte(strings.Repeat("x", 300) + "\x00v"))

	f.Fuzz(func(t *testing.T, data []byte) {
		input := decodeFuzzLabels(data)
		want := referenceNormalize(input)

		packed, n := FromMap(input)

		require.Equal(t, len(want), n, "reported count")
		require.Equal(t, len(want), packed.Len(), "Len()")
		require.Equal(t, len(want) == 0, packed.IsEmpty(), "IsEmpty()")

		got := map[string]string{}
		prev := ""
		first := true
		packed.Range(func(name, value string) bool {
			if !first {
				require.Less(t, prev, name, "names must be sorted ascending")
			}
			prev, first = name, false
			got[name] = value
			return true
		})
		require.Equal(t, want, got, "round trip")

		// Every stored label must be findable, with the right value.
		for name, value := range want {
			gotValue, ok := packed.Get(name)
			require.True(t, ok, "Get(%q) must find the label", name)
			require.Equal(t, value, gotValue, "Get(%q)", name)
		}

		// Names that were normalized away must not be findable.
		for name := range input {
			if _, kept := want[name]; kept {
				continue
			}
			_, ok := packed.Get(name)
			require.False(t, ok, "Get(%q) must not find a dropped label", name)
		}
	})
}

// FuzzEncodingMatchesPrometheus checks that our packed bytes are identical to
// the Prometheus stringlabels encoding of the same label set. This is the main
// safety net for the hand-rolled encoder: if it ever diverges, StableHash-based
// clustering compatibility would silently break.
func FuzzEncodingMatchesPrometheus(f *testing.F) {
	if modellabels.ImplementationName != "stringlabels" {
		f.Skipf("prometheus labels implementation is %q, not stringlabels", modellabels.ImplementationName)
	}

	f.Add([]byte("a\x001\x00b\x002"))
	f.Add([]byte("zed\x00z\x00abc\x00a"))
	f.Add([]byte(strings.Repeat("x", 254) + "\x00v"))
	f.Add([]byte(strings.Repeat("x", 255) + "\x00v"))
	f.Add([]byte(strings.Repeat("x", 256) + "\x00v"))
	f.Add([]byte("n\x00" + strings.Repeat("v", 300)))

	f.Fuzz(func(t *testing.T, data []byte) {
		want := referenceNormalize(decodeFuzzLabels(data))

		ours, _ := FromMap(want)

		promPairs := make([]modellabels.Label, 0, len(want))
		for _, name := range slices.Sorted(maps.Keys(want)) {
			promPairs = append(promPairs, modellabels.Label{Name: name, Value: want[name]})
		}
		theirs := modellabels.New(promPairs...)

		require.Equal(t, string(theirs.Bytes(nil)), string(ours), "encoding must match prometheus stringlabels")

		// Get must agree with Prometheus for every name, including absent ones.
		for name := range want {
			gotValue, ok := ours.Get(name)
			require.True(t, ok)
			require.Equal(t, theirs.Get(name), gotValue, "Get(%q)", name)
		}
	})
}

// FuzzMerge checks the two-way merge against a naive map-based reference, where
// own shadows group.
func FuzzMerge(f *testing.F) {
	f.Add([]byte("a\x001\x00b\x002"), []byte("a\x00override"))
	f.Add([]byte("b\x00gb\x00d\x00gd"), []byte("a\x00oa\x00c\x00oc\x00e\x00oe"))
	f.Add([]byte(""), []byte("a\x001"))
	f.Add([]byte("a\x001"), []byte(""))

	f.Fuzz(func(t *testing.T, groupData, ownData []byte) {
		groupMap := referenceNormalize(decodeFuzzLabels(groupData))
		ownMap := referenceNormalize(decodeFuzzLabels(ownData))

		// Reference: union with own winning.
		want := make(map[string]string, len(groupMap)+len(ownMap))
		maps.Copy(want, groupMap)
		maps.Copy(want, ownMap)

		group, _ := FromMap(groupMap)
		own, _ := FromMap(ownMap)

		got := map[string]string{}
		prev := ""
		first := true
		finished := RangeMerged(group, own, func(name, value string) bool {
			if !first {
				require.Less(t, prev, name, "merged names must be sorted ascending")
			}
			prev, first = name, false
			_, dup := got[name]
			require.False(t, dup, "label %q yielded twice", name)
			got[name] = value
			return true
		})
		require.True(t, finished)
		require.Equal(t, want, got, "RangeMerged")
		require.Equal(t, len(want), MergedLen(group, own), "MergedLen")
	})
}

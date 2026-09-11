// Package labelpack provides a compact, immutable representation of a set of
// label name/value pairs packed into a single Go string.
//
// The encoding is byte-identical to the "stringlabels" implementation in
// github.com/prometheus/prometheus/model/labels: each name and each value is
// preceded by its length, encoded as a single byte for lengths 0-254, or as the
// byte 255 followed by 3 bytes little-endian for longer strings. The maximum
// encodable length is 1<<24 (16MiB); attempting to encode anything longer
// panics, matching Prometheus' behaviour.
//
// A valid packed buffer always satisfies these invariants:
//
//  1. Label names are sorted in ascending byte order.
//  2. Label names are unique.
//  3. No label has an empty value. An empty value is indistinguishable from an
//     absent label, so such labels are dropped at construction time.
//  4. No label has an empty name. An empty name is not a valid label name, and
//     Get cannot report one as present, so such labels are dropped at
//     construction time.
//
// Callers must only construct Labels through the functions in this package, as
// the accessors rely on the invariants above and will panic or misbehave on
// malformed data.
package labelpack

import (
	"slices"
	"strings"
	"unsafe"

	commonlabels "github.com/prometheus/common/model"
)

// maxLabelLen is the longest name or value that can be encoded. It matches the
// limit of the Prometheus stringlabels encoding, where lengths of 255 or more
// are stored in 3 bytes little-endian.
const maxLabelLen = 1 << 24

// smallSetSize is the number of pairs we can sort on the stack without
// allocating. Real targets are almost always below this.
const smallSetSize = 32

// Labels is an immutable, name-sorted set of label pairs packed into a single
// string. The zero value is a valid, empty set.
type Labels string

// Pair is a single label name/value pair, used when building Labels.
type Pair struct {
	Name  string
	Value string
}

// Empty is the empty set of labels.
const Empty Labels = ""

// FromPairs builds Labels from pairs. It sorts p in place, drops pairs with an
// empty name or an empty value, and de-duplicates by name keeping the last
// occurrence of each name. It returns the packed labels and the number of labels
// retained.
func FromPairs(p []Pair) (Labels, int) {
	if len(p) == 0 {
		return Empty, 0
	}

	// Stable sort so that "keep the last occurrence" is well defined for
	// duplicate names.
	slices.SortStableFunc(p, func(a, b Pair) int { return strings.Compare(a.Name, b.Name) })

	// Compact in place: for runs of equal names keep only the last entry, then
	// drop anything with an empty name or an empty value.
	out := p[:0]
	for i := 0; i < len(p); {
		j := i + 1
		for j < len(p) && p[j].Name == p[i].Name {
			j++
		}
		// p[j-1] is the last pair with this name.
		if last := p[j-1]; last.Name != "" && last.Value != "" {
			out = append(out, last)
		}
		i = j
	}
	p = out

	if len(p) == 0 {
		return Empty, 0
	}

	size := 0
	for i := range p {
		size += pairSize(p[i].Name) + pairSize(p[i].Value)
	}

	buf := make([]byte, 0, size)
	for i := range p {
		buf = appendString(buf, p[i].Name)
		buf = appendString(buf, p[i].Value)
	}
	return Labels(yoloString(buf)), len(p)
}

// yoloString converts buf to a string without copying it. This is the same trick
// prometheus/model/labels uses, and it is safe here because buf was allocated in
// this package and is never written to again after the conversion. Using a
// regular []byte to string conversion would double the allocations of every
// label set built.
func yoloString(buf []byte) string {
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

// FromLabelSet builds Labels from a commonlabels.LabelSet. Labels with an empty
// value are dropped. It returns the packed labels and the number of labels
// retained.
func FromLabelSet(ls commonlabels.LabelSet) (Labels, int) {
	if len(ls) == 0 {
		return Empty, 0
	}

	var stack [smallSetSize]Pair
	var pairs []Pair
	if len(ls) <= len(stack) {
		pairs = stack[:0]
	} else {
		pairs = make([]Pair, 0, len(ls))
	}
	for name, value := range ls {
		if name == "" || value == "" {
			continue
		}
		pairs = append(pairs, Pair{Name: string(name), Value: string(value)})
	}
	return FromPairs(pairs)
}

// FromMap builds Labels from a map of strings. Labels with an empty value are
// dropped. It returns the packed labels and the number of labels retained.
func FromMap(m map[string]string) (Labels, int) {
	if len(m) == 0 {
		return Empty, 0
	}

	var stack [smallSetSize]Pair
	var pairs []Pair
	if len(m) <= len(stack) {
		pairs = stack[:0]
	} else {
		pairs = make([]Pair, 0, len(m))
	}
	for name, value := range m {
		if name == "" || value == "" {
			continue
		}
		pairs = append(pairs, Pair{Name: name, Value: value})
	}
	return FromPairs(pairs)
}

// IsEmpty reports whether l holds no labels.
func (l Labels) IsEmpty() bool { return len(l) == 0 }

// Get returns the value of the label with the given name, and whether it was
// present. Because names are stored in sorted order, the scan stops early once
// it passes the position where name would be.
func (l Labels) Get(name string) (string, bool) {
	if name == "" {
		// An empty name can never be stored, and indexing name[0] below would
		// panic.
		return "", false
	}
	data := string(l)
	for i := 0; i < len(data); {
		var size int
		size, i = decodeSize(data, i)
		// Compare the first byte before slicing, to skip non-matching names as
		// cheaply as possible.
		switch {
		case data[i] == name[0]:
			lName := data[i : i+size]
			i += size
			if lName == name {
				value, _ := decodeString(data, i)
				return value, true
			}
			if lName > name {
				// Names are sorted, so name cannot appear later.
				return "", false
			}
		case data[i] > name[0]:
			// Names are sorted, so name cannot appear later.
			return "", false
		default:
			i += size
		}
		// Skip over the value.
		size, i = decodeSize(data, i)
		i += size
	}
	return "", false
}

// Has reports whether a label with the given name is present.
func (l Labels) Has(name string) bool {
	_, ok := l.Get(name)
	return ok
}

// Len returns the number of labels. It requires a full scan of the buffer, so
// callers on a hot path should cache the count returned at construction time.
func (l Labels) Len() int {
	count := 0
	data := string(l)
	for i := 0; i < len(data); {
		var size int
		size, i = decodeSize(data, i)
		i += size
		size, i = decodeSize(data, i)
		i += size
		count++
	}
	return count
}

// Range calls f for each label in ascending name order. If f returns false the
// iteration stops and Range returns false; otherwise Range returns true once
// every label has been visited.
func (l Labels) Range(f func(name, value string) bool) bool {
	data := string(l)
	for i := 0; i < len(data); {
		var name, value string
		name, i = decodeString(data, i)
		value, i = decodeString(data, i)
		if !f(name, value) {
			return false
		}
	}
	return true
}

// pairSize returns the number of bytes needed to encode s with its length
// prefix. It panics if s is too long to encode.
func pairSize(s string) int {
	switch n := len(s); {
	case n < 255:
		return 1 + n
	case n <= maxLabelLen:
		return 4 + n
	default:
		panic("labelpack: label too long to encode")
	}
}

// appendString appends s to buf, prefixed by its length.
func appendString(buf []byte, s string) []byte {
	if n := len(s); n < 255 {
		buf = append(buf, uint8(n))
	} else if n <= maxLabelLen {
		buf = append(buf, 255, byte(n), byte(n>>8), byte(n>>16))
	} else {
		panic("labelpack: label too long to encode")
	}
	return append(buf, s...)
}

// decodeSize reads a length prefix at index and returns it along with the index
// of the first byte after the prefix.
func decodeSize(data string, index int) (int, int) {
	b := data[index]
	index++
	if b == 255 {
		// Larger numbers are encoded as 3 bytes little-endian. Indexing past the
		// end panics, which is the intended behaviour: all Labels are built
		// inside this package, so malformed data indicates a bug or memory
		// corruption.
		return int(data[index]) + (int(data[index+1]) << 8) + (int(data[index+2]) << 16), index + 3
	}
	return int(b), index
}

// decodeString reads a length-prefixed string at index and returns it along
// with the index of the first byte after it. The returned string shares memory
// with data, which is safe because Labels is immutable.
func decodeString(data string, index int) (string, int) {
	var size int
	size, index = decodeSize(data, index)
	return data[index : index+size], index + size
}

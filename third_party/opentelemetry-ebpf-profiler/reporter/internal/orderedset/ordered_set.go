package orderedset // import "go.opentelemetry.io/ebpf-profiler/reporter/internal/orderedset"

// OrderedSet is a set that keeps order of insertion.
type OrderedSet[T comparable] map[T]int32

// Add adds an element to the set and returns its index.
func (os OrderedSet[T]) Add(key T) int32 {
	idx, _ := os.AddWithCheck(key)
	return idx
}

// AddWithCheck adds an element to the set, returns its index and presence state.
func (os OrderedSet[T]) AddWithCheck(key T) (int32, bool) {
	if idx, exists := os[key]; exists {
		return idx, true
	}

	idx := int32(len(os))
	os[key] = idx
	return idx, false
}

// ToSlice returns the elements of the set as a slice, in insertion order.
func (os OrderedSet[T]) ToSlice() []T {
	ret := make([]T, len(os))
	for key, idx := range os {
		ret[idx] = key
	}

	return ret
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf // import "go.opentelemetry.io/ebpf-profiler/libpf"

// Set is a convenience alias for a map with a `Void` key.
type Set[T comparable] map[T]Void

// ToSlice converts the Set keys into a slice.
func (s Set[T]) ToSlice() []T {
	slice := make([]T, 0, len(s))
	for item := range s {
		slice = append(slice, item)
	}
	return slice
}

// MapKeysToSlice creates a slice from a map's keys.
func MapKeysToSlice[K comparable, V any](m map[K]V) []K {
	slice := make([]K, 0, len(m))
	for key := range m {
		slice = append(slice, key)
	}
	return slice
}

// MapKeysToSet creates a set from a map's keys.
func MapKeysToSet[K comparable, V any](m map[K]V) Set[K] {
	set := make(Set[K], len(m))
	for key := range m {
		set[key] = Void{}
	}
	return set
}

// SliceAllEqual checks whether all items in a slice have a given value.
func SliceAllEqual[T comparable](s []T, value T) bool {
	for _, item := range s {
		if item != value {
			return false
		}
	}

	return true
}

//go:build slicelabels

package discovery

import "github.com/cespare/xxhash/v2"

var (
	seps = []byte{'\xff'}
)

// NOTE 1: This function is copied from Prometheus codebase and adapted to work correctly with Alloy types.
// NOTE 2: It is important to keep the hashing function consistent between Alloy versions in order to have
//
//	smooth rollouts without duplicated or missing data. There are tests to verify this behaviour. Do not change it.
func HashLabelsInOrder(t Target, order []string) uint64 {
	// This optimisation is adapted from prometheus/model/labels.
	// Use xxhash.Sum64(b) for fast path as it's faster.
	b := make([]byte, 0, 1024)
	mustGet := func(label string) string {
		val, _ := t.Get(label)
		// if val is not found it would mean there is a bug and Target is no longer immutable. But we can still provide
		// a consistent hashing behaviour by returning empty string we got from Get.
		return val
	}

	for i, key := range order {
		value := mustGet(key)
		if len(b)+len(key)+len(value)+2 >= cap(b) {
			// If labels entry is 1KB+ do not allocate whole entry.
			h := xxhash.New()
			_, _ = h.Write(b)
			for _, key := range order[i:] {
				_, _ = h.WriteString(key)
				_, _ = h.Write(seps)
				_, _ = h.WriteString(mustGet(key))
				_, _ = h.Write(seps)
			}
			return h.Sum64()
		}

		b = append(b, key...)
		b = append(b, seps[0])
		b = append(b, value...)
		b = append(b, seps[0])
	}
	return xxhash.Sum64(b)
}

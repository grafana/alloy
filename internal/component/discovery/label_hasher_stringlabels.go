//go:build !slicelabels

package discovery

import "github.com/prometheus/prometheus/model/labels"

func hashLabelsInOrder(t Target, order []string) uint64 {
	builder := labels.NewScratchBuilder(len(order))
	mustGet := func(label string) string {
		val, _ := t.Get(label)
		// if val is not found it would mean there is a bug and Target is no longer immutable. But we can still provide
		// a consistent hashing behaviour by returning empty string we got from Get.
		return val
	}

	for _, key := range order {
		value := mustGet(key)
		builder.Add(key, value)
	}

	builder.Sort()
	return builder.Labels().Hash()
}

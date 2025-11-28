package reporter

import (
	"math/rand"
	"testing"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

func TestMapShrink(t *testing.T) {
	tr := newInProgressTracker(0.2)
	r := rand.New(rand.NewSource(0))

	items := make([]libpf.FileID, 100)
	for i := 0; i < 100; i++ {
		items[i] = libpf.NewFileID(
			r.Uint64(),
			r.Uint64(),
		)

		tr.GetOrAdd(items[i])
	}

	if tr.maxSizeSeen != 100 {
		t.Errorf("expected 100, got %d", tr.maxSizeSeen)
	}

	for i := 0; i < 10; i++ {
		tr.Remove(items[i])
	}

	if tr.maxSizeSeen != 100 {
		t.Errorf("expected 100, got %d", tr.maxSizeSeen)
	}

	for i := 10; i < 20; i++ {
		tr.Remove(items[i])
	}

	if tr.maxSizeSeen != 83 {
		t.Errorf("expected 83, got %d", tr.maxSizeSeen)
	}

	// adding up to 83 doesn't change anything
	for i := 10; i < 13; i++ {
		tr.GetOrAdd(items[i])
	}

	if tr.maxSizeSeen != 83 {
		t.Errorf("expected 83, got %d", tr.maxSizeSeen)
	}

	// adding 84th item should increases the max size
	tr.GetOrAdd(items[13])

	if tr.maxSizeSeen != 84 {
		t.Errorf("expected 84, got %d", tr.maxSizeSeen)
	}
}

package deps

import (
	"slices"
	"sort"
	"testing"
)

func TestResolve_OK(t *testing.T) {
	got, err := Resolve([]string{"loki", "mimir"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var names []string
	for _, s := range got {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	if !slices.Equal(names, []string{"loki", "mimir"}) {
		t.Fatalf("unexpected specs: %v", names)
	}
}

func TestResolve_Unknown(t *testing.T) {
	_, err := Resolve([]string{"loki", "tempo"})
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestAllContainsAllKnownBackends(t *testing.T) {
	want := []Spec{Loki, Mimir}
	if !slices.EqualFunc(want, All, func(a, b Spec) bool { return a.Name == b.Name }) {
		t.Fatalf("All out of sync with named vars: %v", All)
	}
}

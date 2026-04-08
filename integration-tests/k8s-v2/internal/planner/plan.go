package planner

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

func SelectTests(all []TestCase, selected string) ([]TestCase, error) {
	if selected == "" || selected == "all" {
		return slices.Clone(all), nil
	}

	want := map[string]struct{}{}
	for _, name := range strings.Split(selected, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		want[name] = struct{}{}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("selected test set is empty")
	}

	out := make([]TestCase, 0, len(want))
	for _, tc := range all {
		if _, ok := want[tc.Name]; ok {
			out = append(out, tc)
			delete(want, tc.Name)
		}
	}
	if len(want) > 0 {
		unknown := make([]string, 0, len(want))
		for n := range want {
			unknown = append(unknown, n)
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown test(s): %s", strings.Join(unknown, ", "))
	}
	return out, nil
}

func RequirementUnion(selected []TestCase) []string {
	union := map[string]struct{}{}
	for _, tc := range selected {
		for _, dep := range tc.Requires {
			union[dep] = struct{}{}
		}
	}
	out := make([]string, 0, len(union))
	for dep := range union {
		out = append(out, dep)
	}
	sort.Strings(out)
	return out
}

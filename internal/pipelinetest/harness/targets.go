package harness

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
)

// targetsOf reads the targets a component exports, such as
// discovery.relabel.pods.output. The reference is the component ID followed
// by the name of an export holding a list of targets.
func (a *Alloy) targetsOf(ref string) ([]map[string]string, error) {
	i := strings.LastIndex(ref, ".")
	if i < 0 {
		return nil, fmt.Errorf("%q is not a component export such as discovery.relabel.pods.output", ref)
	}
	id, field := ref[:i], ref[i+1:]

	info, err := a.ctrl.GetComponent(component.ParseID(id), component.InfoOptions{GetExports: true})
	if err != nil {
		return nil, err
	}
	if info.Exports == nil {
		return nil, fmt.Errorf("component %s has not exported anything yet", id)
	}

	v := reflect.ValueOf(info.Exports)
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("component %s exports %T, not a block of exports", id, info.Exports)
	}
	for i := range v.NumField() {
		name, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("alloy"), ",")
		if name != field {
			continue
		}
		targets, ok := v.Field(i).Interface().([]discovery.Target)
		if !ok {
			return nil, fmt.Errorf("%s is %s, not a list of targets", ref, v.Field(i).Type())
		}
		out := make([]map[string]string, 0, len(targets))
		for _, t := range targets {
			out = append(out, t.AsMap())
		}
		return out, nil
	}
	return nil, fmt.Errorf("component %s has no export named %s", id, field)
}

// targetsSnapshot holds the targets of one export, or the error reading them.
type targetsSnapshot struct {
	targets []map[string]string
	err     error
}

// TargetMatcher matches one target's labels.
type TargetMatcher struct {
	match func(target map[string]string) bool
	text  string
}

// TargetSelector matches targets whose labels satisfy every matcher.
func TargetSelector(selector string, matchers []*labels.Matcher) TargetMatcher {
	return TargetMatcher{
		match: func(t map[string]string) bool {
			for _, m := range matchers {
				if !m.Matches(t[m.Name]) {
					return false
				}
			}
			return true
		},
		text: "selector " + selector,
	}
}

// TargetLabels matches target labels. When partial is true, the target may
// carry labels beyond want.
func TargetLabels(want map[string]string, partial bool) TargetMatcher {
	return TargetMatcher{
		match: func(t map[string]string) bool { return stringMapMatches(t, want, partial) },
		text:  "labels " + renderMapMatch(want, partial),
	}
}

// TargetCount returns an Assertion that passes when exactly want targets of
// ref match all matchers.
func TargetCount(ref string, want int, matchers ...TargetMatcher) Assertion {
	return func(s snapshot) error {
		targets, err := s.targetsFor(ref)
		if err != nil {
			return AssertionError{Kind: "targets.count", Message: err.Error()}
		}
		got := 0
		for _, t := range targets {
			if targetMatches(t, matchers) {
				got++
			}
		}
		if got == want {
			return nil
		}
		msg := fmt.Sprintf("want %d, got %d targets of %s", want, got, ref)
		if c := targetConditions(matchers); c != "" {
			msg += " for " + c
		}
		return AssertionError{Kind: "targets.count", Message: msg}
	}
}

// TargetContains returns an Assertion that passes when at least one target of
// ref matches all matchers.
func TargetContains(ref string, matchers ...TargetMatcher) Assertion {
	return func(s snapshot) error {
		targets, err := s.targetsFor(ref)
		if err != nil {
			return AssertionError{Kind: "targets.contains", Message: err.Error()}
		}
		for _, t := range targets {
			if targetMatches(t, matchers) {
				return nil
			}
		}
		msg := "no matching target of " + ref
		if c := targetConditions(matchers); c != "" {
			msg += " for " + c
		}
		return AssertionError{Kind: "targets.contains", Message: msg}
	}
}

func (s snapshot) targetsFor(ref string) ([]map[string]string, error) {
	ts, ok := s.targets[ref]
	if !ok {
		return nil, fmt.Errorf("targets of %s were not collected", ref)
	}
	if ts.err != nil {
		return nil, fmt.Errorf("cannot read targets of %s: %w", ref, ts.err)
	}
	return ts.targets, nil
}

func targetMatches(t map[string]string, matchers []TargetMatcher) bool {
	for _, m := range matchers {
		if !m.match(t) {
			return false
		}
	}
	return true
}

func targetConditions(matchers []TargetMatcher) string {
	parts := make([]string, 0, len(matchers))
	for _, m := range matchers {
		parts = append(parts, m.text)
	}
	return strings.Join(parts, ", ")
}

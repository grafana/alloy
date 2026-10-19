// Package shapecheck compares the shape of a hand-written Alloy Arguments
// struct against the upstream collector config it converts to, so upstream
// adding, dropping or renaming a field fails a test instead of drifting.
package shapecheck

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/confmap"
)

var unmarshalerType = reflect.TypeFor[confmap.Unmarshaler]()

// Allow lists every intentional difference between the two shapes. Paths are
// dotted: mapstructure names for upstream, alloy names for Alloy.
type Allow struct {
	// Hoist names Alloy blocks whose fields mirror the parent upstream struct
	// (such as `client`). Their children are compared one level up.
	Hoist []string
	// Rename maps an upstream path to the Alloy path it is exposed as, along
	// with everything below it (such as a plural list exposed as a singular
	// repeated block).
	Rename map[string]string
	// Skip lists upstream paths (and everything below) Alloy doesn't expose.
	Skip map[string]string
	// AlloyOnly lists Alloy paths (and everything below) with no upstream field.
	AlloyOnly map[string]string
}

// Check fails t for every difference between upstream and args that allow
// doesn't list, and for every allow entry that no longer matches anything.
func Check(t testing.TB, upstream, args any, allow Allow) {
	t.Helper()
	for _, p := range Diff(upstream, args, allow) {
		t.Error(p)
	}
}

// Diff returns the problems Check reports.
func Diff(upstream, args any, allow Allow) []string {
	up := map[string]bool{}
	walk(reflect.TypeOf(upstream), "", "mapstructure", nil, up)
	al := map[string]bool{}
	walk(reflect.TypeOf(args), "", "alloy", allow.Hoist, al)

	var out []string
	covered := func(m map[string]string, path string) bool {
		for p := range m {
			if path == p || strings.HasPrefix(path, p+".") {
				return true
			}
		}
		return false
	}

	// exposedAs returns the Alloy path upstream path p is exposed under.
	exposedAs := func(p string) string {
		best := ""
		for u := range allow.Rename {
			if (p == u || strings.HasPrefix(p, u+".")) && len(u) > len(best) {
				best = u
			}
		}
		if best == "" {
			return p
		}
		return allow.Rename[best] + strings.TrimPrefix(p, best)
	}

	renamed := map[string]bool{}
	for p := range up {
		renamed[exposedAs(p)] = true
	}
	for u, a := range allow.Rename {
		renamed[a] = true
		if !up[u] {
			out = append(out, fmt.Sprintf("stale Rename: upstream no longer has %q (was exposed as %q)", u, a))
		}
		if !al[a] {
			out = append(out, fmt.Sprintf("stale Rename: Alloy no longer has %q (maps upstream %q)", a, u))
		}
	}
	for p, reason := range allow.Skip {
		if !up[p] {
			out = append(out, fmt.Sprintf("stale Skip: upstream no longer has %q (%s)", p, reason))
		}
	}
	for p, reason := range allow.AlloyOnly {
		if !al[p] {
			out = append(out, fmt.Sprintf("stale AlloyOnly: Alloy no longer has %q (%s)", p, reason))
		}
		if up[p] {
			out = append(out, fmt.Sprintf("stale AlloyOnly: upstream now has %q, drop it from AlloyOnly (%s)", p, reason))
		}
	}

	missing := map[string]bool{}
	for p := range up {
		if !al[exposedAs(p)] && !covered(allow.Skip, p) {
			missing[p] = true
		}
	}
	for _, p := range sorted(missing) {
		// A wrapper block Alloy flattens away (such as `protocols`) is fine as
		// long as everything under it is accounted for.
		if hasChild(up, p) && !hasChild(missing, p) {
			continue
		}
		out = append(out, fmt.Sprintf("upstream field %q isn't exposed in Alloy: add it to Arguments or to Skip with a reason", p))
	}
	for _, p := range sorted(al) {
		if renamed[p] || covered(allow.AlloyOnly, p) {
			continue
		}
		out = append(out, fmt.Sprintf("Alloy field %q has no upstream counterpart: upstream dropped or renamed it, or add it to AlloyOnly with a reason", p))
	}
	return out
}

func hasChild(m map[string]bool, p string) bool {
	for q := range m {
		if strings.HasPrefix(q, p+".") {
			return true
		}
	}
	return false
}

func sorted(m map[string]bool) []string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

// walk records every dotted path (blocks and leaves) reachable from t.
// A struct with no tagged fields is a leaf, which covers opaque upstream types
// such as component.ID and Alloy capsule handlers. Alloy slices are repeated
// blocks and are walked. Upstream slices and maps are walked too, unless the
// type decodes itself (configopaque.MapList takes a map form).
func walk(t reflect.Type, prefix, tag string, hoist []string, out map[string]bool) {
	for t.Kind() == reflect.Pointer ||
		((t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && (tag == "alloy" || !reflect.PointerTo(t).Implements(unmarshalerType))) ||
		(t.Kind() == reflect.Map && tag == "mapstructure" && !reflect.PointerTo(t).Implements(unmarshalerType)) {
		t = t.Elem()
	}
	if inner, ok := optionalInner(t); ok {
		t = inner
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		v, ok := sf.Tag.Lookup(tag)
		if !ok {
			continue
		}
		parts := strings.Split(v, ",")
		name := parts[0]
		squash := slices.Contains(parts[1:], "squash")
		// mapstructure decodes into an unexported embedded struct when it's squashed.
		if !sf.IsExported() && !(sf.Anonymous && squash) {
			continue
		}
		if name == "-" || slices.Contains(parts[1:], "label") || slices.Contains(parts[1:], "enum") {
			continue
		}
		if squash {
			walk(sf.Type, prefix, tag, hoist, out)
			continue
		}
		// A nameless field (mapstructure's ",remain") holds unknown keys, not a field.
		if name == "" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if slices.Contains(hoist, path) {
			walk(sf.Type, prefix, tag, hoist, out)
			continue
		}
		out[path] = true
		walk(sf.Type, path, tag, hoist, out)
	}
}

// optionalInner unwraps configoptional.Optional[T] to T.
func optionalInner(t reflect.Type) (reflect.Type, bool) {
	if t.Kind() != reflect.Struct || !strings.HasPrefix(t.Name(), "Optional[") {
		return nil, false
	}
	get, ok := reflect.PointerTo(t).MethodByName("Get")
	if !ok || get.Type.NumOut() != 1 || get.Type.Out(0).Kind() != reflect.Pointer {
		return nil, false
	}
	return get.Type.Out(0).Elem(), true
}

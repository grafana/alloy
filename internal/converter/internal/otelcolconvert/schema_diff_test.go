package otelcolconvert

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/grafana/alloy/internal/component"
)

// TestSchemaDiffReport compares every hand-written otelcol component with the
// schema the bridge would derive from its upstream configuration. It shows
// which components could move to the bridge without a breaking change.
//
// It only runs when ALLOY_SCHEMA_DIFF_REPORT names a file to write the
// Markdown report to:
//
//	ALLOY_SCHEMA_DIFF_REPORT=/tmp/report.md go test -run TestSchemaDiffReport ./internal/converter/internal/otelcolconvert/
func TestSchemaDiffReport(t *testing.T) {
	out := os.Getenv("ALLOY_SCHEMA_DIFF_REPORT")
	if out == "" {
		t.Skip("set ALLOY_SCHEMA_DIFF_REPORT to write the report")
	}

	var rows []schemaDiff
	for _, c := range converters {
		reg, ok := component.Get(c.InputComponentName())
		if !ok {
			continue
		}
		rows = append(rows, diffComponent(c.InputComponentName(), c.Factory(), reg))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	require.NoError(t, os.WriteFile(out, []byte(renderReport(rows)), 0o600))
}

type schemaDiff struct {
	name     string
	category string

	alloyOnly     []string // Paths only in Alloy (renamed or Alloy-specific settings).
	kindMismatch  []string // Paths that are a block on one side and an attribute on the other.
	upstreamOnly  []string // Paths only upstream (settings Alloy doesn't expose yet).
	defaultsDiff  []string // Keys whose default values differ.
	defaultsError string
}

const (
	catBridged      = "already bridged"
	catIdentical    = "identical"
	catAdditive     = "upstream adds settings only"
	catDefaultsOnly = "defaults differ"
	catBreaking     = "breaking"
)

func diffComponent(name string, factory otelcomponent.Factory, reg component.Registration) schemaDiff {
	d := schemaDiff{name: name}

	argsType := reflect.TypeOf(reg.Args)
	if strings.Contains(argsType.PkgPath(), "/otelcol/bridge") {
		d.category = catBridged
		return d
	}

	alloy := map[string]string{}
	walkAlloy(argsType, "", alloy)
	for _, wrapperOnly := range []string{"output", "debug_metrics"} {
		for path := range alloy {
			if path == wrapperOnly || strings.HasPrefix(path, wrapperOnly+".") {
				delete(alloy, path)
			}
		}
	}

	upstream := map[string]string{}
	walkMapstructure(reflect.TypeOf(factory.CreateDefaultConfig()), "", upstream, map[reflect.Type]bool{})

	for path, kind := range alloy {
		switch upKind, ok := upstream[path]; {
		case !ok:
			d.alloyOnly = append(d.alloyOnly, path)
		case upKind != kind:
			d.kindMismatch = append(d.kindMismatch, fmt.Sprintf("%s (Alloy %s, upstream %s)", path, kind, upKind))
		}
	}
	for path := range upstream {
		if _, ok := alloy[path]; !ok && !hasAlloyAncestorMissing(path, alloy, upstream) {
			d.upstreamOnly = append(d.upstreamOnly, path)
		}
	}

	d.defaultsDiff, d.defaultsError = diffDefaults(factory, reg)

	slices.Sort(d.alloyOnly)
	slices.Sort(d.kindMismatch)
	slices.Sort(d.upstreamOnly)

	switch {
	case len(d.alloyOnly) > 0 || len(d.kindMismatch) > 0:
		d.category = catBreaking
	case len(d.defaultsDiff) > 0 || d.defaultsError != "":
		d.category = catDefaultsOnly
	case len(d.upstreamOnly) > 0:
		d.category = catAdditive
	default:
		d.category = catIdentical
	}
	return d
}

// hasAlloyAncestorMissing reports whether an upstream path sits below a block
// that Alloy doesn't have at all, so only the top-most missing block is
// reported.
func hasAlloyAncestorMissing(path string, alloy, upstream map[string]string) bool {
	parts := strings.Split(path, ".")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], ".")
		if _, inUpstream := upstream[parent]; inUpstream {
			if _, inAlloy := alloy[parent]; !inAlloy {
				return true
			}
		}
	}
	return false
}

// walkAlloy records every attribute and block path of an Alloy arguments type.
func walkAlloy(ty reflect.Type, prefix string, out map[string]string) {
	ty = deref(ty)
	if ty.Kind() != reflect.Struct {
		return
	}
	for field := range ty.Fields() {
		tag, ok := field.Tag.Lookup("alloy")
		if !ok {
			continue
		}
		name, flags, _ := strings.Cut(tag, ",")
		switch {
		case strings.HasPrefix(flags, "squash,mapstructure"):
			walkMapstructure(field.Type, prefix, out, map[reflect.Type]bool{})
		case strings.HasPrefix(flags, "squash"):
			walkAlloy(field.Type, prefix, out)
		case strings.HasPrefix(flags, "label"):
		case strings.HasPrefix(flags, "block"), strings.HasPrefix(flags, "enum"):
			path := prefix + name
			out[path] = "block"
			walkAlloy(elemStruct(field.Type), path+".", out)
		default:
			out[prefix+name] = "attr"
		}
	}
}

// walkMapstructure records every path of an upstream configuration type using
// the same rules as the Alloy syntax mapstructure dialect.
func walkMapstructure(ty reflect.Type, prefix string, out map[string]string, seen map[reflect.Type]bool) {
	ty = deref(ty)
	if ty.Kind() != reflect.Struct || seen[ty] {
		return
	}
	seen[ty] = true
	defer delete(seen, ty)

	for field := range ty.Fields() {
		name, opts, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")
		if name == "-" {
			continue
		}
		squash := slices.Contains(strings.Split(opts, ","), "squash")
		if !field.IsExported() && !(squash && field.Anonymous) {
			continue
		}
		if squash {
			walkMapstructure(field.Type, prefix, out, seen)
			continue
		}
		if name == "" {
			name = field.Name
		}
		path := prefix + name
		if isMapstructureBlock(field.Type) {
			out[path] = "block"
			walkMapstructure(elemStruct(field.Type), path+".", out, seen)
		} else {
			out[path] = "attr"
		}
	}
}

func isMapstructureBlock(ty reflect.Type) bool {
	if isTextUnmarshaler(ty) {
		return false
	}
	base := deref(ty)
	switch base.Kind() {
	case reflect.Struct:
		return true
	case reflect.Slice, reflect.Array:
		return !isTextUnmarshaler(base.Elem()) && deref(base.Elem()).Kind() == reflect.Struct
	}
	return false
}

func isTextUnmarshaler(ty reflect.Type) bool {
	tu := reflect.TypeFor[interface{ UnmarshalText([]byte) error }]()
	return ty.Implements(tu) || reflect.PointerTo(ty).Implements(tu)
}

func elemStruct(ty reflect.Type) reflect.Type {
	ty = deref(ty)
	if ty.Kind() == reflect.Slice || ty.Kind() == reflect.Array {
		ty = deref(ty.Elem())
	}
	return ty
}

func deref(ty reflect.Type) reflect.Type {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	return ty
}

// diffDefaults compares the upstream configuration produced by Alloy's
// default arguments with the upstream factory defaults.
func diffDefaults(factory otelcomponent.Factory, reg component.Registration) (diff []string, errMsg string) {
	args := reflect.New(reflect.TypeOf(reg.Args))
	if d, ok := args.Interface().(interface{ SetToDefault() }); ok {
		d.SetToDefault()
	}
	conv, ok := args.Elem().Interface().(interface {
		Convert() (otelcomponent.Config, error)
	})
	if !ok {
		return nil, "no Convert method"
	}

	var alloyCfg otelcomponent.Config
	func() {
		defer func() {
			if r := recover(); r != nil {
				errMsg = fmt.Sprintf("Convert panicked on defaults: %v", r)
			}
		}()
		var err error
		if alloyCfg, err = conv.Convert(); err != nil {
			errMsg = "Convert failed on defaults: " + err.Error()
		}
	}()
	if errMsg != "" {
		return nil, errMsg
	}

	alloyMap, err := flatten(alloyCfg)
	if err != nil {
		return nil, "marshal: " + err.Error()
	}
	upstreamMap, err := flatten(factory.CreateDefaultConfig())
	if err != nil {
		return nil, "marshal: " + err.Error()
	}

	keys := map[string]bool{}
	for k := range alloyMap {
		keys[k] = true
	}
	for k := range upstreamMap {
		keys[k] = true
	}
	for k := range keys {
		a, u := alloyMap[k], upstreamMap[k]
		if isEmpty(a) && isEmpty(u) {
			continue
		}
		if !reflect.DeepEqual(a, u) {
			diff = append(diff, fmt.Sprintf("%s: Alloy %v, upstream %v", k, a, u))
		}
	}
	slices.Sort(diff)
	return diff, ""
}

func flatten(cfg any) (map[string]any, error) {
	conf := confmap.New()
	if err := conf.Marshal(cfg); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, k := range conf.AllKeys() {
		out[k] = conf.Get(k)
	}
	return out, nil
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Map, reflect.String:
		return rv.Len() == 0
	}
	return rv.IsZero()
}

func renderReport(rows []schemaDiff) string {
	var sb strings.Builder

	counts := map[string]int{}
	for _, r := range rows {
		counts[r.category]++
	}
	sb.WriteString("# otelcol schema diff\n\n")
	sb.WriteString("| Category | Components |\n|---|---|\n")
	for _, c := range []string{catBridged, catIdentical, catAdditive, catDefaultsOnly, catBreaking} {
		fmt.Fprintf(&sb, "| %s | %d |\n", c, counts[c])
	}

	sb.WriteString("\n| Component | Category | Alloy-only | Kind mismatch | Upstream-only | Defaults differ |\n|---|---|---|---|---|---|\n")
	for _, r := range rows {
		defaults := fmt.Sprint(len(r.defaultsDiff))
		if r.defaultsError != "" {
			defaults = "error"
		}
		fmt.Fprintf(&sb, "| `%s` | %s | %d | %d | %d | %s |\n",
			r.name, r.category, len(r.alloyOnly), len(r.kindMismatch), len(r.upstreamOnly), defaults)
	}

	sb.WriteString("\n## Details\n")
	for _, r := range rows {
		if r.category == catBridged || r.category == catIdentical {
			continue
		}
		fmt.Fprintf(&sb, "\n### `%s` (%s)\n", r.name, r.category)
		writeList(&sb, "Alloy-only", r.alloyOnly)
		writeList(&sb, "Kind mismatch", r.kindMismatch)
		writeList(&sb, "Upstream-only", r.upstreamOnly)
		writeList(&sb, "Defaults differ", r.defaultsDiff)
		if r.defaultsError != "" {
			fmt.Fprintf(&sb, "- Defaults: %s\n", r.defaultsError)
		}
	}
	return sb.String()
}

func writeList(sb *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(sb, "- %s:\n", title)
	for _, item := range items {
		fmt.Fprintf(sb, "  - `%s`\n", item)
	}
}

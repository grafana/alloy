//go:build ignore

// Code generator for args_gen.go and beyla_config_gen.go.
//
// Usage: go run ./gen/main.go (from internal/component/beyla/ebpf/)
//
// This generator:
//  1. Parses all *.go files in the parent directory to extract existing struct
//     fields and their alloy tags (preserving Go names and types).
//  2. Reads schema.json (Beyla's published config schema).
//  3. Reads gen/mappings.json for declarative divergence rules.
//  4. Auto-detects sections from schema.properties × Arguments alloy tags.
//  5. Auto-classifies types as migrated/keep based on schema cross-reference.
//  6. Generates args_gen.go: struct definitions for all schema-derived types.
//  7. Rewrites args.go: removes struct declarations migrated to args_gen.go.
//  8. Generates beyla_config_gen.go: config emission functions + fill helpers.
//
// Only mappings.json ever needs to be edited by humans when Beyla updates its
// schema. The generator derives everything else from schema.json + the
// Arguments struct.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// ── Mapping types ─────────────────────────────────────────────────────────────

type MultiSectionEntry struct {
	AlloyPath   string   `json:"alloy_path"`
	YamlSection string   `json:"yaml_section"`
	AlloyFields []string `json:"alloy_fields"` // empty = all fields of struct at alloy_path
}

type FlattenTransform struct {
	SchemaType  string `json:"schema_type"`
	Prefix      string `json:"prefix"`
	AlloyBlock  string `json:"alloy_block"`
	GoSliceType string `json:"go_slice_type"`
	GoItemType  string `json:"go_item_type"`
	// ExtraSchemaTypes are additional schema $defs with the same shape that map to
	// the same Go types (e.g. RegexSelector alongside GlobAttributes → Service).
	ExtraSchemaTypes []string `json:"extra_schema_types"`
}

type Mappings struct {
	NameOverrides     map[string]string     `json:"name_overrides"`
	Skip              []string              `json:"skip"`
	Aliases           map[string]AliasEntry `json:"aliases"`
	MultiSection      []MultiSectionEntry   `json:"multi_section"`
	FlattenTransforms []FlattenTransform    `json:"flatten_transforms"`
	MapKeyedBy        map[string]string     `json:"map_keyed_by"`
	InjectWrappers    []InjectWrapper       `json:"inject_wrappers"`
	ManualSections    []string              `json:"manual_sections"`
	// FieldAliases maps a schema key to a different Alloy attribute name (e.g.
	// camelCase "ownerKinds" → snake_case "owner_kinds"). The emitted YAML key
	// stays the schema key; only the Alloy tag changes.
	FieldAliases map[string]string `json:"field_aliases"`
	// PointerFields lists boolean fields (by "<GoType>.<schemaKey>") to emit as
	// *bool so an explicit false is distinguishable from unset — required for
	// fields whose Beyla default is true, which a plain bool could never disable.
	PointerFields []string `json:"pointer_fields"`
	// ExcludeTopLevel lists schema top-level keys that are intentionally NOT
	// exposed as Alloy config (Alloy-managed/injected, or internal/CLI). A
	// top-level key that is neither exposed (has an Arguments field) nor listed
	// here (nor manual/multi) fails generation.
	ExcludeTopLevel []string `json:"exclude_toplevel"`
	// EnumValidators maps a Go validator name to the schema location of an enum
	// (a "$def", "$def.property", or top-level property). The generator emits a
	// <name>Values list + valid<Name>() helper into validation_gen.go so the
	// accepted-value sets never drift from the schema. validation.go calls them.
	EnumValidators map[string]string `json:"enum_validators"`
	// ScalarTypes forces a schema $def to resolve to a scalar Go type instead of a
	// struct. Beyla exposes some enum/wrapper types (e.g. InstrumentableType) as
	// self-referential object defs carrying no usable schema, so the generator
	// can't infer they are strings; this maps the def name to its scalar Go type.
	ScalarTypes map[string]string `json:"scalar_types"`
}

type AliasEntry struct {
	AlloyKey string `json:"alloy_key"`
	Warn     bool   `json:"warn"`
}

type InjectWrapper struct {
	SchemaPath  string   `json:"schema_path"`
	SchemaKey   string   `json:"schema_key"`
	AlloyFields []string `json:"alloy_fields"`
}

// ── Schema types ──────────────────────────────────────────────────────────────

type schemaProp struct {
	Ref                  string                 `json:"$ref"`
	Type                 interface{}            `json:"type"`
	Enum                 []string               `json:"enum"`
	Description          string                 `json:"description"`
	Properties           map[string]*schemaProp `json:"properties"`
	Items                *schemaProp            `json:"items"`
	AllOf                []*schemaProp          `json:"allOf"`
	AnyOf                []*schemaProp          `json:"anyOf"`
	OneOf                []*schemaProp          `json:"oneOf"`
	Pattern              string                 `json:"pattern"`
	AdditionalProperties interface{}            `json:"additionalProperties"`
}

type schemaDoc struct {
	Defs       map[string]*schemaProp `json:"$defs"`
	Properties map[string]*schemaProp `json:"properties"`
}

// ── Go field types ────────────────────────────────────────────────────────────

type fieldDef struct {
	GoName  string
	TypeStr string // "string", "bool", "int", "time.Duration", "[]string",
	// "map[string]string", "*bool", "*int", "*time.Duration",
	// "Services", or a named struct type
}

// ── Sections ──────────────────────────────────────────────────────────────────

type section struct {
	FuncName   string // generated function name, e.g. "addRoutesConfig"
	ConfigKey  string // key in the YAML config map, e.g. "routes"
	DefName    string // schema $defs key, e.g. "RoutesConfig"
	ArgsExpr   string // Go expression for the top-level field, e.g. "b.args.Routes"
	StructName string // Go struct name, e.g. "Routes"
}

// ── Runtime-computed globals (derived from schema + mappings, never hard-coded) ──

var (
	sections      []section
	migratedTypes map[string]bool // Go type + schema def match → migrated to args_gen.go
	keepTypes     map[string]bool // Go type + no schema def → stays in args.go
	defToGo       map[string]string
	goToDef       map[string]string
	flatIdx       map[string]FlattenTransform // go_slice_type → FlattenTransform
	fieldAliasIdx map[string]string           // schema key → Alloy attr name
	pointerSet    map[string]bool             // full schema path → emit bool as *bool
	scalarTypeIdx map[string]string           // schema def name → forced scalar Go type
)

// aliasField returns the Alloy attribute name for a schema key, applying
// field_aliases (defaults to the schema key itself).
func aliasField(schemaKey string) string {
	if a, ok := fieldAliasIdx[schemaKey]; ok {
		return a
	}
	return schemaKey
}

// pointerizeBool upgrades a "bool" type to "*bool" when fieldKey ("<GoType>.<schemaKey>")
// is opted in via pointer_fields, so an explicit false survives to the generated YAML.
func pointerizeBool(fieldKey, typeStr string) string {
	if typeStr == "bool" && pointerSet[fieldKey] {
		return "*bool"
	}
	return typeStr
}

// allStructs holds all parsed struct types from Go source files.
// outer key: struct name; inner key: alloy tag → fieldDef.
var allStructs map[string]map[string]fieldDef

// namedSlices maps named slice type names to their element type names.
// e.g. "AttributeFamilies" → "AttributeFamily", "Services" → "Service"
var namedSlices = map[string]string{}

// flatBySchemaType maps a flatten_transform's schema_type to its transform, so a
// schema array of that type resolves to the named go_slice_type and the element
// type maps to go_item_type (e.g. GlobAttributes → Service, arrays → Services).
var flatBySchemaType = map[string]FlattenTransform{}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		die(err)
	}

	argsFile := filepath.Join(cwd, "args.go")
	argsGenFile := filepath.Join(cwd, "args_gen.go")
	schemaFile := filepath.Join(cwd, "gen", "schema.json")
	mappingsFile := filepath.Join(cwd, "gen", "mappings.json")
	outConfigGen := filepath.Join(cwd, "config_gen.go")
	outArgsGen := filepath.Join(cwd, "args_gen.go")
	outExamples := filepath.Join(cwd, "gen", "mappings_examples.md")
	outValidationGen := filepath.Join(cwd, "validation_gen.go")
	outSample := filepath.Join(cwd, "gen", "sample_config.alloy")

	// Parse all Go source files to build allStructs.
	allStructs, err = parseGoFiles(argsFile, argsGenFile)
	if err != nil {
		die(fmt.Errorf("parse Go files: %w", err))
	}

	sc, err := readSchema(schemaFile)
	if err != nil {
		die(fmt.Errorf("read schema: %w", err))
	}

	mappings, err := loadMappings(mappingsFile)
	if err != nil {
		die(fmt.Errorf("read mappings: %w", err))
	}

	// Build runtime globals from schema + mappings.
	defToGo, goToDef = buildNameOverrides(mappings)
	flatIdx = buildFlattenIndex(mappings)
	flatBySchemaType = make(map[string]FlattenTransform, len(mappings.FlattenTransforms))
	for _, ft := range mappings.FlattenTransforms {
		flatBySchemaType[ft.SchemaType] = ft
		for _, st := range ft.ExtraSchemaTypes {
			flatBySchemaType[st] = ft
		}
	}
	fieldAliasIdx = mappings.FieldAliases
	scalarTypeIdx = mappings.ScalarTypes
	pointerSet = make(map[string]bool, len(mappings.PointerFields))
	for _, p := range mappings.PointerFields {
		pointerSet[p] = true
	}
	migratedTypes, keepTypes = buildTypeClassification(allStructs, sc.Defs, goToDef)
	sections = detectSections(sc, mappings)

	if err := validateMappings(sc, mappings); err != nil {
		die(fmt.Errorf("invalid mappings.json: %w", err))
	}

	if err := checkTopLevelCoverage(sc, mappings); err != nil {
		die(err)
	}

	skipSet := buildSkipSet(mappings)
	iwIdx := buildInjectWrapperIndex(mappings)

	// Read args.go source for rewriting.
	argsFileSrc, err := os.ReadFile(argsFile)
	if err != nil {
		die(fmt.Errorf("read args.go: %w", err))
	}

	// Generate args_gen.go.
	argsGenSrc, generatedSet, err := generateArgsGen(sc, skipSet, iwIdx)
	if err != nil {
		die(fmt.Errorf("generate args_gen.go: %w", err))
	}

	// Rewrite args.go: remove only types that were actually generated.
	newArgsSrc, err := rewriteArgsGo(argsFileSrc, generatedSet)
	if err != nil {
		die(fmt.Errorf("rewrite args.go: %w", err))
	}

	// Re-parse allStructs from updated sources for config gen.
	allStructs, err = parseGoSource(newArgsSrc, argsGenSrc)
	if err != nil {
		die(fmt.Errorf("re-parse structs: %w", err))
	}

	// Recompute classification after re-parse.
	migratedTypes, keepTypes = buildTypeClassification(allStructs, sc.Defs, goToDef)

	// Generate beyla_config_gen.go.
	configGenSrc, err := generateConfigGen(sc, skipSet, iwIdx, mappings)
	if err != nil {
		die(fmt.Errorf("generate config gen: %w", err))
	}

	if err := os.WriteFile(outArgsGen, argsGenSrc, 0644); err != nil {
		die(err)
	}
	if err := os.WriteFile(argsFile, newArgsSrc, 0644); err != nil {
		die(err)
	}
	if err := os.WriteFile(outConfigGen, configGenSrc, 0644); err != nil {
		die(err)
	}

	examplesSrc := generateMappingsExamples(mappings, sc)
	if err := os.WriteFile(outExamples, examplesSrc, 0644); err != nil {
		die(err)
	}

	// Generate validation_gen.go (enum value-sets from the schema).
	validationGenSrc, err := generateValidationGen(sc, mappings)
	if err != nil {
		die(fmt.Errorf("generate validation gen: %w", err))
	}
	if err := os.WriteFile(outValidationGen, validationGenSrc, 0644); err != nil {
		die(err)
	}

	if err := os.WriteFile(outSample, generateSampleConfig(sc), 0644); err != nil {
		die(err)
	}

	fmt.Println("Generated", outArgsGen)
	fmt.Println("Updated", argsFile)
	fmt.Println("Generated", outConfigGen)
	fmt.Println("Generated", outExamples)
	fmt.Println("Generated", outValidationGen)
	fmt.Println("Generated", outSample)
}

// ── Mappings ──────────────────────────────────────────────────────────────────

func loadMappings(path string) (*Mappings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Mappings
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// buildNameOverrides creates bidirectional override maps from name_overrides.
func buildNameOverrides(m *Mappings) (d2g, g2d map[string]string) {
	d2g = make(map[string]string, len(m.NameOverrides))
	g2d = make(map[string]string, len(m.NameOverrides))
	for defName, goName := range m.NameOverrides {
		d2g[defName] = goName
		g2d[goName] = defName
	}
	return
}

// buildTypeClassification classifies all parsed Go types as migrated or keep.
func buildFlattenIndex(m *Mappings) map[string]FlattenTransform {
	idx := make(map[string]FlattenTransform)
	for _, ft := range m.FlattenTransforms {
		idx[ft.GoSliceType] = ft
	}
	return idx
}

// migrated: Go type has a matching schema $def that is a named struct (has properties).
// keep: Go type has no matching schema $def, or the def is a non-struct (e.g. map type).
func buildTypeClassification(structs map[string]map[string]fieldDef, defs map[string]*schemaProp, g2d map[string]string) (migrated, keep map[string]bool) {
	migrated = make(map[string]bool)
	keep = make(map[string]bool)
	for goName := range structs {
		defName := goName
		if d, ok := g2d[goName]; ok {
			defName = d
		}
		if def, exists := defs[defName]; exists && resolveProps(defs, def) != nil {
			migrated[goName] = true
		} else {
			keep[goName] = true
		}
	}
	return
}

// detectSections auto-detects schema sections from schema.properties × Arguments alloy tags.
// Any schema top-level property that has a matching field in the Arguments struct becomes a
// section. Multi_section yaml_sections and manual_sections are excluded.
func detectSections(sc *schemaDoc, mappings *Mappings) []section {
	excluded := make(map[string]bool)
	for _, s := range mappings.ManualSections {
		excluded[s] = true
	}
	for _, ms := range mappings.MultiSection {
		excluded[ms.YamlSection] = true
		// Exclude the top-level alloy_path component (e.g. "metrics" from "metrics.network").
		topAlloyPath := strings.SplitN(ms.AlloyPath, ".", 2)[0]
		excluded[topAlloyPath] = true
	}

	// Top-level alias map: schema key → alloy tag.
	schemaToAlloy := make(map[string]string)
	for schemaKey, entry := range mappings.Aliases {
		if !strings.Contains(schemaKey, ".") {
			schemaToAlloy[schemaKey] = entry.AlloyKey
		}
	}

	argumentsFields := allStructs["Arguments"]

	var secs []section
	for _, schemaKey := range sortedSchemaKeys(sc.Properties) {
		if excluded[schemaKey] {
			continue
		}
		alloyTag := schemaKey
		if alias, ok := schemaToAlloy[schemaKey]; ok {
			alloyTag = alias
		}
		fd, ok := argumentsFields[alloyTag]
		if !ok || !isBlockType(fd.TypeStr) {
			continue
		}
		prop := sc.Properties[schemaKey]
		defName := ""
		if prop != nil && prop.Ref != "" {
			defName = strings.TrimPrefix(prop.Ref, "#/$defs/")
		}
		goName := fd.TypeStr
		if override := schemaGoName(defName); override != defName && override != "" {
			goName = override
		}
		secs = append(secs, section{
			FuncName:   "add" + toPascalCase(schemaKey) + "Config",
			ConfigKey:  schemaKey,
			DefName:    defName,
			ArgsExpr:   "b.args." + fd.GoName,
			StructName: goName,
		})
	}
	return secs
}

func validateMappings(sc *schemaDoc, m *Mappings) error {
	var errs []string
	for _, entry := range m.Skip {
		parts := strings.Split(entry, ".")
		if !schemaPathExists(sc, parts) {
			errs = append(errs, fmt.Sprintf("skip %q: schema path not found", entry))
		}
	}
	for _, iw := range m.InjectWrappers {
		parts := append(strings.Split(iw.SchemaPath, "."), iw.SchemaKey)
		if !schemaPathExists(sc, parts) {
			errs = append(errs, fmt.Sprintf("inject_wrappers path=%q key=%q: not found in schema", iw.SchemaPath, iw.SchemaKey))
		}
	}
	for defName := range m.ScalarTypes {
		if _, ok := sc.Defs[defName]; !ok {
			errs = append(errs, fmt.Sprintf("scalar_types %q: not a schema $def (renamed or removed in Beyla?)", defName))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// checkTopLevelCoverage enforces expose-by-default-or-classify: every schema
// top-level property must be either exposed (has an Arguments field), handled
// by a manual/multi section, or explicitly listed in exclude_toplevel. Any
// top-level that is none of these fails generation.
func checkTopLevelCoverage(sc *schemaDoc, m *Mappings) error {
	classified := make(map[string]bool)
	for _, k := range m.ExcludeTopLevel {
		classified[k] = true
	}
	for _, k := range m.ManualSections {
		classified[k] = true
	}
	for _, ms := range m.MultiSection {
		classified[ms.YamlSection] = true
		classified[strings.SplitN(ms.AlloyPath, ".", 2)[0]] = true
	}
	// Validate exclude_toplevel entries actually exist (catch typos / renames).
	var errs []string
	for _, k := range m.ExcludeTopLevel {
		if _, ok := sc.Properties[k]; !ok {
			errs = append(errs, fmt.Sprintf("exclude_toplevel %q: not a schema top-level property", k))
		}
	}

	// Top-level alias map (e.g. schema "filter" → alloy "filters").
	schemaToAlloy := make(map[string]string)
	for schemaKey, entry := range m.Aliases {
		if !strings.Contains(schemaKey, ".") {
			schemaToAlloy[schemaKey] = entry.AlloyKey
		}
	}
	argsFields := allStructs["Arguments"]

	var missing []string
	for _, key := range sortedSchemaKeys(sc.Properties) {
		if classified[key] {
			continue
		}
		alloyTag := key
		if a, ok := schemaToAlloy[key]; ok {
			alloyTag = a
		}
		if _, ok := argsFields[alloyTag]; ok {
			continue // exposed
		}
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		errs = append(errs, fmt.Sprintf("unclassified schema top-level(s) %v: expose each (add an Arguments field) "+
			"or add it to exclude_toplevel in mappings.json", missing))
	}
	if len(errs) > 0 {
		return fmt.Errorf("top-level coverage: %s", strings.Join(errs, "; "))
	}
	return nil
}

func schemaPathExists(sc *schemaDoc, parts []string) bool {
	if len(parts) < 2 {
		return false
	}
	sectionKey := parts[0]
	var sectionDef *schemaProp
	for _, sec := range sections {
		if sec.ConfigKey == sectionKey {
			sectionDef = sc.Defs[sec.DefName]
			break
		}
	}
	if sectionDef == nil {
		return false
	}
	current := resolveProps(sc.Defs, sectionDef)
	for i := 1; i < len(parts); i++ {
		prop, ok := current[parts[i]]
		if !ok {
			return false
		}
		if i < len(parts)-1 {
			resolved := resolveRef(sc.Defs, prop)
			if resolved == nil {
				return false
			}
			current = resolveProps(sc.Defs, resolved)
			if current == nil {
				return false
			}
		}
	}
	return true
}

func buildSkipSet(m *Mappings) map[string]bool {
	s := make(map[string]bool, len(m.Skip))
	for _, e := range m.Skip {
		s[e] = true
	}
	return s
}

func buildInjectWrapperIndex(m *Mappings) map[string]*InjectWrapper {
	idx := make(map[string]*InjectWrapper, len(m.InjectWrappers))
	for i := range m.InjectWrappers {
		iw := &m.InjectWrappers[i]
		idx[iw.SchemaPath+"."+iw.SchemaKey] = iw
	}
	return idx
}

// ── args_gen.go generation ────────────────────────────────────────────────────

const argsGenHeader = `// Code generated by internal/component/beyla/ebpf/internal/config/gen/main.go; DO NOT EDIT.

package config

import "time"

`

// genCtx holds generation state threaded through recursive struct generation.
type genCtx struct {
	sc        *schemaDoc
	skipSet   map[string]bool
	iwIdx     map[string]*InjectWrapper
	generated map[string]bool
	sb        *strings.Builder
}

func generateArgsGen(sc *schemaDoc, skipSet map[string]bool, iwIdx map[string]*InjectWrapper) ([]byte, map[string]bool, error) {
	var sb strings.Builder
	sb.WriteString(argsGenHeader)
	ctx := &genCtx{sc: sc, skipSet: skipSet, iwIdx: iwIdx, generated: make(map[string]bool), sb: &sb}

	// Generate all section types (sub-types are generated on-demand via ensureSubType).
	for _, sec := range sections {
		if migratedTypes[sec.StructName] {
			ctx.generateMigratedType(sec.StructName, sec.DefName, sec.ConfigKey)
		} else if sec.DefName != "" {
			ctx.generateFreshType(sec.StructName, sec.DefName, sec.ConfigKey)
		}
	}

	src, err := format.Source([]byte(sb.String()))
	if err != nil {
		return []byte(sb.String()), ctx.generated, fmt.Errorf("format: %w\n\nsource:\n%s", err, sb.String())
	}
	return src, ctx.generated, nil
}

// generateFreshType writes a brand-new struct derived entirely from a schema def.
func (ctx *genCtx) generateFreshType(goName, defName, schemaPath string) {
	if ctx.generated[goName] || keepTypes[goName] {
		return
	}
	if _, ok := namedSlices[goName]; ok {
		return // hand-written named slice (e.g. Services); never generated
	}
	ctx.generated[goName] = true

	def := ctx.sc.Defs[defName]
	if def == nil {
		return
	}
	props := resolveProps(ctx.sc.Defs, def)
	if props == nil {
		return
	}

	// Pre-generate all block sub-types BEFORE writing this struct.
	for _, key := range sortedSchemaKeys(props) {
		if schemaPath != "" && ctx.skipSet[schemaPath+"."+key] {
			continue
		}
		_, ts, isBlk := ctx.deriveField(key, props[key])
		if isBlk && ts != "" {
			ctx.ensureSubType(ts, "")
		}
	}

	fmt.Fprintf(ctx.sb, "type %s struct {\n", goName)
	for _, key := range sortedSchemaKeys(props) {
		fullKey := schemaPath + "." + key
		if schemaPath != "" && ctx.skipSet[fullKey] {
			continue
		}
		prop := props[key]
		gn, ts, isBlk := ctx.deriveField(key, prop)
		if ts == "" {
			continue
		}
		ts = pointerizeBool(goName+"."+key, ts)
		tag := "attr"
		if isBlk {
			tag = "block"
		}
		fmt.Fprintf(ctx.sb, "\t%s %s `alloy:\"%s,%s,optional\"`\n", gn, ts, aliasField(key), tag)
	}
	fmt.Fprintln(ctx.sb, "}")
	fmt.Fprintln(ctx.sb)
}

// generateMigratedType writes a "migrated" type — one with both a hand-written
// definition and a schema $def — merging the existing fields with the schema
// fields and tracking schema path for inject_wrappers.
func (ctx *genCtx) generateMigratedType(goName, defName, schemaPath string) {
	if ctx.generated[goName] || keepTypes[goName] {
		return
	}
	if _, ok := namedSlices[goName]; ok {
		return // hand-written named slice (e.g. Services); never generated
	}
	ctx.generated[goName] = true

	def := ctx.sc.Defs[defName]
	if def == nil {
		return
	}
	props := resolveProps(ctx.sc.Defs, def)
	existingFields := allStructs[goName]

	type outField struct {
		alloyTag string
		goName   string
		typeStr  string
		isBlk    bool
	}
	var fields []outField
	covered := make(map[string]bool)

	// Pre-pass: collect all block sub-types and generate them BEFORE this struct.
	for _, key := range sortedSchemaKeys(props) {
		fullKey := schemaPath + "." + key
		if ctx.skipSet[fullKey] {
			continue
		}
		if iw, ok := ctx.iwIdx[fullKey]; ok {
			for _, af := range iw.AlloyFields {
				if fd, ok := existingFields[af]; ok {
					if isBlockType(fd.TypeStr) {
						ctx.ensureSubType(fd.TypeStr, schemaPath+"."+af)
					}
				} else {
					_, ts, isBlk := ctx.deriveInjectWrapperField(af, iw)
					if isBlk && ts != "" {
						ctx.ensureSubType(ts, "")
					}
				}
			}
			continue
		}
		prop := props[key]
		if fd, ok := existingFields[key]; ok {
			if isBlockType(fd.TypeStr) {
				ctx.ensureSubType(fd.TypeStr, fullKey)
			}
		} else {
			_, ts, isBlk := ctx.deriveField(key, prop)
			if isBlk && ts != "" {
				ctx.ensureSubType(ts, "")
			}
		}
	}

	// Pass 1: schema-driven walk.
	for _, key := range sortedSchemaKeys(props) {
		fullKey := schemaPath + "." + key
		if ctx.skipSet[fullKey] {
			covered[key] = true
			continue
		}

		// Check for inject wrapper: replace this schema key with alloy fields.
		if iw, ok := ctx.iwIdx[fullKey]; ok {
			for _, af := range iw.AlloyFields {
				if covered[af] {
					continue
				}
				covered[af] = true
				if fd, ok := existingFields[af]; ok {
					fields = append(fields, outField{af, fd.GoName, fd.TypeStr, isBlockType(fd.TypeStr)})
				} else {
					gn, ts, isBlk := ctx.deriveInjectWrapperField(af, iw)
					if ts != "" {
						fields = append(fields, outField{af, gn, ts, isBlk})
					}
				}
			}
			covered[key] = true
			continue
		}

		prop := props[key]
		alloyKey := aliasField(key)
		if fd, ok := existingFields[alloyKey]; ok {
			goNm, typeStr, blk := fd.GoName, fd.TypeStr, isBlockType(fd.TypeStr)
			// Re-derive from the schema when the carried-over type points at a $def
			// the schema dropped (a renamed type would dangle) or at a flatten type
			// (so the flatten applies, e.g. []GlobAttributes → Services).
			if ctx.isDanglingType(typeStr) || ctx.refsFlattenType(typeStr) {
				if gn, ts, isBlk := ctx.deriveField(key, prop); ts != "" {
					goNm, typeStr, blk = gn, ts, isBlk
				}
			}
			covered[alloyKey] = true
			fields = append(fields, outField{alloyKey, goNm, pointerizeBool(goName+"."+key, typeStr), blk})
		} else {
			gn, ts, isBlk := ctx.deriveField(key, prop)
			if ts != "" {
				covered[alloyKey] = true
				fields = append(fields, outField{alloyKey, gn, pointerizeBool(goName+"."+key, ts), isBlk})
			}
		}
	}

	// Pass 2: existing alloy fields not covered by schema (preserve them).
	for _, key := range sortedFieldKeys(existingFields) {
		if covered[key] || covered[aliasField(key)] {
			continue
		}
		fd := existingFields[key]
		// Skip struct fields whose type is neither migrated nor kept — they belong
		// to hand-written sections that stay in args.go.
		if isBlockType(fd.TypeStr) && !migratedTypes[fd.TypeStr] && !keepTypes[fd.TypeStr] {
			continue
		}
		fields = append(fields, outField{key, fd.GoName, fd.TypeStr, isBlockType(fd.TypeStr)})
	}

	// Write the struct.
	fmt.Fprintf(ctx.sb, "type %s struct {\n", goName)
	for _, f := range fields {
		tag := "attr"
		if f.isBlk {
			tag = "block"
		}
		fmt.Fprintf(ctx.sb, "\t%s %s `alloy:\"%s,%s,optional\"`\n", f.goName, f.typeStr, aliasField(f.alloyTag), tag)
	}
	fmt.Fprintln(ctx.sb, "}")
	fmt.Fprintln(ctx.sb)
}

// ensureSubType generates a sub-type if not yet done.
// schemaPath is used for migrated types to enable skip/inject_wrapper lookups.
func (ctx *genCtx) ensureSubType(goName, schemaPath string) {
	goName = strings.TrimPrefix(goName, "[]") // slice-of-struct fields carry their element type
	if ctx.generated[goName] || keepTypes[goName] {
		return
	}
	if _, ok := flatBySchemaType[goName]; ok {
		return // flatten type, represented by a hand-written go_item_type
	}
	if _, ok := namedSlices[goName]; ok {
		return // hand-written named slice (e.g. Services)
	}
	defName := goNameToSchemaDef(goName)
	def := ctx.sc.Defs[defName]
	if def == nil {
		return
	}
	if migratedTypes[goName] {
		ctx.generateMigratedType(goName, defName, schemaPath)
	} else {
		ctx.generateFreshType(goName, defName, schemaPath)
	}
}

// isDanglingType reports whether a (possibly slice) block type refers to a schema
// $def that no longer exists — e.g. a type renamed or removed on a Beyla bump.
// A field carried over from the previously generated args_gen.go with such a type
// would not compile, so it must be re-derived from the current schema.
func (ctx *genCtx) isDanglingType(ts string) bool {
	elem := strings.TrimPrefix(ts, "[]")
	if !isBlockType(elem) || keepTypes[elem] || migratedTypes[elem] {
		return false
	}
	if _, ok := namedSlices[elem]; ok {
		return false // a hand-written named slice (e.g. Services), not dangling
	}
	_, ok := ctx.sc.Defs[goNameToSchemaDef(elem)]
	return !ok
}

// refsFlattenType reports whether a (possibly slice) type's element is a
// flatten_transform schema_type. Such a carried-over field is re-derived so the
// flatten applies (e.g. []GlobAttributes → Services).
func (ctx *genCtx) refsFlattenType(ts string) bool {
	elem := strings.TrimPrefix(ts, "[]")
	_, ok := flatBySchemaType[goNameToSchemaDef(elem)]
	return ok
}

// flattenSlice returns the named go_slice_type when an array's items are a
// flatten_transform schema_type (e.g. items of GlobAttributes → Services).
func (ctx *genCtx) flattenSlice(items *schemaProp) (string, bool) {
	if items == nil || items.Ref == "" {
		return "", false
	}
	if ft, ok := flatBySchemaType[strings.TrimPrefix(items.Ref, "#/$defs/")]; ok {
		return ft.GoSliceType, true
	}
	return "", false
}

// resolveDefType resolves a schema $def (by name) to a Go type string and whether
// it is a block (struct) type. It handles string-typed defs (→ string), array-typed
// defs (→ []elem), and object defs (→ the struct type). Returns "" if unresolvable.
func (ctx *genCtx) resolveDefType(defName string) (typeStr string, isBlock bool) {
	goName := schemaGoName(defName)
	if keepTypes[goName] || migratedTypes[goName] {
		return goName, true
	}
	// A scalar_types override short-circuits defs whose schema is unusable (e.g.
	// InstrumentableType, a self-referential enum wrapper) to their real Go type.
	if st, ok := scalarTypeIdx[defName]; ok {
		return st, false
	}
	// A flatten_transform schema_type is represented by its hand-written go_item_type
	// (e.g. GlobAttributes → Service); arrays of it become the go_slice_type below.
	if ft, ok := flatBySchemaType[defName]; ok {
		return ft.GoItemType, true
	}
	def := ctx.sc.Defs[defName]
	if def == nil {
		return "", false
	}
	switch t, _ := def.Type.(string); t {
	case "string":
		if isDurationPattern(def.Pattern) {
			return "time.Duration", false
		}
		return "string", false
	case "boolean":
		return "bool", false
	case "integer":
		return "int", false
	case "array":
		if st, ok := ctx.flattenSlice(def.Items); ok {
			return st, true
		}
		if it, _ := ctx.resolveItemType(def.Items); it != "" {
			_, ib := ctx.resolveItemType(def.Items)
			return "[]" + it, ib
		}
		return "", false
	}
	if resolveProps(ctx.sc.Defs, def) != nil {
		return goName, true
	}
	return "", false
}

// resolveItemType resolves an array's item schema to a Go type string + block-ness.
func (ctx *genCtx) resolveItemType(items *schemaProp) (string, bool) {
	if items == nil {
		return "", false
	}
	if items.Ref != "" {
		return ctx.resolveDefType(strings.TrimPrefix(items.Ref, "#/$defs/"))
	}
	if t, _ := items.Type.(string); t == "string" {
		return "string", false
	}
	return "", false
}

// deriveField derives (GoName, TypeStr, isBlock) from a schema property key+prop.
func (ctx *genCtx) deriveField(schemaKey string, prop *schemaProp) (string, string, bool) {
	goName := toPascalCase(schemaKey)
	if prop == nil {
		return goName, "string", false
	}

	if prop.Ref != "" {
		defName := strings.TrimPrefix(prop.Ref, "#/$defs/")
		ts, isBlk := ctx.resolveDefType(defName)
		if ts == "" {
			return "", "", false
		}
		return goName, ts, isBlk
	}

	typeStr, _ := prop.Type.(string)
	switch typeStr {
	case "boolean":
		return goName, "bool", false
	case "integer":
		return goName, "int", false
	case "string":
		if isDurationPattern(prop.Pattern) {
			return goName, "time.Duration", false
		}
		return goName, "string", false
	case "array":
		if st, ok := ctx.flattenSlice(prop.Items); ok {
			return goName, st, true
		}
		if it, ib := ctx.resolveItemType(prop.Items); it != "" {
			return goName, "[]" + it, ib
		}
		return "", "", false
	case "object":
		if m, ok := prop.AdditionalProperties.(map[string]interface{}); ok {
			if m["type"] == "string" {
				return goName, "map[string]string", false
			}
			if ref, ok := m["$ref"].(string); ok {
				if rt, isBlk := ctx.resolveDefType(strings.TrimPrefix(ref, "#/$defs/")); rt == "string" && !isBlk {
					return goName, "map[string]string", false
				}
			}
		}
		return "", "", false
	}
	return "", "", false
}

// deriveInjectWrapperField derives the field info for a new inject_wrapper alloy field.
func (ctx *genCtx) deriveInjectWrapperField(alloyField string, iw *InjectWrapper) (string, string, bool) {
	iwSchemaPath := strings.Split(iw.SchemaPath, ".")
	if len(iwSchemaPath) == 0 {
		return toPascalCase(alloyField), "", false
	}
	sectionKey := iwSchemaPath[0]
	var def *schemaProp
	for _, sec := range sections {
		if sec.ConfigKey == sectionKey {
			def = ctx.sc.Defs[sec.DefName]
			break
		}
	}
	if def == nil {
		return toPascalCase(alloyField), "", false
	}
	current := resolveProps(ctx.sc.Defs, def)
	for _, part := range iwSchemaPath[1:] {
		prop := current[part]
		if prop == nil {
			return toPascalCase(alloyField), "", false
		}
		resolved := resolveRef(ctx.sc.Defs, prop)
		if resolved == nil {
			return toPascalCase(alloyField), "", false
		}
		current = resolveProps(ctx.sc.Defs, resolved)
	}
	wrapperProp := current[iw.SchemaKey]
	if wrapperProp == nil {
		return toPascalCase(alloyField), "", false
	}
	wrapperDef := resolveRef(ctx.sc.Defs, wrapperProp)
	if wrapperDef == nil {
		return toPascalCase(alloyField), "", false
	}
	wrapperProps := resolveProps(ctx.sc.Defs, wrapperDef)
	if wrapperProps == nil {
		return toPascalCase(alloyField), "", false
	}
	innerProp := wrapperProps[alloyField]
	return ctx.deriveField(alloyField, innerProp)
}

func isDurationPattern(p string) bool {
	return strings.Contains(p, "(ms|s|m)")
}

// schemaGoName derives the Go type name from a schema def name using name_overrides.
func schemaGoName(defName string) string {
	if n, ok := defToGo[defName]; ok {
		return n
	}
	return defName
}

// goNameToSchemaDef derives the schema def name from a Go type name using name_overrides.
func goNameToSchemaDef(goName string) string {
	if n, ok := goToDef[goName]; ok {
		return n
	}
	return goName
}

func isBlockType(ts string) bool {
	switch ts {
	case "", "string", "bool", "int", "time.Duration",
		"[]string", "map[string]string",
		"*bool", "*int", "*time.Duration":
		return false
	}
	return true
}

func isStructOrServices(ts string) bool {
	if _, isFlatSlice := flatIdx[ts]; isFlatSlice {
		return true
	}
	return isBlockType(ts)
}

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

// ── args.go rewriting ─────────────────────────────────────────────────────────

func rewriteArgsGo(src []byte, generatedSet map[string]bool) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "args.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	type span struct{ start, end int }
	var removeSpans []span

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !generatedSet[ts.Name.Name] {
				continue
			}
			start := fset.Position(genDecl.Pos()).Offset
			if genDecl.Doc != nil {
				docStart := fset.Position(genDecl.Doc.Pos()).Offset
				if docStart < start {
					start = docStart
				}
			}
			end := fset.Position(genDecl.End()).Offset
			removeSpans = append(removeSpans, span{start, end})
		}
	}

	if len(removeSpans) == 0 {
		return format.Source(src)
	}

	sort.Slice(removeSpans, func(i, j int) bool {
		return removeSpans[i].start < removeSpans[j].start
	})

	var buf bytes.Buffer
	pos := 0
	for _, sp := range removeSpans {
		if sp.start > pos {
			buf.Write(src[pos:sp.start])
		}
		pos = sp.end
	}
	buf.Write(src[pos:])

	return format.Source(buf.Bytes())
}

// ── beyla_config_gen.go generation ───────────────────────────────────────────

const configGenHeader = `// Code generated by internal/component/beyla/ebpf/internal/config/gen/main.go; DO NOT EDIT.

package config

`

func generateConfigGen(sc *schemaDoc, skipSet map[string]bool, iwIdx map[string]*InjectWrapper, mappings *Mappings) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString(configGenHeader)

	// Generate full add* functions for direct schema sections.
	var dispatched []string
	for _, sec := range sections {
		def := sc.Defs[sec.DefName]
		if def == nil {
			fmt.Fprintf(&sb, "// TODO: schema def %q not found — %s not generated\n\n", sec.DefName, sec.FuncName)
			continue
		}
		props := resolveProps(sc.Defs, def)
		fields := allStructs[sec.StructName]
		genFunc(&sb, sec, fields, sc.Defs, props, sec.ConfigKey, skipSet, iwIdx)
		dispatched = append(dispatched, sec.FuncName)
	}

	// Dispatcher: calls every generated section translation, so adding a section
	// auto-wires it into build().
	fmt.Fprintf(&sb, "// addGeneratedConfig invokes every generated section translation.\n")
	fmt.Fprintf(&sb, "func (b *builder) addGeneratedConfig(config map[string]interface{}) {\n")
	for _, fn := range dispatched {
		fmt.Fprintf(&sb, "\tb.%s(config)\n", fn)
	}
	fmt.Fprintln(&sb, "}")
	fmt.Fprintln(&sb)

	// Generate fill helpers for multi_section entries.
	for _, ms := range mappings.MultiSection {
		var msProps map[string]*schemaProp
		if p, ok := sc.Properties[ms.YamlSection]; ok {
			if r := resolveRef(sc.Defs, p); r != nil {
				msProps = resolveProps(sc.Defs, r)
			}
		}
		genFillHelper(&sb, ms, mappings.Aliases, sc.Defs, msProps, skipSet, iwIdx)
	}

	// Generate map_keyed_by helpers.
	mkbKeys := make([]string, 0, len(mappings.MapKeyedBy))
	for k := range mappings.MapKeyedBy {
		mkbKeys = append(mkbKeys, k)
	}
	sort.Strings(mkbKeys)
	for _, typeName := range mkbKeys {
		keyField := mappings.MapKeyedBy[typeName]
		genMapKeyedByHelper(&sb, typeName, keyField)
	}

	// Generate flat-slice serializer functions (build<Type>YAML).
	for _, ft := range mappings.FlattenTransforms {
		genFlatSliceFunc(&sb, ft, sc.Defs, skipSet, iwIdx)
	}

	src, err := format.Source([]byte(sb.String()))
	if err != nil {
		return []byte(sb.String()), fmt.Errorf("format: %w\n\nsource:\n%s", err, sb.String())
	}
	return src, nil
}

func genFunc(sb *strings.Builder, sec section, fields map[string]fieldDef, defs map[string]*schemaProp, props map[string]*schemaProp, schemaPath string, skipSet map[string]bool, iwIdx map[string]*InjectWrapper) {
	fmt.Fprintf(sb, "// %s adds %s configuration.\nfunc (b *builder) %s(config map[string]interface{}) {\n",
		sec.FuncName, sec.ConfigKey, sec.FuncName)
	fmt.Fprintln(sb, "\tm := make(map[string]interface{})")
	genBlock(sb, "\t", "m", sec.ArgsExpr, fields, defs, props, 0, schemaPath, skipSet, iwIdx)
	fmt.Fprintf(sb, "\tif len(m) > 0 {\n\t\tconfig[%q] = m\n\t}\n", sec.ConfigKey)
	fmt.Fprintln(sb, "}")
	fmt.Fprintln(sb)
}

// genFillHelper generates a fill<YamlSection>Config helper for a multi_section entry.
// The helper populates an already-created map from the Alloy struct at alloy_path.
// Hand-written skeletons call this helper after handling lock/runtime-state/conditions.
func genFillHelper(sb *strings.Builder, ms MultiSectionEntry, aliases map[string]AliasEntry, defs map[string]*schemaProp, props map[string]*schemaProp, skipSet map[string]bool, iwIdx map[string]*InjectWrapper) {
	argsExpr, structName := alloyPathToExpr(ms.AlloyPath)
	if structName == "" {
		fmt.Fprintf(sb, "// TODO: fill helper for %q: alloy_path %q not resolvable\n\n", ms.YamlSection, ms.AlloyPath)
		return
	}
	fields := allStructs[structName]
	if fields == nil {
		fmt.Fprintf(sb, "// TODO: fill helper for %q: struct %q not found\n\n", ms.YamlSection, structName)
		return
	}

	// Build reverse alias map: for this yaml section, alloyField → yaml key.
	aliasYamlKey := make(map[string]string)
	for schemaKey, entry := range aliases {
		if strings.HasPrefix(schemaKey, ms.YamlSection+".") {
			yamlKey := strings.TrimPrefix(schemaKey, ms.YamlSection+".")
			aliasYamlKey[entry.AlloyKey] = yamlKey
		}
	}

	// Determine which alloy fields to emit.
	alloyFields := ms.AlloyFields
	if len(alloyFields) == 0 {
		alloyFields = sortedFieldKeys(fields)
	}

	funcName := "fill" + toPascalCase(ms.YamlSection) + "Config"
	fmt.Fprintf(sb, "// %s fills m with %s fields from %s.\n", funcName, ms.YamlSection, ms.AlloyPath)
	fmt.Fprintf(sb, "func (b *builder) %s(m map[string]interface{}) {\n", funcName)

	for _, alloyField := range alloyFields {
		fd, ok := fields[alloyField]
		if !ok {
			fmt.Fprintf(sb, "\t// TODO: alloy field %q not found in struct %s\n", alloyField, structName)
			continue
		}
		yamlKey := alloyField
		if alias, ok := aliasYamlKey[alloyField]; ok {
			yamlKey = alias
		}
		var prop *schemaProp
		if props != nil {
			prop = props[yamlKey]
		}
		genField(sb, "\t", "m", argsExpr, yamlKey, fd, defs, prop, 0, ms.YamlSection+"."+yamlKey, skipSet, iwIdx)
	}

	fmt.Fprintln(sb, "}")
	fmt.Fprintln(sb)
}

// alloyPathToExpr resolves a dot-separated alloy path (e.g. "metrics.network")
// to a Go access expression and the struct type at that path.
func alloyPathToExpr(alloyPath string) (expr string, typeName string) {
	parts := strings.Split(alloyPath, ".")
	expr = "b.args"
	typeName = "Arguments"
	for _, part := range parts {
		fields, ok := allStructs[typeName]
		if !ok {
			return "", ""
		}
		fd, ok := fields[part]
		if !ok {
			return "", ""
		}
		expr += "." + fd.GoName
		typeName = fd.TypeStr
	}
	return
}

// genMapKeyedByHelper generates a fill helper for a map_keyed_by type.
// The helper iterates the slice and builds a map keyed by the declared field.
// Signature: func fill<TypeName>Config(m map[string]interface{}, items <TypeName>)
func genMapKeyedByHelper(sb *strings.Builder, typeName, keyField string) {
	fields := allStructs[typeName]
	if fields == nil {
		// typeName may be a named slice (e.g. AttributeFamilies → AttributeFamily)
		elemType := namedSlices[typeName]
		if elemType != "" {
			fields = allStructs[elemType]
		}
		if fields == nil {
			fmt.Fprintf(sb, "// TODO: map_keyed_by helper for %q: struct not found\n\n", typeName)
			return
		}
	}
	keyFd, ok := fields[keyField]
	if !ok {
		fmt.Fprintf(sb, "// TODO: map_keyed_by helper for %q: key field %q not found\n\n", typeName, keyField)
		return
	}

	funcName := "fill" + typeName + "Config"
	fmt.Fprintf(sb, "// %s builds a YAML map keyed by the %q field.\n", funcName, keyField)
	fmt.Fprintf(sb, "func %s(m map[string]interface{}, items %s) {\n", funcName, typeName)
	fmt.Fprintln(sb, "\tfor _, item := range items {")
	fmt.Fprintln(sb, "\t\tsub := make(map[string]interface{})")

	for _, field := range sortedFieldKeys(fields) {
		if field == keyField {
			continue
		}
		fd := fields[field]
		emitLeaf(sb, "\t\t", "sub", field, "item."+fd.GoName, fd.TypeStr)
	}

	fmt.Fprintf(sb, "\t\tif len(sub) > 0 {\n\t\t\tm[item.%s] = sub\n\t\t}\n", keyFd.GoName)
	fmt.Fprintln(sb, "\t}")
	fmt.Fprintln(sb, "}")
	fmt.Fprintln(sb)
}

// ── validation_gen.go generation ─────────────────────────────────────────────

const validationGenHeader = `// Code generated by internal/component/beyla/ebpf/internal/config/gen/main.go; DO NOT EDIT.

package config

`

// generateValidationGen emits an enum value-set + membership helper per
// mappings.enum_validators entry, read straight from the schema so the accepted
// values can't drift from Beyla. validation.go calls valid<Name>() and uses
// <name>Values for its error messages.
func generateValidationGen(sc *schemaDoc, mappings *Mappings) ([]byte, error) {
	var sb strings.Builder
	sb.WriteString(validationGenHeader)

	names := make([]string, 0, len(mappings.EnumValidators))
	for name := range mappings.EnumValidators {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		loc := mappings.EnumValidators[name]
		values, err := resolveEnum(sc, loc)
		if err != nil {
			return nil, fmt.Errorf("enum_validators[%q] -> %q: %w", name, loc, err)
		}
		varName := lowerFirst(name) + "Values"
		fmt.Fprintf(&sb, "// %s are the values Beyla's schema accepts at %q.\n", varName, loc)
		fmt.Fprintf(&sb, "var %s = []string{\n", varName)
		for _, v := range values {
			fmt.Fprintf(&sb, "\t%q,\n", v)
		}
		fmt.Fprintln(&sb, "}")
		fmt.Fprintln(&sb)
		fmt.Fprintf(&sb, "// valid%s reports whether v is an accepted %q value.\n", name, loc)
		fmt.Fprintf(&sb, "func valid%s(v string) bool {\n", name)
		fmt.Fprintf(&sb, "\tfor _, x := range %s {\n\t\tif v == x {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n\n", varName)
	}

	return format.Source([]byte(sb.String()))
}

// resolveEnum returns the enum values at a schema location: "Def.property", a
// "$def" name, or a top-level property name. Arrays unwrap to their items' enum;
// a single $ref is followed.
func resolveEnum(sc *schemaDoc, loc string) ([]string, error) {
	var p *schemaProp
	if def, prop, ok := strings.Cut(loc, "."); ok {
		d := sc.Defs[def]
		if d == nil {
			return nil, fmt.Errorf("no $def %q", def)
		}
		p = resolveProps(sc.Defs, d)[prop]
		if p == nil {
			return nil, fmt.Errorf("$def %q has no property %q", def, prop)
		}
	} else if d, ok := sc.Defs[loc]; ok {
		p = d
	} else if prop, ok := sc.Properties[loc]; ok {
		p = prop
	} else {
		return nil, fmt.Errorf("not a $def or top-level property")
	}
	v := enumOf(sc, p)
	if len(v) == 0 {
		return nil, fmt.Errorf("no enum at location")
	}
	return v, nil
}

// enumOf reads the enum off a prop, unwrapping array items and following a $ref.
func enumOf(sc *schemaDoc, p *schemaProp) []string {
	if p == nil {
		return nil
	}
	if len(p.Enum) > 0 {
		return p.Enum
	}
	if p.Items != nil {
		if len(p.Items.Enum) > 0 {
			return p.Items.Enum
		}
		if p.Items.Ref != "" {
			return enumOf(sc, resolveRef(sc.Defs, p.Items))
		}
	}
	if p.Ref != "" {
		return enumOf(sc, resolveRef(sc.Defs, p))
	}
	return nil
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// genFlatSliceFunc generates a build<GoSliceType>YAML function that serializes a
// slice of structs, flattening one nested block into the parent map with a prefix.
func genFlatSliceFunc(sb *strings.Builder, ft FlattenTransform, defs map[string]*schemaProp, skipSet map[string]bool, iwIdx map[string]*InjectWrapper) {
	itemFields := allStructs[ft.GoItemType]
	if itemFields == nil {
		fmt.Fprintf(sb, "// TODO: build%sYAML: item type %q not found in allStructs\n\n", ft.GoSliceType, ft.GoItemType)
		return
	}

	funcName := "build" + ft.GoSliceType + "YAML"
	fmt.Fprintf(sb, "func %s(items %s) []map[string]interface{} {\n", funcName, ft.GoSliceType)
	fmt.Fprintf(sb, "\tresult := make([]map[string]interface{}, 0, len(items))\n")
	fmt.Fprintf(sb, "\tfor _, item := range items {\n")
	fmt.Fprintf(sb, "\t\tm := make(map[string]interface{})\n")

	for _, key := range sortedFieldKeys(itemFields) {
		fd := itemFields[key]
		if key == ft.AlloyBlock {
			subFields := allStructs[fd.TypeStr]
			if subFields != nil {
				for _, subKey := range sortedFieldKeys(subFields) {
					subFd := subFields[subKey]
					emitLeaf(sb, "\t\t", "m", ft.Prefix+subKey, "item."+fd.GoName+"."+subFd.GoName, subFd.TypeStr)
				}
			}
		} else {
			genField(sb, "\t\t", "m", "item", key, fd, defs, nil, 0, ft.GoItemType+"."+key, skipSet, iwIdx)
		}
	}

	fmt.Fprintf(sb, "\t\tif len(m) > 0 {\n\t\t\tresult = append(result, m)\n\t\t}\n")
	fmt.Fprintf(sb, "\t}\n")
	fmt.Fprintf(sb, "\treturn result\n")
	fmt.Fprintf(sb, "}\n\n")
}

func genBlock(sb *strings.Builder, indent, mapVar, argsExpr string, fields map[string]fieldDef, defs map[string]*schemaProp, props map[string]*schemaProp, depth int, schemaPath string, skipSet map[string]bool, iwIdx map[string]*InjectWrapper) {
	covered := make(map[string]bool)

	// Pass 1: schema-driven walk.
	for _, key := range sortedSchemaKeys(props) {
		fullKey := schemaPath + "." + key
		if skipSet[fullKey] {
			covered[key] = true
			continue
		}

		// Inject wrapper: replace schema key with wrapped alloy fields.
		if iw, ok := iwIdx[fullKey]; ok {
			emitInjectWrapper(sb, indent, mapVar, argsExpr, key, iw, fields, defs, depth, fullKey, skipSet, iwIdx)
			for _, af := range iw.AlloyFields {
				covered[af] = true
			}
			covered[key] = true
			continue
		}

		prop := props[key]
		alloyKey := aliasField(key)
		fd, ok := fields[alloyKey]
		if !ok {
			fmt.Fprintf(sb, "%s// TODO: schema field %q has no Alloy mapping\n", indent, key)
			continue
		}
		covered[key] = true
		covered[alloyKey] = true
		genField(sb, indent, mapVar, argsExpr, key, fd, defs, prop, depth, fullKey, skipSet, iwIdx)
	}

	// Pass 2: alloy fields not covered by schema.
	for _, key := range sortedFieldKeys(fields) {
		if covered[key] {
			continue
		}
		fd := fields[key]
		if isStructOrServices(fd.TypeStr) {
			fmt.Fprintf(sb, "%s// NOTE: alloy field %q (type %s) has no schema counterpart; add manually if needed\n", indent, key, fd.TypeStr)
			continue
		}
		genField(sb, indent, mapVar, argsExpr, key, fd, defs, nil, depth, schemaPath+"."+key, skipSet, iwIdx)
	}
}

func emitInjectWrapper(sb *strings.Builder, indent, mapVar, argsExpr, wrapKey string, iw *InjectWrapper, fields map[string]fieldDef, defs map[string]*schemaProp, depth int, fullKey string, skipSet map[string]bool, iwIdx map[string]*InjectWrapper) {
	subVar := fmt.Sprintf("m%d", depth+1)
	fmt.Fprintf(sb, "%s{\n%s\t%s := make(map[string]interface{})\n", indent, indent, subVar)
	for _, af := range iw.AlloyFields {
		fd, ok := fields[af]
		if !ok {
			continue
		}
		genField(sb, indent+"\t", subVar, argsExpr, af, fd, defs, nil, depth+1, fullKey+"."+af, skipSet, iwIdx)
	}
	fmt.Fprintf(sb, "%s\tif len(%s) > 0 {\n%s\t\t%s[%q] = %s\n%s\t}\n%s}\n",
		indent, subVar, indent, mapVar, wrapKey, subVar, indent, indent)
}

func genField(sb *strings.Builder, indent, mapVar, argsExpr, key string, fd fieldDef, defs map[string]*schemaProp, prop *schemaProp, depth int, fullKey string, skipSet map[string]bool, iwIdx map[string]*InjectWrapper) {
	expr := argsExpr + "." + fd.GoName

	switch {
	case flatIdx[fd.TypeStr].GoSliceType != "":
		ft := flatIdx[fd.TypeStr]
		funcName := "build" + ft.GoSliceType + "YAML"
		fmt.Fprintf(sb, "%sif len(%s) > 0 {\n%s\t%s[%q] = %s(%s)\n%s}\n",
			indent, expr, indent, mapVar, key, funcName, expr, indent)

	case isKnownStruct(fd.TypeStr):
		subVar := fmt.Sprintf("m%d", depth+1)
		subFields := allStructs[fd.TypeStr]
		var subProps map[string]*schemaProp
		if prop != nil {
			resolved := resolveRef(defs, prop)
			if resolved != nil {
				subProps = resolveProps(defs, resolved)
			}
		}
		fmt.Fprintf(sb, "%s{\n%s\t%s := make(map[string]interface{})\n", indent, indent, subVar)
		genBlock(sb, indent+"\t", subVar, expr, subFields, defs, subProps, depth+1, fullKey, skipSet, iwIdx)
		fmt.Fprintf(sb, "%s\tif len(%s) > 0 {\n%s\t\t%s[%q] = %s\n%s\t}\n%s}\n",
			indent, subVar, indent, mapVar, key, subVar, indent, indent)

	case sliceElemStruct(fd.TypeStr) != "":
		// Repeated block decoded into a slice (e.g. []K8sSelector) → list of maps.
		subFields := allStructs[sliceElemStruct(fd.TypeStr)]
		var itemProps map[string]*schemaProp
		if prop != nil {
			items := prop.Items
			if items == nil {
				if r := resolveRef(defs, prop); r != nil {
					items = r.Items
				}
			}
			if items != nil {
				if rr := resolveRef(defs, items); rr != nil {
					itemProps = resolveProps(defs, rr)
				}
			}
		}
		itemVar := fmt.Sprintf("item%d", depth+1)
		listVar := fmt.Sprintf("list%d", depth+1)
		subVar := fmt.Sprintf("m%d", depth+1)
		fmt.Fprintf(sb, "%s{\n%s\t%s := make([]map[string]interface{}, 0, len(%s))\n", indent, indent, listVar, expr)
		fmt.Fprintf(sb, "%s\tfor _, %s := range %s {\n", indent, itemVar, expr)
		fmt.Fprintf(sb, "%s\t\t%s := make(map[string]interface{})\n", indent, subVar)
		genBlock(sb, indent+"\t\t", subVar, itemVar, subFields, defs, itemProps, depth+1, fullKey, skipSet, iwIdx)
		fmt.Fprintf(sb, "%s\t\tif len(%s) > 0 {\n%s\t\t\t%s = append(%s, %s)\n%s\t\t}\n", indent, subVar, indent, listVar, listVar, subVar, indent)
		fmt.Fprintf(sb, "%s\t}\n", indent)
		fmt.Fprintf(sb, "%s\tif len(%s) > 0 {\n%s\t\t%s[%q] = %s\n%s\t}\n%s}\n", indent, listVar, indent, mapVar, key, listVar, indent, indent)

	default:
		emitLeaf(sb, indent, mapVar, key, expr, fd.TypeStr)
	}
}

func emitLeaf(sb *strings.Builder, indent, mapVar, key, expr, typ string) {
	switch typ {
	case "string":
		fmt.Fprintf(sb, "%sif v := %s; v != \"\" {\n%s\t%s[%q] = v\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "bool":
		fmt.Fprintf(sb, "%sif %s {\n%s\t%s[%q] = true\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "int":
		fmt.Fprintf(sb, "%sif v := %s; v != 0 {\n%s\t%s[%q] = v\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "time.Duration":
		fmt.Fprintf(sb, "%sif v := %s; v != 0 {\n%s\t%s[%q] = v.String()\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "[]string":
		fmt.Fprintf(sb, "%sif v := %s; len(v) > 0 {\n%s\t%s[%q] = v\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "*bool":
		fmt.Fprintf(sb, "%sif v := %s; v != nil {\n%s\t%s[%q] = *v\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "*int":
		fmt.Fprintf(sb, "%sif v := %s; v != nil {\n%s\t%s[%q] = *v\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "*time.Duration":
		fmt.Fprintf(sb, "%sif v := %s; v != nil {\n%s\t%s[%q] = v.String()\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	case "map[string]string":
		fmt.Fprintf(sb, "%sif v := %s; len(v) > 0 {\n%s\t%s[%q] = v\n%s}\n",
			indent, expr, indent, mapVar, key, indent)
	default:
		fmt.Fprintf(sb, "%s// TODO: unhandled type %q for field %q — add manually\n", indent, typ, key)
	}
}

func isKnownStruct(ts string) bool {
	if ts == "" {
		return false
	}
	if _, isFlatSlice := flatIdx[ts]; isFlatSlice {
		return false
	}
	_, ok := allStructs[ts]
	return ok
}

// sliceElemStruct returns the element type name if ts is a slice of a known
// struct (e.g. "[]K8sSelector" → "K8sSelector"), else "".
func sliceElemStruct(ts string) string {
	elem, ok := strings.CutPrefix(ts, "[]")
	if !ok {
		return ""
	}
	if _, ok := allStructs[elem]; ok {
		return elem
	}
	return ""
}

// ── Parse Go source files ─────────────────────────────────────────────────────

func parseGoFiles(paths ...string) (map[string]map[string]fieldDef, error) {
	result := make(map[string]map[string]fieldDef)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		if err := mergeStructs(result, data); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
	}
	return result, nil
}

func parseGoSource(sources ...[]byte) (map[string]map[string]fieldDef, error) {
	result := make(map[string]map[string]fieldDef)
	for _, src := range sources {
		if err := mergeStructs(result, src); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func mergeStructs(result map[string]map[string]fieldDef, src []byte) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return err
	}
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		switch t := ts.Type.(type) {
		case *ast.StructType:
			fields := make(map[string]fieldDef)
			for _, field := range t.Fields.List {
				if field.Tag == nil || len(field.Names) == 0 {
					continue
				}
				key := extractAlloyKey(strings.Trim(field.Tag.Value, "`"))
				if key == "" || key == "-" {
					continue
				}
				fields[key] = fieldDef{
					GoName:  field.Names[0].Name,
					TypeStr: typeStr(field.Type),
				}
			}
			result[ts.Name.Name] = fields
		case *ast.ArrayType:
			if t.Len == nil {
				namedSlices[ts.Name.Name] = typeStr(t.Elt)
			}
		}
		return true
	})
	return nil
}

func extractAlloyKey(tag string) string {
	const p = `alloy:"`
	i := strings.Index(tag, p)
	if i < 0 {
		return ""
	}
	rest := tag[i+len(p):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	parts := strings.SplitN(rest[:j], ",", 2)
	return parts[0]
}

func typeStr(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if pkg, ok := x.X.(*ast.Ident); ok {
			return pkg.Name + "." + x.Sel.Name
		}
		return x.Sel.Name
	case *ast.StarExpr:
		return "*" + typeStr(x.X)
	case *ast.ArrayType:
		if x.Len == nil {
			return "[]" + typeStr(x.Elt)
		}
	case *ast.MapType:
		return "map[" + typeStr(x.Key) + "]" + typeStr(x.Value)
	}
	return "unknown"
}

// ── Schema helpers ────────────────────────────────────────────────────────────

func readSchema(path string) (*schemaDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc schemaDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func resolveRef(defs map[string]*schemaProp, prop *schemaProp) *schemaProp {
	if prop == nil || prop.Ref == "" {
		return prop
	}
	name := strings.TrimPrefix(prop.Ref, "#/$defs/")
	return defs[name]
}

func resolveProps(defs map[string]*schemaProp, node *schemaProp) map[string]*schemaProp {
	if node == nil {
		return nil
	}
	if node.Properties != nil {
		return node.Properties
	}
	if node.Ref != "" {
		if r := resolveRef(defs, node); r != nil {
			return resolveProps(defs, r)
		}
	}
	for _, sub := range node.AllOf {
		if r := resolveRef(defs, sub); r != nil {
			if p := resolveProps(defs, r); p != nil {
				return p
			}
		}
	}
	return nil
}

func sortedSchemaKeys(m map[string]*schemaProp) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedFieldKeys(m map[string]fieldDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ── mappings_examples.md generation ──────────────────────────────────────────

func generateMappingsExamples(m *Mappings, sc *schemaDoc) []byte {
	var sb strings.Builder

	sb.WriteString("# mappings.json — transformation examples\n\n")
	sb.WriteString("Auto-generated by `go run ./gen/main.go`. DO NOT EDIT.\n\n")
	sb.WriteString("Each section shows: the Beyla YAML format, the Alloy input form that produces it.\n\n")
	sb.WriteString("---\n\n")

	// inject_wrappers
	if len(m.InjectWrappers) > 0 {
		sb.WriteString("## inject_wrappers\n\n")
		for _, iw := range m.InjectWrappers {
			fmt.Fprintf(&sb, "### `%s` wraps `%s` under `%s`\n\n", iw.SchemaPath, strings.Join(iw.AlloyFields, "`, `"), iw.SchemaKey)

			pathParts := strings.Split(iw.SchemaPath, ".")
			exampleField := iw.AlloyFields[0]

			sb.WriteString("Beyla YAML:\n\n```yaml\n")
			for i, p := range pathParts {
				fmt.Fprintf(&sb, "%s%s:\n", strings.Repeat("  ", i), p)
			}
			indent := strings.Repeat("  ", len(pathParts))
			fmt.Fprintf(&sb, "%s%s:\n", indent, iw.SchemaKey)
			fmt.Fprintf(&sb, "%s  %s:\n", indent, exampleField)
			fmt.Fprintf(&sb, "%s    enabled: true\n", indent)
			sb.WriteString("```\n\n")

			sb.WriteString("Alloy:\n\n```\n")
			for i, p := range pathParts {
				fmt.Fprintf(&sb, "%s%s {\n", strings.Repeat("  ", i), p)
			}
			fmt.Fprintf(&sb, "%s%s {\n", indent, exampleField)
			fmt.Fprintf(&sb, "%s  enabled = true\n", indent)
			fmt.Fprintf(&sb, "%s}\n", indent)
			for i := len(pathParts) - 1; i >= 0; i-- {
				fmt.Fprintf(&sb, "%s}\n", strings.Repeat("  ", i))
			}
			sb.WriteString("```\n\n")
		}
	}

	// aliases
	if len(m.Aliases) > 0 {
		sb.WriteString("## aliases\n\n")
		aliasKeys := make([]string, 0, len(m.Aliases))
		for k := range m.Aliases {
			aliasKeys = append(aliasKeys, k)
		}
		sort.Strings(aliasKeys)

		for _, schemaKey := range aliasKeys {
			entry := m.Aliases[schemaKey]
			warnNote := ""
			if entry.Warn {
				warnNote = " (warn: true)"
			}
			fmt.Fprintf(&sb, "### `%s` → `%s`%s\n\n", schemaKey, entry.AlloyKey, warnNote)

			parts := strings.Split(schemaKey, ".")
			leafKey := parts[len(parts)-1]

			sb.WriteString("Beyla YAML:\n\n```yaml\n")
			for i, p := range parts {
				if i < len(parts)-1 {
					fmt.Fprintf(&sb, "%s%s:\n", strings.Repeat("  ", i), p)
				} else {
					fmt.Fprintf(&sb, "%s%s:\n", strings.Repeat("  ", i), p)
					fmt.Fprintf(&sb, "%s  - example_value\n", strings.Repeat("  ", i))
				}
			}
			sb.WriteString("```\n\n")

			sb.WriteString("Alloy (canonical):\n\n```\n")
			alloySection := parts[0]
			if len(parts) > 1 {
				alloySection = strings.Join(parts[:len(parts)-1], ".")
			}
			fmt.Fprintf(&sb, "%s {\n  %s = [\"example_value\"]\n}\n", alloySection, leafKey)
			sb.WriteString("```\n\n")

			sb.WriteString("Alloy (compat")
			if entry.Warn {
				sb.WriteString(", emits deprecation warning")
			}
			sb.WriteString("):\n\n```\n")
			fmt.Fprintf(&sb, "%s {\n  %s = [\"example_value\"]\n}\n", alloySection, entry.AlloyKey)
			sb.WriteString("```\n\n")
		}
	}

	// skip
	if len(m.Skip) > 0 {
		sb.WriteString("## skip\n\n")
		sb.WriteString("Schema fields suppressed from TODO generation. Each is handled manually with a different alloy key.\n\n")
		sb.WriteString("| Schema path | Reason |\n|---|---|\n")
		for _, entry := range m.Skip {
			parts := strings.Split(entry, ".")
			leaf := parts[len(parts)-1]
			fmt.Fprintf(&sb, "| `%s` | `%s` has no direct alloy mapping; handled by hand-written code |\n", entry, leaf)
		}
		sb.WriteString("\n")
	}

	// manual_sections
	if len(m.ManualSections) > 0 {
		sb.WriteString("## manual_sections\n\n")
		sb.WriteString("Top-level YAML sections absent from `schema.json`; config emission is hand-written.\n\n")
		for _, s := range m.ManualSections {
			fmt.Fprintf(&sb, "- `%s`\n", s)
		}
		sb.WriteString("\n")
	}

	// multi_section
	if len(m.MultiSection) > 0 {
		sb.WriteString("## multi_section\n\n")
		sb.WriteString("One Alloy block maps to multiple Beyla YAML sections. Fill helpers are generated.\n\n")
		sb.WriteString("| Alloy path | Beyla YAML section | Fill helper |\n|---|---|---|\n")
		for _, entry := range m.MultiSection {
			fmt.Fprintf(&sb, "| `%s` | `%s` | `fill%sConfig` |\n",
				entry.AlloyPath, entry.YamlSection, toPascalCase(entry.YamlSection))
		}
		sb.WriteString("\n")
	}

	// flatten_transforms
	if len(m.FlattenTransforms) > 0 {
		sb.WriteString("## flatten_transforms\n\n")
		sb.WriteString("Schema fields with a common prefix are grouped into a nested Alloy block.\n\n")
		for _, ft := range m.FlattenTransforms {
			fmt.Fprintf(&sb, "### `%s`: `%s*` → `%s {}`\n\n", ft.SchemaType, ft.Prefix, ft.AlloyBlock)
			fmt.Fprintf(&sb, "Schema fields prefixed `%s` (e.g. `%snamespace`) are exposed as `%s { namespace = ... }`.\n\n",
				ft.Prefix, ft.Prefix, ft.AlloyBlock)
		}
	}

	// map_keyed_by
	if len(m.MapKeyedBy) > 0 {
		sb.WriteString("## map_keyed_by\n\n")
		sb.WriteString("A YAML map keyed by a named field is represented as a repeated block in Alloy.\n\n")
		sb.WriteString("| Go type | Key field | Fill helper |\n|---|---|---|\n")
		keys := make([]string, 0, len(m.MapKeyedBy))
		for k := range m.MapKeyedBy {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "| `%s` | `%s` | `fill%sConfig` |\n", k, m.MapKeyedBy[k], k)
		}
		sb.WriteString("\n")
	}

	// name_overrides
	if len(m.NameOverrides) > 0 {
		sb.WriteString("## name_overrides\n\n")
		sb.WriteString("Schema `$def` names mapped to custom Go type names.\n\n")
		sb.WriteString("| Schema def | Go type |\n|---|---|\n")
		noKeys := make([]string, 0, len(m.NameOverrides))
		for k := range m.NameOverrides {
			noKeys = append(noKeys, k)
		}
		sort.Strings(noKeys)
		for _, k := range noKeys {
			fmt.Fprintf(&sb, "| `%s` | `%s` |\n", k, m.NameOverrides[k])
		}
		sb.WriteString("\n")
	}

	return []byte(sb.String())
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

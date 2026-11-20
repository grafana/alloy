package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"text/template"
	"time"

	otelcomponent "go.opentelemetry.io/collector/component"
	otelprocessor "go.opentelemetry.io/collector/processor"

	"github.com/grafana/alloy/internal/component/otelcol/otelgen"
	"github.com/grafana/alloy/internal/featuregate"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"lowerFirst": lowerFirst,
}).ParseFS(templateFS, "templates/*.tmpl"))

func isBasic(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func fullName(c otelgen.Component) string {
	return "otelcol." + string(c.Kind()) + "." + c.ComponentName()
}

// generate writes the wrapper for c to <kind>/<name> under root, the otelcol
// package directory.
func generate(c otelgen.Component, root string, updateTests bool) error {
	pkg := c.Name()
	dir := filepath.Join(root, string(c.Kind()), pkg)

	data, err := buildData(c, pkg)
	if err != nil {
		return err
	}

	kind := string(c.Kind())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := render(kind+".go.tmpl", data, filepath.Join(dir, c.Name()+".go")); err != nil {
		return err
	}

	// The conversions are written by hand, so only a missing file gets stubs.
	convertFile := filepath.Join(dir, "convert.go")
	if _, err := os.Stat(convertFile); len(data.CustomFields) > 0 && os.IsNotExist(err) {
		if err := render("convert.go.tmpl", data, convertFile); err != nil {
			return err
		}
	}

	testFile := filepath.Join(dir, "zz_generated_test.go")
	if !updateTests {
		if _, err := os.Stat(testFile); err == nil {
			return nil
		}
	}
	return render(kind+"_test.go.tmpl", data, testFile)
}

func render(tmpl string, data templateData, out string) error {
	t := templates.Lookup(tmpl)
	if t == nil {
		return fmt.Errorf("missing template %s", tmpl)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format %s: %w\n%s", out, err, buf.String())
	}
	return os.WriteFile(out, src, 0o644)
}

type templateData struct {
	Package   string
	Name      string
	Stability string
	// StabilityDoc is the OverrideStability reason as // lines, empty without
	// an override.
	StabilityDoc    string
	UpstreamPath    string
	UpstreamPackage string
	Fields          []field
	// Blocks are the nested structs of the config, in the order found.
	Blocks []*block
	// StdImports and Imports are the packages field types need, split into
	// the standard library and everything else.
	StdImports []string
	Imports    []string
	// TestStdImports and TestImports are the packages the default literals in
	// the test need.
	TestStdImports []string
	TestImports    []string
	// Renames are the argument names set with OverrideArgumentName, rewritten
	// in upstream validation errors.
	Renames []rename
	// CustomFields are the fields set with OverrideArgumentType, which need
	// hand-written conversions.
	CustomFields []field
	// StubStdImports and StubImports are the packages the conversion stubs
	// need.
	StubStdImports []string
	StubImports    []string
	// Signals are the upstream stability levels of each signal, Undefined when
	// the factory does not support it.
	Signals []signal
	// Overrides are the fields OverrideDefaultArguments changes from upstream.
	Overrides []override
	// OverridesDoc is the OverrideDefaultArguments reason as // lines.
	OverridesDoc string
}

type field struct {
	// Name is the upstream Go field name.
	Name string
	// ArgName is the field name in Arguments, Name unless set with
	// OverrideArgumentGoName.
	ArgName   string
	AlloyName string
	Type      string
	// Doc is the upstream doc comment as // lines, empty when there is none.
	Doc string
	// Default is the Go literal of the upstream default, empty when zero.
	Default string
	// Required is set with OverrideArgumentRequired, fields are optional
	// otherwise.
	Required bool
	// Block is set for struct fields, which become Alloy blocks.
	Block *block
	// UpstreamType is set when the upstream type is a named basic type, which
	// Arguments holds as its underlying type.
	UpstreamType string
	// Custom is set with OverrideArgumentType, converted by hand-written functions.
	Custom bool
	// UpstreamGoType is the upstream type of a Custom field.
	UpstreamGoType string
}

// ToAlloy returns expr, an upstream value of f, as its Arguments type.
func (f field) ToAlloy(expr string) string {
	switch {
	case f.Custom:
		return lowerFirst(f.ArgName) + "FromUpstream(" + expr + ")"
	case f.Block != nil:
		return lowerFirst(f.Block.Name) + "FromUpstream(" + expr + ")"
	case f.UpstreamType != "":
		return f.Type + "(" + expr + ")"
	}
	return expr
}

// ToUpstream returns expr, an Arguments value of f, as its upstream type.
func (f field) ToUpstream(expr string) string {
	switch {
	case f.Custom:
		return lowerFirst(f.ArgName) + "ToUpstream(" + expr + ")"
	case f.Block != nil:
		return expr + ".Convert()"
	case f.UpstreamType != "":
		return f.UpstreamType + "(" + expr + ")"
	}
	return expr
}

// block is an Alloy struct generated for a nested upstream struct.
type block struct {
	// Name is the upstream type name, reused for the Alloy struct.
	Name string
	// UpstreamType is the qualified upstream type, e.g. pkg.PassThrough.
	UpstreamType string
	Fields       []field
}

// signal is the upstream stability of a signal, Level is a
// component.StabilityLevel constant name.
type signal struct {
	Name  string
	Level string
}

// rename is an upstream mapstructure name From that Alloy calls To.
type rename struct {
	From string
	To   string
}

// override sets the field at Path, a dotted Go path, to the literal Value.
type override struct {
	Path  string
	Value string
}

var stabilityConsts = map[featuregate.Stability]string{
	featuregate.StabilityExperimental:       "StabilityExperimental",
	featuregate.StabilityPublicPreview:      "StabilityPublicPreview",
	featuregate.StabilityGenerallyAvailable: "StabilityGenerallyAvailable",
}

// stability returns the Alloy stability for the lowest upstream stability of
// the signals f supports.
func stability(f otelprocessor.Factory) (featuregate.Stability, error) {
	lowest := otelcomponent.StabilityLevelUndefined
	for _, l := range []otelcomponent.StabilityLevel{f.TracesStability(), f.MetricsStability(), f.LogsStability()} {
		if l != otelcomponent.StabilityLevelUndefined && (lowest == otelcomponent.StabilityLevelUndefined || l < lowest) {
			lowest = l
		}
	}
	switch lowest {
	case otelcomponent.StabilityLevelDevelopment, otelcomponent.StabilityLevelAlpha:
		return featuregate.StabilityExperimental, nil
	case otelcomponent.StabilityLevelBeta:
		return featuregate.StabilityPublicPreview, nil
	case otelcomponent.StabilityLevelStable:
		return featuregate.StabilityGenerallyAvailable, nil
	}
	return featuregate.StabilityUndefined, fmt.Errorf("no Alloy stability for upstream stability %s", lowest)
}

func buildData(c otelgen.Component, pkg string) (templateData, error) {
	f, ok := c.Factory().(otelprocessor.Factory)
	if !ok {
		return templateData{}, fmt.Errorf("unsupported factory %T", c.Factory())
	}
	s, err := stability(f)
	if err != nil && c.Stability() == featuregate.StabilityUndefined {
		return templateData{}, err
	}
	var stabilityDoc string
	if c.Stability() != featuregate.StabilityUndefined {
		if c.Stability() == s {
			return templateData{}, fmt.Errorf("OverrideStability: upstream stability is already %s", s)
		}
		if _, ok := stabilityConsts[c.Stability()]; !ok {
			return templateData{}, fmt.Errorf("OverrideStability: unsupported stability %v", c.Stability())
		}
		if strings.TrimSpace(c.StabilityReason()) == "" {
			return templateData{}, fmt.Errorf("OverrideStability: reason is required")
		}
		s = c.Stability()
		stabilityDoc = commentLines("Alloy stability differs from upstream: " + c.StabilityReason())
	}

	base := reflect.ValueOf(c.Factory().CreateDefaultConfig())
	if base.Kind() != reflect.Pointer || base.Elem().Kind() != reflect.Struct {
		return templateData{}, fmt.Errorf("default config must be a pointer to a struct, got %s", base.Type())
	}
	if fn, t := c.DefaultArguments(); fn != nil {
		if t != base.Type() {
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: takes %s, config is %s", t, base.Type())
		}
		if strings.TrimSpace(c.DefaultArgumentsReason()) == "" {
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: reason is required")
		}
	}
	def := c.Factory().CreateDefaultConfig()
	if fn, _ := c.DefaultArguments(); fn != nil {
		fn(def)
	}
	cfg := reflect.ValueOf(def).Elem()

	// The config type can be an alias of a type in an internal package, so the
	// import path comes from the package declaring NewFactory.
	fn := runtime.FuncForPC(reflect.ValueOf(c.NewFactory()).Pointer()).Name()
	upstreamPath, ok := strings.CutSuffix(fn, ".NewFactory")
	if !ok {
		return templateData{}, fmt.Errorf("factory constructor %s is not a NewFactory function", fn)
	}
	w := &walker{
		argumentNames: c.ArgumentNames(),
		usedNames:     map[string]bool{},
		required:      c.RequiredArguments(),
		usedRequired:  map[string]bool{},
		argumentTypes: c.ArgumentTypes(),
		usedArguments: map[string]bool{},
		goNames:       c.ArgumentGoNames(),
		usedGoNames:   map[string]bool{},
		stubImports:   map[string]struct{}{},
		imports:       map[string]struct{}{},
		testImports:   map[string]struct{}{},
	}
	fields, err := w.fields(cfg.Type(), cfg, base.Elem(), "", "")
	if err != nil {
		return templateData{}, err
	}
	for p := range w.argumentNames {
		if !w.usedNames[p] {
			return templateData{}, fmt.Errorf("OverrideArgumentName: no upstream field at %q", p)
		}
	}
	for p := range w.goNames {
		if !w.usedGoNames[p] {
			return templateData{}, fmt.Errorf("OverrideArgumentGoName: no upstream field at %q", p)
		}
	}
	for p := range w.argumentTypes {
		if !w.usedArguments[p] {
			return templateData{}, fmt.Errorf("OverrideArgumentType: no upstream field at %q", p)
		}
	}
	for p := range w.required {
		if !w.usedRequired[p] {
			return templateData{}, fmt.Errorf("OverrideArgumentRequired: no upstream field at %q", p)
		}
	}

	data := templateData{
		Package:         pkg,
		Name:            fullName(c),
		Stability:       stabilityConsts[s],
		StabilityDoc:    stabilityDoc,
		UpstreamPath:    upstreamPath,
		UpstreamPackage: path.Base(upstreamPath),
		Fields:          fields,
		Blocks:          w.blocks,
		Overrides:       w.overrides,
		OverridesDoc:    commentLines("Alloy defaults differ from upstream: " + c.DefaultArgumentsReason()),
	}
	data.Signals = []signal{
		{"Traces", "StabilityLevel" + f.TracesStability().String()},
		{"Metrics", "StabilityLevel" + f.MetricsStability().String()},
		{"Logs", "StabilityLevel" + f.LogsStability().String()},
	}
	imports, testImports := w.imports, w.testImports

	if len(w.renames) > 0 {
		// Longest first, so a name never matches inside a longer one.
		sort.Slice(w.renames, func(i, j int) bool { return len(w.renames[i].From) > len(w.renames[j].From) })
		data.Renames = w.renames
		imports["errors"] = struct{}{}
		imports["strings"] = struct{}{}
	}
	data.StdImports, data.Imports = splitImports(imports, upstreamPath)
	data.TestStdImports, data.TestImports = splitImports(testImports, upstreamPath)
	data.CustomFields = w.customFields
	data.StubStdImports, data.StubImports = splitImports(w.stubImports, "")
	return data, nil
}

// walker collects fields, blocks, imports and overrides from an upstream
// config struct.
type walker struct {
	argumentNames map[string]string
	usedNames     map[string]bool
	renames       []rename
	required      map[string]bool
	usedRequired  map[string]bool
	argumentTypes map[string]reflect.Type
	goNames       map[string]string
	usedGoNames   map[string]bool
	usedArguments map[string]bool
	customFields  []field
	stubImports   map[string]struct{}
	imports       map[string]struct{}
	testImports   map[string]struct{}
	blocks        []*block
	overrides     []override
}

// fields returns the fields of struct type t with Alloy defaults def and
// upstream defaults base. prefix is its dotted Go path and tagPrefix its
// dotted mapstructure path.
func (w *walker) fields(t reflect.Type, def, base reflect.Value, prefix, tagPrefix string) ([]field, error) {
	docs, err := fieldDocs(t.PkgPath(), t.Name())
	if err != nil {
		return nil, err
	}

	var out []field
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		tag := strings.Split(sf.Tag.Get("mapstructure"), ",")[0]
		if tag == "" {
			return nil, fmt.Errorf("field %s%s: missing mapstructure name", prefix, sf.Name)
		}
		v := def.Field(i)
		tagPath := tagPrefix + tag

		alloyName := tag
		if name, ok := w.argumentNames[tagPath]; ok {
			alloyName = name
			w.usedNames[tagPath] = true
			w.renames = append(w.renames, rename{From: tag, To: name})
		}

		f := field{
			Name:      sf.Name,
			ArgName:   sf.Name,
			AlloyName: alloyName,
			Type:      sf.Type.String(),
			Doc:       docs[sf.Name],
			Required:  w.required[tagPath],
		}
		if f.Required {
			w.usedRequired[tagPath] = true
		}
		if name, ok := w.goNames[tagPath]; ok {
			f.ArgName = name
			w.usedGoNames[tagPath] = true
		}
		if !v.IsZero() {
			f.Default = literal(v, w.testImports)
		}

		if t, ok := w.argumentTypes[tagPath]; ok {
			w.usedArguments[tagPath] = true
			f.Custom = true
			f.UpstreamGoType = sf.Type.String()
			f.Type = t.String()
			// The upstream doc describes the upstream type and unit.
			f.Doc = ""
			if err := collectImports(t, w.imports); err != nil {
				return nil, fmt.Errorf("field %s%s: %w", prefix, sf.Name, err)
			}
			if err := collectImports(t, w.stubImports); err != nil {
				return nil, fmt.Errorf("field %s%s: %w", prefix, sf.Name, err)
			}
			if err := collectImports(sf.Type, w.stubImports); err != nil {
				return nil, fmt.Errorf("field %s%s: %w", prefix, sf.Name, err)
			}
			w.customFields = append(w.customFields, f)
			out = append(out, f)
			continue
		}

		if sf.Type.Kind() == reflect.Struct {
			sub, err := w.fields(sf.Type, v, base.Field(i), prefix+sf.Name+".", tagPath+".")
			if err != nil {
				return nil, err
			}
			f.Block = &block{Name: sf.Type.Name(), UpstreamType: sf.Type.String(), Fields: sub}
			f.Type = sf.Type.Name()
			w.blocks = append(w.blocks, f.Block)
			w.imports[sf.Type.PkgPath()] = struct{}{}
			out = append(out, f)
			continue
		}

		if err := collectImports(sf.Type, w.imports); err != nil {
			return nil, fmt.Errorf("field %s%s: %w", prefix, sf.Name, err)
		}
		if sf.Type.PkgPath() != "" && sf.Type != reflect.TypeFor[time.Duration]() && isBasic(sf.Type.Kind()) {
			f.UpstreamType = sf.Type.String()
			f.Type = sf.Type.Kind().String()
		}
		if !reflect.DeepEqual(v.Interface(), base.Field(i).Interface()) {
			w.overrides = append(w.overrides, override{
				Path:  prefix + sf.Name,
				Value: literal(v, w.imports),
			})
		}
		out = append(out, f)
	}
	return out, nil
}

// splitImports returns the sorted standard library and other imports, leaving
// out the upstream package which templates import themselves.
func splitImports(imports map[string]struct{}, upstreamPath string) ([]string, []string) {
	var std, other []string
	for p := range imports {
		switch {
		case p == upstreamPath:
		case isStdlib(p):
			std = append(std, p)
		default:
			other = append(other, p)
		}
	}
	sort.Strings(std)
	sort.Strings(other)
	return std, other
}

var durationType = reflect.TypeFor[time.Duration]()

var durationUnits = []struct {
	unit time.Duration
	name string
}{
	{time.Hour, "time.Hour"},
	{time.Minute, "time.Minute"},
	{time.Second, "time.Second"},
	{time.Millisecond, "time.Millisecond"},
	{time.Microsecond, "time.Microsecond"},
	{time.Nanosecond, "time.Nanosecond"},
}

var intLimits = map[reflect.Kind]map[uint64]string{
	reflect.Int:    {math.MaxInt: "math.MaxInt"},
	reflect.Int32:  {math.MaxInt32: "math.MaxInt32"},
	reflect.Int64:  {math.MaxInt64: "math.MaxInt64"},
	reflect.Uint:   {math.MaxUint: "math.MaxUint"},
	reflect.Uint32: {math.MaxUint32: "math.MaxUint32"},
	reflect.Uint64: {math.MaxUint64: "math.MaxUint64"},
}

// literal returns v as a readable Go literal and adds the packages it uses to
// imports. Values it does not recognize fall back to %#v.
func literal(v reflect.Value, imports map[string]struct{}) string {
	if v.Type() == durationType {
		d := time.Duration(v.Int())
		for _, u := range durationUnits {
			if d%u.unit == 0 {
				imports["time"] = struct{}{}
				return fmt.Sprintf("%d * %s", d/u.unit, u.name)
			}
		}
	}

	if limits, ok := intLimits[v.Kind()]; ok && v.Type().PkgPath() == "" {
		var n uint64
		if v.CanInt() {
			n = uint64(v.Int())
		} else {
			n = v.Uint()
		}
		if name, ok := limits[n]; ok {
			imports["math"] = struct{}{}
			return name
		}
	}

	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	}

	if v.Kind() == reflect.Struct {
		var parts []string
		for i := range v.NumField() {
			sf := v.Type().Field(i)
			if sf.IsExported() && !v.Field(i).IsZero() {
				parts = append(parts, sf.Name+": "+literal(v.Field(i), imports))
			}
		}
		imports[v.Type().PkgPath()] = struct{}{}
		return v.Type().String() + "{" + strings.Join(parts, ", ") + "}"
	}

	return fmt.Sprintf("%#v", v.Interface())
}

// collectImports adds the packages t needs to imports and rejects types the
// generator cannot write yet.
func collectImports(t reflect.Type, imports map[string]struct{}) error {
	switch t.Kind() {
	case reflect.Slice, reflect.Pointer:
		return collectImports(t.Elem(), imports)
	case reflect.Map:
		if err := collectImports(t.Key(), imports); err != nil {
			return err
		}
		return collectImports(t.Elem(), imports)
	case reflect.Struct, reflect.Interface, reflect.Func, reflect.Chan, reflect.Array:
		return fmt.Errorf("unsupported type %s", t)
	}
	if p := t.PkgPath(); p != "" {
		imports[p] = struct{}{}
	}
	return nil
}

func isStdlib(importPath string) bool {
	return !strings.Contains(strings.Split(importPath, "/")[0], ".")
}

// fieldDocs returns the doc comments of the fields of struct typeName in the
// package at importPath, formatted as // lines.
func fieldDocs(importPath, typeName string) (map[string]string, error) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", importPath).Output()
	if err != nil {
		return nil, fmt.Errorf("go list %s: %w", importPath, err)
	}
	dir := strings.TrimSpace(string(out))

	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}

	docs := map[string]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return false
			}
			for _, f := range st.Fields.List {
				if f.Doc == nil {
					continue
				}
				for _, n := range f.Names {
					docs[n.Name] = commentLines(f.Doc.Text())
				}
			}
			return false
		})
	}
	return docs, nil
}

func commentLines(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight("// "+l, " ")
	}
	return strings.Join(lines, "\n")
}

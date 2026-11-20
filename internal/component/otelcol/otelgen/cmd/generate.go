package main

import (
	"bytes"
	"embed"
	"encoding"
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
	otelconnector "go.opentelemetry.io/collector/connector"
	otelprocessor "go.opentelemetry.io/collector/processor"

	"github.com/grafana/alloy/internal/component/otelcol/otelgen"
	"github.com/grafana/alloy/internal/featuregate"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"lowerFirst": lowerFirst,
	"canFail":    canFail,
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

	extraFile := filepath.Join(dir, "extra.go")
	if _, err := os.Stat(extraFile); data.ExtraArguments && os.IsNotExist(err) {
		if err := render("extra.go.tmpl", data, extraFile); err != nil {
			return err
		}
	}

	testFile := filepath.Join(dir, "zz_generated_test.go")
	if !updateTests {
		if _, err := os.Stat(testFile); err == nil {
			return nil
		}
	}
	return render("test.go.tmpl", data, testFile)
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
	// ExtraArguments is set with ExtraArguments.
	ExtraArguments bool
	// HasText is set when a field is converted through its text form, which
	// needs the marshalText helper.
	HasText bool
	// ConnectorSignals are the signal pairs a connector supports.
	ConnectorSignals []signal
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
	// Slice is set for slices of structs, which become repeated Alloy blocks.
	Slice bool
	// Optional and Pointer are set for configoptional.Optional and pointers
	// to structs, which become pointer blocks, nil when unset.
	Optional bool
	Pointer  bool
	// Text is set for upstream types converted through their text form,
	// which Arguments holds as a string.
	Text bool
	// UpstreamGoType is the upstream type of a Custom field.
	UpstreamGoType string
}

// ToAlloy returns expr, an upstream value of f, as its Arguments type.
func (f field) ToAlloy(expr string) string {
	switch {
	case f.Text:
		return "marshalText(&" + expr + ")"
	case f.Custom:
		return lowerFirst(f.ArgName) + "FromUpstream(" + expr + ")"
	case f.Slice:
		return lowerFirst(f.Block.Name) + "SliceFromUpstream(" + expr + ")"
	case f.Optional:
		return lowerFirst(f.Block.Name) + "OptionalFromUpstream(" + expr + ")"
	case f.Pointer:
		return lowerFirst(f.Block.Name) + "PtrFromUpstream(" + expr + ")"
	case f.Block != nil:
		return lowerFirst(f.Block.Name) + "FromUpstream(" + expr + ")"
	case f.UpstreamType != "":
		return f.Type + "(" + expr + ")"
	}
	return expr
}

// Assign returns the statement setting dst, an upstream value of f, from src,
// an Arguments value. Conversions that can fail return errRet.
func (f field) Assign(dst, src, errRet string) string {
	switch {
	case f.Text:
		return "if err = (&" + dst + ").UnmarshalText([]byte(" + src + ")); err != nil {\n\treturn " + errRet + "\n}"
	case f.CanFail():
		return "if " + dst + ", err = " + f.ToUpstream(src) + "; err != nil {\n\treturn " + errRet + "\n}"
	}
	return dst + " = " + f.ToUpstream(src)
}

// CanFail reports whether converting f to upstream can return an error.
func (f field) CanFail() bool { return f.Text || (f.Block != nil && f.Block.CanFail) }

func canFail(fields []field) bool {
	for _, f := range fields {
		if f.CanFail() {
			return true
		}
	}
	return false
}

// ToUpstream returns expr, an Arguments value of f, as its upstream type.
// Blocks return the value and an error.
func (f field) ToUpstream(expr string) string {
	switch {
	case f.Custom:
		return lowerFirst(f.ArgName) + "ToUpstream(" + expr + ")"
	case f.Slice:
		return lowerFirst(f.Block.Name) + "SliceToUpstream(" + expr + ")"
	case f.Optional:
		return lowerFirst(f.Block.Name) + "OptionalToUpstream(" + expr + ")"
	case f.Pointer:
		return lowerFirst(f.Block.Name) + "PtrToUpstream(" + expr + ")"
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
	// InSlice is set when a field holds a slice of the block, which needs
	// slice conversion helpers.
	InSlice bool
	// InOptional and InPointer are set when a field holds the block as
	// configoptional.Optional or a pointer upstream, which need conversion
	// helpers.
	InOptional bool
	InPointer  bool
	// CanFail is set when converting a field of the block can fail, so its
	// conversions return an error.
	CanFail bool
	// InPlain is set when a field holds the block by value.
	InPlain bool
	// Defaults are the fields OverrideDefaultArguments sets when the block is
	// set, DefaultsDoc its reason as // lines.
	Defaults    []override
	DefaultsDoc string
}

// signal is the upstream stability of a signal, or a signal pair such as
// TracesToMetrics for connectors.
type signal struct {
	Name      string
	Stability otelcomponent.StabilityLevel
}

// Level returns the component.StabilityLevel constant name of the stability.
func (s signal) Level() string { return "StabilityLevel" + s.Stability.String() }

// Supported reports whether the factory supports the signal.
func (s signal) Supported() bool { return s.Stability != otelcomponent.StabilityLevelUndefined }

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

// signals returns the upstream stability of each signal, or signal pair for
// connectors, that f can support.
func signals(f otelcomponent.Factory) ([]signal, error) {
	switch f := f.(type) {
	case otelprocessor.Factory:
		return []signal{
			{"Traces", f.TracesStability()},
			{"Metrics", f.MetricsStability()},
			{"Logs", f.LogsStability()},
		}, nil
	case otelconnector.Factory:
		return []signal{
			{"TracesToTraces", f.TracesToTracesStability()},
			{"TracesToMetrics", f.TracesToMetricsStability()},
			{"TracesToLogs", f.TracesToLogsStability()},
			{"MetricsToTraces", f.MetricsToTracesStability()},
			{"MetricsToMetrics", f.MetricsToMetricsStability()},
			{"MetricsToLogs", f.MetricsToLogsStability()},
			{"LogsToTraces", f.LogsToTracesStability()},
			{"LogsToMetrics", f.LogsToMetricsStability()},
			{"LogsToLogs", f.LogsToLogsStability()},
		}, nil
	}
	return nil, fmt.Errorf("unsupported factory %T", f)
}

// stability returns the Alloy stability for the lowest upstream stability of
// the supported signals.
func stability(signals []signal) (featuregate.Stability, error) {
	lowest := otelcomponent.StabilityLevelUndefined
	for _, s := range signals {
		if l := s.Stability; l != otelcomponent.StabilityLevelUndefined && (lowest == otelcomponent.StabilityLevelUndefined || l < lowest) {
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
	sigs, err := signals(c.Factory())
	if err != nil {
		return templateData{}, err
	}
	s, err := stability(sigs)
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
	var root *otelgen.DefaultArguments
	var blockDefaults []otelgen.DefaultArguments
	for _, d := range c.DefaultArguments() {
		if strings.TrimSpace(d.Reason) == "" {
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: reason is required")
		}
		switch {
		case d.Type == base.Type():
			if root != nil {
				return templateData{}, fmt.Errorf("OverrideDefaultArguments: %s set twice", d.Type)
			}
			root = &d
		case d.Type.Kind() == reflect.Pointer && d.Type.Elem().Kind() == reflect.Struct:
			blockDefaults = append(blockDefaults, d)
		default:
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: takes %s, want a pointer to the config or a block type", d.Type)
		}
	}
	def := c.Factory().CreateDefaultConfig()
	if root != nil {
		root.Fn(def)
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
	}
	if root != nil {
		data.OverridesDoc = commentLines("Alloy defaults differ from upstream: " + root.Reason)
	}
	for _, d := range blockDefaults {
		b := w.blockOf(d.Type.Elem())
		if b == nil {
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: no block of type %s", d.Type.Elem())
		}
		if b.InPlain || !(b.InSlice || b.InOptional || b.InPointer) {
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: %s is a plain block, set its defaults on the config", d.Type.Elem())
		}
		if b.Defaults != nil {
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: %s set twice", d.Type.Elem())
		}
		v := reflect.New(d.Type.Elem())
		d.Fn(v.Interface())
		b.Defaults = diffDefaults(reflect.New(d.Type.Elem()).Elem(), v.Elem(), "", w.imports)
		if len(b.Defaults) == 0 {
			return templateData{}, fmt.Errorf("OverrideDefaultArguments: %s changes no field", d.Type.Elem())
		}
		b.DefaultsDoc = commentLines("Alloy defaults when the block is set: " + d.Reason)
	}
	data.Signals = sigs
	data.ExtraArguments = c.HasExtraArguments()
	for _, s := range sigs {
		if s.Supported() {
			data.ConnectorSignals = append(data.ConnectorSignals, s)
		}
	}
	imports, testImports := w.imports, w.testImports

	if len(w.renames) > 0 {
		// Longest first, so a name never matches inside a longer one.
		sort.Slice(w.renames, func(i, j int) bool { return len(w.renames[i].From) > len(w.renames[j].From) })
		data.Renames = w.renames
		imports["errors"] = struct{}{}
		imports["strings"] = struct{}{}
	}
	if w.hasText {
		data.HasText = true
		imports["encoding"] = struct{}{}
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
	hasText       bool
	stubImports   map[string]struct{}
	imports       map[string]struct{}
	testImports   map[string]struct{}
	blocks        []*block
	overrides     []override
}

// block returns the block for struct type t, found at the dotted Go path
// prefix and mapstructure path tagPrefix. A type used at several paths gets one
// block, so every path must generate the same fields.
func (w *walker) block(t reflect.Type, def, base reflect.Value, prefix, tagPrefix string) (*block, error) {
	sub, err := w.fields(t, def, base, prefix, tagPrefix)
	if err != nil {
		return nil, err
	}
	for _, b := range w.blocks {
		if b.UpstreamType != t.String() {
			continue
		}
		if !reflect.DeepEqual(b.Fields, sub) {
			return nil, fmt.Errorf("type %s generates different fields at %s, overrides must apply to every use", t, strings.TrimSuffix(tagPrefix, "."))
		}
		return b, nil
	}
	b := &block{Name: t.Name(), UpstreamType: t.String(), Fields: sub, CanFail: canFail(sub)}
	w.blocks = append(w.blocks, b)
	w.imports[t.PkgPath()] = struct{}{}
	return b, nil
}

// blockOf returns the block generated for upstream struct type t, or nil.
func (w *walker) blockOf(t reflect.Type) *block {
	for _, b := range w.blocks {
		if b.UpstreamType == t.String() {
			return b
		}
	}
	return nil
}

// diffDefaults returns assignments for the fields of struct b that differ from
// a, recursing into nested structs. prefix is the dotted Go path of a and b.
func diffDefaults(a, b reflect.Value, prefix string, imports map[string]struct{}) []override {
	var out []override
	for i := range a.NumField() {
		sf := a.Type().Field(i)
		if !sf.IsExported() {
			continue
		}
		av, bv := a.Field(i), b.Field(i)
		if _, ok := optionalElem(sf.Type); !ok && sf.Type.Kind() == reflect.Struct {
			out = append(out, diffDefaults(av, bv, prefix+sf.Name+".", imports)...)
			continue
		}
		if !reflect.DeepEqual(av.Interface(), bv.Interface()) {
			out = append(out, override{Path: prefix + sf.Name, Value: literal(bv, imports)})
		}
	}
	return out
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

		if isText(sf.Type) {
			w.hasText = true
			f.Text = true
			f.Type = "string"
			out = append(out, f)
			continue
		}
		if elem, ok := optionalElem(sf.Type); ok {
			if elem.Kind() != reflect.Struct {
				return nil, fmt.Errorf("field %s%s: unsupported type %s", prefix, sf.Name, sf.Type)
			}
			zero := reflect.New(elem).Elem()
			b, err := w.block(elem, zero, zero, prefix+sf.Name+".", tagPath+".")
			if err != nil {
				return nil, err
			}
			b.InOptional = true
			w.imports[optionalPkg] = struct{}{}
			f.Block = b
			f.Optional = true
			f.Type = "*" + b.Name
			out = append(out, f)
			continue
		}
		if sf.Type.Kind() == reflect.Pointer && sf.Type.Elem().Kind() == reflect.Struct {
			elem := sf.Type.Elem()
			d, bs := reflect.New(elem).Elem(), reflect.New(elem).Elem()
			if !v.IsNil() {
				d = v.Elem()
			}
			if !base.Field(i).IsNil() {
				bs = base.Field(i).Elem()
			}
			b, err := w.block(elem, d, bs, prefix+sf.Name+".", tagPath+".")
			if err != nil {
				return nil, err
			}
			b.InPointer = true
			f.Block = b
			f.Pointer = true
			f.Type = "*" + b.Name
			out = append(out, f)
			continue
		}
		if sf.Type.Kind() == reflect.Struct {
			b, err := w.block(sf.Type, v, base.Field(i), prefix+sf.Name+".", tagPath+".")
			if err != nil {
				return nil, err
			}
			f.Block = b
			b.InPlain = true
			f.Type = b.Name
			out = append(out, f)
			continue
		}
		if sf.Type.Kind() == reflect.Slice && sf.Type.Elem().Kind() == reflect.Struct {
			elem := reflect.New(sf.Type.Elem()).Elem()
			b, err := w.block(sf.Type.Elem(), elem, elem, prefix+sf.Name+".", tagPath+".")
			if err != nil {
				return nil, err
			}
			b.InSlice = true
			f.Block = b
			f.Slice = true
			f.Type = "[]" + b.Name
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
var (
	textMarshaler   = reflect.TypeFor[encoding.TextMarshaler]()
	textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// isText reports whether t is converted through its text form. Named string
// types are left out, they convert directly and configopaque.String redacts its
// text.
func isText(t reflect.Type) bool {
	if t.Kind() == reflect.String || t.Kind() == reflect.Pointer {
		return false
	}
	p := reflect.PointerTo(t)
	return p.Implements(textMarshaler) && p.Implements(textUnmarshaler)
}

const optionalPkg = "go.opentelemetry.io/collector/config/configoptional"

// optionalElem returns T for configoptional.Optional[T].
func optionalElem(t reflect.Type) (reflect.Type, bool) {
	if t.Kind() != reflect.Struct || t.PkgPath() != optionalPkg || !strings.HasPrefix(t.Name(), "Optional[") {
		return nil, false
	}
	get, _ := reflect.PointerTo(t).MethodByName("Get")
	return get.Type.Out(0).Elem(), true
}

func literal(v reflect.Value, imports map[string]struct{}) string {
	if _, ok := optionalElem(v.Type()); ok {
		p := reflect.New(v.Type())
		p.Elem().Set(v)
		imports[optionalPkg] = struct{}{}
		if p.MethodByName("HasValue").Call(nil)[0].Bool() {
			return "configoptional.Some(" + literal(p.MethodByName("Get").Call(nil)[0].Elem(), imports) + ")"
		}
		if v.IsZero() {
			return "configoptional.None[" + v.Type().Name()[len("Optional["):len(v.Type().Name())-1] + "]()"
		}
		return "configoptional.Default(" + literal(p.MethodByName("GetOrInsertDefault").Call(nil)[0].Elem(), imports) + ")"
	}
	if v.Kind() == reflect.Slice && !v.IsNil() {
		var parts []string
		for i := range v.Len() {
			parts = append(parts, literal(v.Index(i), imports))
		}
		if p := v.Type().Elem().PkgPath(); p != "" {
			imports[p] = struct{}{}
		}
		return v.Type().String() + "{" + strings.Join(parts, ", ") + "}"
	}
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		if v.Type().Elem().Kind() == reflect.Struct {
			return "&" + literal(v.Elem(), imports)
		}
		lit := literal(v.Elem(), imports)
		// Untyped constants default to these types, others need a conversion.
		switch t := v.Type().Elem(); {
		case t == durationType, t == reflect.TypeFor[int](), t == reflect.TypeFor[float64](),
			t == reflect.TypeFor[string](), t == reflect.TypeFor[bool]():
		default:
			lit = t.String() + "(" + lit + ")"
		}
		return "new(" + lit + ")"
	}

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

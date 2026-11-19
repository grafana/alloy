package pipelinetest

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
)

// signal names the telemetry a test input feeds or a sink captures.
type signal string

const (
	signalLogs    signal = "logs"
	signalMetrics signal = "metrics"
	signalTraces  signal = "traces"
)

// sinkTarget describes how references to one replaced exit component are
// rewritten to the test sink.
type sinkTarget struct {
	// field is the export other components reference, such as receiver.
	field string
	// replacement is the sink export that takes its place.
	replacement string
}

// sinkTargetFor returns how to replace the component with the given name, such
// as loki.write, with the test sink. Only components whose single exported
// receiver can be swapped for one of the sink's are supported.
func sinkTargetFor(name, sinkID string) (sinkTarget, error) {
	switch {
	case name == "loki.write":
		return sinkTarget{field: "receiver", replacement: sinkID + ".loki_receiver"}, nil
	case name == "prometheus.remote_write":
		return sinkTarget{field: "receiver", replacement: sinkID + ".prometheus_receiver"}, nil
	case strings.HasPrefix(name, "otelcol.exporter."):
		return sinkTarget{field: "input", replacement: sinkID + ".otel_input"}, nil
	case name == "pyroscope.write":
		return sinkTarget{}, errors.New("profiles are not supported by pipeline tests yet")
	default:
		return sinkTarget{}, fmt.Errorf("only loki.write, prometheus.remote_write and otelcol.exporter.* components can be replaced with the sink, got %s", name)
	}
}

// replacedSource is an entry component removed from the config, whose
// downstream targets a generated test input sends to instead.
type replacedSource struct {
	// forwardTo is the rewritten source text of the expression the component
	// sent its data to.
	forwardTo string
	// modules lists the module instances, such as cluster_metrics.feature,
	// that hold the generated pipelinetest.source when the replaced component
	// is inside a declare block. It's empty for a top-level component, whose
	// source goes after the config.
	modules []string
}

// transformedConfig is the user config with entry and exit components
// replaced.
type transformedConfig struct {
	source string
	// sources maps a replaced entry component ID to where it sent its data.
	sources map[string]replacedSource
	// sinkRefs maps a full reference, such as loki.write.default.receiver, to
	// the sink export replacing it.
	sinkRefs map[string]string
	// sinks lists the sink generated for each replaced exit component.
	sinks []generatedSink
}

// generatedSink is the pipelinetest.sink that replaces one exit component.
type generatedSink struct {
	// exit is the ID of the replaced component, such as loki.write.default.
	exit string
	// label is the sink's block label.
	label string
	// externalLabels is the source text of the exit's external_labels, or
	// empty when it sets none.
	externalLabels string
	// writeRelabel holds the source text of each write_relabel_config block
	// of a prometheus.remote_write exit, when the test keeps them.
	writeRelabel []string
}

// componentID is the sink's component ID.
func (g generatedSink) componentID() string { return "pipelinetest.sink." + g.label }

// sinkLabel names the sink replacing an exit, such as loki_write_default for
// loki.write.default.
func sinkLabel(exit string) string { return strings.ReplaceAll(exit, ".", "_") }

// sourceReplacement asks for an entry component to be replaced by a test
// input of the given signal.
type sourceReplacement struct {
	id     string
	signal signal
	// name is the label of the generated pipelinetest.source.
	name string
}

// transformSpec lists what transformConfig changes. A component ID can name
// a component inside a declare block as <module>/<component>, such as
// cluster_metrics/discovery.kubernetes.nodes.
type transformSpec struct {
	sources []sourceReplacement
	sinks   []string
	remove  []string
	// discovery maps a discovery component ID to the Alloy list literal that
	// replaces its targets export.
	discovery map[string]string
	// overrides maps a component ID to attribute names and the Alloy
	// expressions that replace their values.
	overrides map[string]map[string]string
	// writeRelabel keeps the write_relabel_config blocks of each
	// prometheus.remote_write exit in its sink.
	writeRelabel bool
}

func (s transformSpec) empty() bool {
	return len(s.sources) == 0 && len(s.sinks) == 0 && len(s.remove) == 0 && len(s.discovery) == 0 && len(s.overrides) == 0
}

// scope is the config's top level or the body of one declare block.
// Components in a module only see the components of the same module.
type scope struct {
	// path is "" for the top level, and the declare labels joined with /
	// for a module, such as cluster_metrics or outer/inner.
	path   string
	label  string
	parent *scope
	// stmts are the scope's statements, without the declare blocks inside
	// it. Those are scopes of their own.
	stmts  []ast.Stmt
	blocks map[string]*ast.BlockStmt
}

func (s *scope) describe() string {
	if s.path == "" {
		return "a top-level block of the config"
	}
	return "a component of module " + s.path
}

// instances returns the IDs of the module instances that run this scope,
// such as cluster_metrics.feature, or "" for the top level. A module's
// instances are the blocks that use it in the scope that declares it.
func (s *scope) instances() []string {
	if s.parent == nil {
		return []string{""}
	}
	var out []string
	for _, parent := range s.parent.instances() {
		for _, stmt := range s.parent.stmts {
			if b, ok := stmt.(*ast.BlockStmt); ok && b.GetBlockName() == s.label {
				id := blockID(b)
				if parent != "" {
					id = parent + "/" + id
				}
				out = append(out, id)
			}
		}
	}
	return out
}

// collectScopes returns the top-level scope and every declare block in it,
// at any depth, keyed by path.
func collectScopes(file *ast.File) map[string]*scope {
	scopes := map[string]*scope{}
	var walk func(path, label string, parent *scope, body ast.Body)
	walk = func(path, label string, parent *scope, body ast.Body) {
		s := &scope{path: path, label: label, parent: parent, blocks: map[string]*ast.BlockStmt{}}
		scopes[path] = s
		for _, stmt := range body {
			block, ok := stmt.(*ast.BlockStmt)
			if ok && block.GetBlockName() == "declare" {
				child := block.Label
				if path != "" {
					child = path + "/" + block.Label
				}
				walk(child, block.Label, s, block.Body)
				continue
			}
			s.stmts = append(s.stmts, stmt)
			if ok {
				s.blocks[blockID(block)] = block
			}
		}
	}
	walk("", "", nil, file.Body)
	return scopes
}

// splitModulePath splits an ID such as cluster_metrics/discovery.kubernetes.nodes
// into the module path and the component ID.
func splitModulePath(id string) (string, string) {
	i := strings.LastIndexByte(id, '/')
	if i < 0 {
		return "", id
	}
	return id[:i], id[i+1:]
}

// transformConfig applies spec to src. It edits the text in place, so
// comments, formatting and line numbers stay the same. A removed block
// becomes as many newlines as it spanned.
func transformConfig(name, src string, spec transformSpec) (transformedConfig, error) {
	out := transformedConfig{
		source:   src,
		sources:  map[string]replacedSource{},
		sinkRefs: map[string]string{},
	}
	if spec.empty() {
		return out, nil
	}
	sources, sinks, remove, discovery := spec.sources, spec.sinks, spec.remove, spec.discovery

	file, err := parser.ParseFile(name, []byte(src))
	if err != nil {
		return out, err
	}
	scopes := collectScopes(file)

	// find returns the block of a component, such as loki.write.default or
	// cluster_metrics/discovery.kubernetes.nodes. what says why it's
	// looked up, for the error.
	find := func(id, what string) (*scope, *ast.BlockStmt, error) {
		path, local := splitModulePath(id)
		s, ok := scopes[path]
		if !ok {
			modules := slices.DeleteFunc(slices.Sorted(maps.Keys(scopes)), func(p string) bool { return p == "" })
			return nil, nil, fmt.Errorf("component %s to %s: the config has no declare block %q; modules: %s", id, what, path, strings.Join(modules, ", "))
		}
		block, ok := s.blocks[local]
		if !ok {
			return nil, nil, fmt.Errorf("component %s to %s is not %s; components: %s", id, what, s.describe(), strings.Join(slices.Sorted(maps.Keys(s.blocks)), ", "))
		}
		return s, block, nil
	}

	var (
		edits   []textEdit
		removed []span
		seen    = map[string]string{}
	)
	lookup := func(id, role string) (*scope, *ast.BlockStmt, error) {
		if prev, ok := seen[id]; ok {
			return nil, nil, fmt.Errorf("component %s is listed twice, as %s and as %s", id, prev, role)
		}
		seen[id] = role
		return find(id, "replace with "+role)
	}

	// refParts maps a scope path to the components whose references are
	// rewritten in that scope.
	refParts := map[string]map[string]sinkTarget{}
	addRef := func(s *scope, local string, target sinkTarget) {
		if refParts[s.path] == nil {
			refParts[s.path] = map[string]sinkTarget{}
		}
		refParts[s.path][local] = target
	}

	for _, id := range sinks {
		s, block, err := lookup(id, "the sink")
		if err != nil {
			return out, err
		}
		if s.path != "" {
			return out, fmt.Errorf("sink %s: exits inside modules can't be replaced yet; pass the exit to the module as an argument", id)
		}
		removed = append(removed, blockSpan(block))
		gen := generatedSink{exit: id, label: sinkLabel(id)}
		target, err := sinkTargetFor(block.GetBlockName(), gen.componentID())
		if err != nil {
			return out, fmt.Errorf("sink %s: %w", id, err)
		}
		addRef(s, id, target)
		out.sinkRefs[id+"."+target.field] = target.replacement
		if attr := findAttribute(block.Body, "external_labels"); attr != nil {
			gen.externalLabels = src[ast.StartPos(attr.Value).Offset() : ast.EndPos(attr.Value).Offset()+1]
		}
		if spec.writeRelabel && block.GetBlockName() == "prometheus.remote_write" {
			if gen.writeRelabel, err = writeRelabelBlocks(src, block); err != nil {
				return out, fmt.Errorf("sink %s: %w", id, err)
			}
		}
		out.sinks = append(out.sinks, gen)
	}

	for _, id := range slices.Sorted(maps.Keys(discovery)) {
		s, block, err := lookup(id, "a targets input")
		if err != nil {
			return out, err
		}
		removed = append(removed, blockSpan(block))
		_, local := splitModulePath(id)
		addRef(s, local, sinkTarget{field: "targets", replacement: discovery[id]})
	}

	type entry struct {
		scope *scope
		block *ast.BlockStmt
		expr  ast.Expr
		spec  sourceReplacement
	}
	var entries []entry
	for _, r := range sources {
		s, block, err := lookup(r.id, "a "+string(r.signal)+" input")
		if err != nil {
			return out, err
		}
		expr, err := forwardToExpr(block, r.signal)
		if err != nil {
			return out, fmt.Errorf("input replacing %s: %w", r.id, err)
		}
		if s.path == "" {
			removed = append(removed, blockSpan(block))
		}
		entries = append(entries, entry{scope: s, block: block, expr: expr, spec: r})
	}

	for _, id := range remove {
		_, block, err := lookup(id, "nothing (remove)")
		if err != nil {
			return out, err
		}
		removed = append(removed, blockSpan(block))
	}

	var overridden []span
	for _, id := range slices.Sorted(maps.Keys(spec.overrides)) {
		if role, ok := seen[id]; ok {
			return out, fmt.Errorf("component %s is replaced with %s, so its arguments cannot be overridden", id, role)
		}
		_, block, err := find(id, "override")
		if err != nil {
			return out, err
		}
		attrs := spec.overrides[id]
		for _, name := range slices.Sorted(maps.Keys(attrs)) {
			attr := findAttribute(block.Body, name)
			if attr == nil {
				return out, fmt.Errorf("component %s does not set %s; override only replaces attributes the config sets", id, name)
			}
			start, end := ast.StartPos(attr.Value).Offset(), ast.EndPos(attr.Value).Offset()+1
			overridden = append(overridden, span{start: start, end: end})
			edits = append(edits, textEdit{start: start, end: end, text: attrs[name]})
		}
	}

	var refEdits []textEdit
	for _, path := range slices.Sorted(maps.Keys(refParts)) {
		e, err := referenceEdits(scopes[path].stmts, refParts[path])
		if err != nil {
			return out, err
		}
		refEdits = append(refEdits, e...)
	}

	for _, e := range entries {
		start, end := ast.StartPos(e.expr).Offset(), ast.EndPos(e.expr).Offset()+1
		forwardTo := applyEdits(src[start:end], start, refEdits)
		if e.scope.path == "" {
			out.sources[e.spec.id] = replacedSource{forwardTo: forwardTo}
			continue
		}
		// A module can't reference components outside it, so the input's
		// source goes where the replaced block was, on its first line.
		modules := e.scope.instances()
		if len(modules) == 0 {
			return out, fmt.Errorf("input replacing %s: nothing in the config uses module %s", e.spec.id, e.scope.path)
		}
		out.sources[e.spec.id] = replacedSource{forwardTo: forwardTo, modules: modules}
		r := blockSpan(e.block)
		removed = append(removed, r)
		text := fmt.Sprintf("pipelinetest.source %q { forward_to { %s = %s } }", e.spec.name, e.spec.signal, forwardTo)
		if pad := strings.Count(src[r.start:r.end], "\n") - strings.Count(forwardTo, "\n"); pad > 0 {
			text += strings.Repeat("\n", pad)
		}
		edits = append(edits, textEdit{start: r.start, end: r.end, text: text})
	}

	for _, e := range refEdits {
		if !insideAny(e.start, removed) && !insideAny(e.start, overridden) {
			edits = append(edits, e)
		}
	}
	for _, r := range removed {
		if slices.ContainsFunc(edits, func(e textEdit) bool { return e.start == r.start && e.end == r.end }) {
			continue
		}
		edits = append(edits, textEdit{start: r.start, end: r.end, text: strings.Repeat("\n", strings.Count(src[r.start:r.end], "\n"))})
	}

	out.source = applyEdits(src, 0, edits)
	return out, nil
}

// writeRelabelBlocks returns the source text of the write_relabel_config
// blocks of a prometheus.remote_write. It rejects more than one endpoint,
// because each endpoint has its own rules and the sink is one.
func writeRelabelBlocks(src string, block *ast.BlockStmt) ([]string, error) {
	var endpoints []*ast.BlockStmt
	for _, stmt := range block.Body {
		if b, ok := stmt.(*ast.BlockStmt); ok && b.GetBlockName() == "endpoint" {
			endpoints = append(endpoints, b)
		}
	}
	if len(endpoints) > 1 {
		return nil, fmt.Errorf("write_relabel needs one endpoint block, the exit has %d", len(endpoints))
	}
	var out []string
	for _, ep := range endpoints {
		for _, stmt := range ep.Body {
			if b, ok := stmt.(*ast.BlockStmt); ok && b.GetBlockName() == "write_relabel_config" {
				sp := blockSpan(b)
				out = append(out, src[sp.start:sp.end])
			}
		}
	}
	return out, nil
}

// forwardToExpr returns the expression an entry component sends its data to:
// forward_to for Loki and Prometheus components, and output.traces for
// OpenTelemetry receivers.
func forwardToExpr(block *ast.BlockStmt, sig signal) (ast.Expr, error) {
	body := block.Body
	attr := "forward_to"
	if sig == signalTraces {
		output := findBlock(body, "output")
		if output == nil {
			return nil, fmt.Errorf("%s has no output block to take the traces destination from", blockID(block))
		}
		body, attr = output.Body, "traces"
	}

	for _, stmt := range body {
		if a, ok := stmt.(*ast.AttributeStmt); ok && a.Name.Name == attr {
			return a.Value, nil
		}
	}
	return nil, fmt.Errorf("%s has no %s attribute to take the %s destination from", blockID(block), attr, sig)
}

func findAttribute(body ast.Body, name string) *ast.AttributeStmt {
	for _, stmt := range body {
		if a, ok := stmt.(*ast.AttributeStmt); ok && a.Name.Name == name {
			return a
		}
	}
	return nil
}

func findBlock(body ast.Body, name string) *ast.BlockStmt {
	for _, stmt := range body {
		if b, ok := stmt.(*ast.BlockStmt); ok && b.GetBlockName() == name {
			return b
		}
	}
	return nil
}

// referenceEdits finds every reference in stmts to an export of a replaced
// component, such as loki.write.default.receiver, and returns the edits
// replacing it.
func referenceEdits(stmts []ast.Stmt, targets map[string]sinkTarget) ([]textEdit, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	v := &refVisitor{sinks: targets}
	for _, stmt := range stmts {
		ast.Walk(v, stmt)
	}
	return v.edits, v.err
}

type refVisitor struct {
	sinks map[string]sinkTarget
	edits []textEdit
	err   error
}

func (v *refVisitor) Visit(node ast.Node) ast.Visitor {
	if v.err != nil {
		return nil
	}
	access, ok := node.(*ast.AccessExpr)
	if !ok {
		return v
	}
	parts, ok := identChain(access)
	if !ok || len(parts) < 2 {
		return v
	}

	id, field := strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
	target, ok := v.sinks[id]
	if !ok {
		return v
	}
	if field != target.field {
		v.err = fmt.Errorf("the config references %s.%s, but replacing %s only replaces %s.%s", id, field, id, id, target.field)
		return nil
	}
	v.edits = append(v.edits, textEdit{
		start: ast.StartPos(access).Offset(),
		end:   ast.EndPos(access).Offset() + 1,
		text:  target.replacement,
	})
	return nil
}

// identChain flattens an access expression made only of identifiers, such as
// loki.write.default.receiver, into its parts.
func identChain(expr ast.Expr) ([]string, bool) {
	switch e := expr.(type) {
	case *ast.IdentifierExpr:
		return []string{e.Ident.Name}, true
	case *ast.AccessExpr:
		parts, ok := identChain(e.Value)
		if !ok {
			return nil, false
		}
		return append(parts, e.Name.Name), true
	default:
		return nil, false
	}
}

func blockID(block *ast.BlockStmt) string {
	if block.Label == "" {
		return block.GetBlockName()
	}
	return block.GetBlockName() + "." + block.Label
}

type span struct{ start, end int }

func blockSpan(block *ast.BlockStmt) span {
	return span{start: ast.StartPos(block).Offset(), end: ast.EndPos(block).Offset() + 1}
}

func insideAny(off int, spans []span) bool {
	for _, s := range spans {
		if off >= s.start && off < s.end {
			return true
		}
	}
	return false
}

type textEdit struct {
	start, end int
	text       string
}

// applyEdits applies the edits that fall within text, which starts at offset
// base of the original source. Edits must not overlap.
func applyEdits(text string, base int, edits []textEdit) string {
	inRange := make([]textEdit, 0, len(edits))
	for _, e := range edits {
		if e.start >= base && e.end <= base+len(text) {
			inRange = append(inRange, e)
		}
	}
	slices.SortFunc(inRange, func(a, b textEdit) int { return b.start - a.start })

	for _, e := range inRange {
		text = text[:e.start-base] + e.text + text[e.end-base:]
	}
	return text
}

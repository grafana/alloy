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
}

// transformSpec lists what transformConfig changes.
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
}

func (s transformSpec) empty() bool {
	return len(s.sources) == 0 && len(s.sinks) == 0 && len(s.remove) == 0 && len(s.discovery) == 0 && len(s.overrides) == 0
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

	blocks := map[string]*ast.BlockStmt{}
	for _, stmt := range file.Body {
		if block, ok := stmt.(*ast.BlockStmt); ok {
			blocks[blockID(block)] = block
		}
	}

	var (
		edits   []textEdit
		removed []span
		seen    = map[string]string{}
	)
	lookup := func(id, role string) (*ast.BlockStmt, error) {
		if prev, ok := seen[id]; ok {
			return nil, fmt.Errorf("component %s is listed twice, as %s and as %s", id, prev, role)
		}
		seen[id] = role
		block, ok := blocks[id]
		if !ok {
			return nil, fmt.Errorf("component %s to replace with %s is not a top-level block of the config; components: %s", id, role, strings.Join(slices.Sorted(maps.Keys(blocks)), ", "))
		}
		removed = append(removed, blockSpan(block))
		return block, nil
	}

	sinkParts := map[string]sinkTarget{}
	for _, id := range sinks {
		block, err := lookup(id, "the sink")
		if err != nil {
			return out, err
		}
		gen := generatedSink{exit: id, label: sinkLabel(id)}
		target, err := sinkTargetFor(block.GetBlockName(), gen.componentID())
		if err != nil {
			return out, fmt.Errorf("sink %s: %w", id, err)
		}
		sinkParts[id] = target
		out.sinkRefs[id+"."+target.field] = target.replacement
		if attr := findAttribute(block.Body, "external_labels"); attr != nil {
			gen.externalLabels = src[ast.StartPos(attr.Value).Offset() : ast.EndPos(attr.Value).Offset()+1]
		}
		out.sinks = append(out.sinks, gen)
	}

	for _, id := range slices.Sorted(maps.Keys(discovery)) {
		if _, err := lookup(id, "a targets input"); err != nil {
			return out, err
		}
		sinkParts[id] = sinkTarget{field: "targets", replacement: discovery[id]}
	}

	forwardExprs := map[string]ast.Expr{}
	for _, s := range sources {
		block, err := lookup(s.id, "a "+string(s.signal)+" input")
		if err != nil {
			return out, err
		}
		expr, err := forwardToExpr(block, s.signal)
		if err != nil {
			return out, fmt.Errorf("input replacing %s: %w", s.id, err)
		}
		forwardExprs[s.id] = expr
	}

	for _, id := range remove {
		if _, err := lookup(id, "nothing (remove)"); err != nil {
			return out, err
		}
	}

	var overridden []span
	for _, id := range slices.Sorted(maps.Keys(spec.overrides)) {
		if role, ok := seen[id]; ok {
			return out, fmt.Errorf("component %s is replaced with %s, so its arguments cannot be overridden", id, role)
		}
		block, ok := blocks[id]
		if !ok {
			return out, fmt.Errorf("component %s to override is not a top-level block of the config", id)
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

	refEdits, err := sinkReferenceEdits(file, sinkParts)
	if err != nil {
		return out, err
	}

	for id, expr := range forwardExprs {
		start, end := ast.StartPos(expr).Offset(), ast.EndPos(expr).Offset()+1
		out.sources[id] = replacedSource{forwardTo: applyEdits(src[start:end], start, refEdits)}
	}

	for _, e := range refEdits {
		if !insideAny(e.start, removed) && !insideAny(e.start, overridden) {
			edits = append(edits, e)
		}
	}
	for _, r := range removed {
		edits = append(edits, textEdit{start: r.start, end: r.end, text: strings.Repeat("\n", strings.Count(src[r.start:r.end], "\n"))})
	}

	out.source = applyEdits(src, 0, edits)
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

// sinkReferenceEdits finds every reference to a sink component's export, such
// as loki.write.default.receiver, and returns the edits replacing it with the
// sink's export.
func sinkReferenceEdits(file *ast.File, sinks map[string]sinkTarget) ([]textEdit, error) {
	if len(sinks) == 0 {
		return nil, nil
	}
	v := &refVisitor{sinks: sinks}
	ast.Walk(v, file)
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

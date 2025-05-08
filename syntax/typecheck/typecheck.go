package typecheck

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/internal/reflectutil"
	"github.com/grafana/alloy/syntax/internal/syntaxtags"
)

type state struct {
	tags       map[string]syntaxtags.Field
	seenAttrs  map[string]struct{}
	blockCount map[string]int
}

func Block(b *ast.BlockStmt, args any) diag.Diagnostics {
	rv := reflectutil.DeferencePointer(reflect.ValueOf(args))
	return block(b, rv)
}

func block(b *ast.BlockStmt, rv reflect.Value) diag.Diagnostics {
	var diags diag.Diagnostics

	s := state{
		tags:       getTags(rv.Type()),
		seenAttrs:  make(map[string]struct{}),
		blockCount: make(map[string]int),
	}

	for _, stmt := range b.Body {
		switch stmt := stmt.(type) {
		case *ast.BlockStmt:
			name := strings.Join(stmt.Name, ".")
			s.blockCount[name]++
		}
	}

	// FIXME(kallep): When we start to type check properties we need to check if block implements a couple of interfaces
	// and handle them properly.
	// - value.Defaulter
	// - value.Unmarshaler
	// - value.ConvertibleFromCapsule
	// - value.ConvertibleIntoCapsule
	// - encoding.TextUnmarshaler
	for _, stmt := range b.Body {
		switch n := stmt.(type) {
		case *ast.BlockStmt:
			diags.Merge(checkBlock(&s, n, rv))
		case *ast.AttributeStmt:
			diags.Merge(checkAttr(&s, n, rv))
		default:
			panic(fmt.Sprintf("syntax/vm: unrecognized node type %T", stmt))
		}
	}

	for name, t := range s.tags {
		if t.IsOptional() {
			continue
		}

		if t.IsAttr() {
			if _, ok := s.seenAttrs[name]; !ok {
				diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					StartPos: ast.StartPos(b).Position(),
					EndPos:   ast.EndPos(b).Position(),
					Message:  fmt.Sprintf("missing required attribute %q", name),
				})
			}
			continue
		}

		if t.IsBlock() {
			if _, ok := s.blockCount[name]; !ok {
				diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					StartPos: ast.StartPos(b).Position(),
					EndPos:   ast.EndPos(b).Position(),
					Message:  fmt.Sprintf("missing required block %q", name),
				})
			}
			continue
		}
	}

	return diags
}

func checkBlock(s *state, b *ast.BlockStmt, rv reflect.Value) diag.Diagnostics {
	name := strings.Join(b.Name, ".")
	tag, ok := s.tags[name]
	if !ok {
		return diag.Diagnostics{{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(b).Position(),
			EndPos:   ast.EndPos(b).Position(),
			Message:  fmt.Sprintf("unrecognized block name %q", name),
		}}
	} else if tag.IsAttr() {
		return diag.Diagnostics{{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(b).Position(),
			EndPos:   ast.EndPos(b).Position(),
			Message:  fmt.Sprintf("%q must be an attribute, but is used as a block", name),
		}}
	}

	field := reflectutil.GetOrAlloc(rv, tag)

	switch field.Kind() {
	case reflect.Slice:
		return block(b, reflectutil.DeferencePointer(field.Index(0)))
	case reflect.Array:
		if field.Len() != s.blockCount[name] {
			return diag.Diagnostics{{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(b).Position(),
				EndPos:   ast.EndPos(b).Position(),
				Message: fmt.Sprintf(
					"block %q must be specified exactly %d times, but was specified %d times",
					name,
					field.Len(),
					s.blockCount[name],
				),
			}}
		}

		return block(b, reflectutil.DeferencePointer(field.Index(0)))
	default:
		if s.blockCount[name] > 1 {
			return diag.Diagnostics{{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(b).Position(),
				EndPos:   ast.EndPos(b).Position(),
				Message:  fmt.Sprintf("block %q may only be specified once", name),
			}}
		}
		return block(b, reflectutil.DeferencePointer(field))
	}

}

func checkAttr(s *state, a *ast.AttributeStmt, _ reflect.Value) diag.Diagnostics {
	tf, ok := s.tags[a.Name.Name]
	if !ok {
		return diag.Diagnostics{{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(a).Position(),
			EndPos:   ast.EndPos(a).Position(),
			Message:  fmt.Sprintf("unrecognized attribute name %q", a.Name.Name),
		}}
	} else if tf.IsBlock() {
		return diag.Diagnostics{{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(a).Position(),
			EndPos:   ast.EndPos(a).Position(),
			Message:  fmt.Sprintf("%q must be a block, but is used as an attribute", a.Name.Name),
		}}
	}

	if _, seen := s.seenAttrs[a.Name.Name]; seen {
		return diag.Diagnostics{{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(a).Position(),
			EndPos:   ast.EndPos(a).Position(),
			Message:  fmt.Sprintf("attribute %q may only be provided once", a.Name.Name),
		}}
	}

	s.seenAttrs[a.Name.Name] = struct{}{}

	return nil
}

func getTags(t reflect.Type) map[string]syntaxtags.Field {
	tags := syntaxtags.Get(t)

	// FIXME: how should we handle enums, they seem to have some special handling
	m := make(map[string]syntaxtags.Field, len(tags))
	for _, tag := range tags {
		m[strings.Join(tag.Name, ".")] = tag
	}

	return m
}

// Fixme remove and simplify
func prepareDecodeValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}

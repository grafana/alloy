package typecheck

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/internal/reflectutil"
	"github.com/grafana/alloy/syntax/internal/tagcache"
	"github.com/grafana/alloy/syntax/internal/transform"
	"github.com/grafana/alloy/syntax/internal/value"
)

type structState struct {
	tags       *tagcache.TagInfo
	seenAttrs  map[string]struct{}
	blockCount map[string]int
}

func Block(b *ast.BlockStmt, args any) diag.Diagnostics {
	rv := reflectutil.DeferencePointer(reflect.ValueOf(args))
	return block(b, rv)
}

func block(b *ast.BlockStmt, rv reflect.Value) diag.Diagnostics {
	var diags diag.Diagnostics

	switch rv.Kind() {
	case reflect.Map:
		return checkMapBlock(b, rv)
	case reflect.Interface:
		var m map[string]any
		rv := reflect.MakeMap(reflect.TypeOf(m))
		return checkMapBlock(b, rv)
	case reflect.Struct:
		s := structState{
			tags:       tagcache.Get(rv.Type()),
			seenAttrs:  make(map[string]struct{}),
			blockCount: make(map[string]int),
		}

		for _, stmt := range b.Body {
			switch stmt := stmt.(type) {
			case *ast.BlockStmt:
				s.blockCount[stmt.GetBlockName()]++
			}
		}

		// FIXME(kallep): When we start to check that correct types are set for properties we most likely need to
		// consider these interfaces.
		// - value.Defaulter
		// - value.Unmarshaler
		// - value.ConvertibleFromCapsule
		// - value.ConvertibleIntoCapsule
		// - encoding.TextUnmarshaler
		for _, stmt := range b.Body {
			switch n := stmt.(type) {
			case *ast.BlockStmt:
				diags.Merge(checkStructBlock(&s, n, rv))
			case *ast.AttributeStmt:
				diags.Merge(checkStructAttr(&s, n, rv))
			default:
				panic(fmt.Sprintf("syntax/vm: unrecognized node type %T", stmt))
			}
		}

		for _, t := range s.tags.Tags {
			if t.IsOptional() {
				continue
			}

			name := strings.Join(t.Name, ".")
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
	default:
		panic(fmt.Sprintf("syntax/typecheck: can only type check arguments of type struct, map and interface, got %s", rv.Kind()))
	}

	return diags
}

// FIXME(kalleep): currently we ignore block maps
func checkMapBlock(b *ast.BlockStmt, _ reflect.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	if b.Label != "" {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: b.NamePos.Position(),
			EndPos:   b.LCurlyPos.Position(),
			Message:  fmt.Sprintf("block %q requires empty label", b.GetBlockName()),
		})
	}
	return diags
}

func checkStructBlock(s *structState, b *ast.BlockStmt, rv reflect.Value) diag.Diagnostics {
	name := b.GetBlockName()
	if _, ok := s.tags.EnumLookup[name]; ok {
		return checkStructEnum(s, b, rv)
	}

	tag, ok := s.tags.TagLookup[name]
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
		// NOTE: we do not need to store any values so we can always set len and cap to 1 and reuse the same slot
		field.Set(reflect.MakeSlice(field.Type(), 1, 1))
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

func checkStructEnum(s *structState, b *ast.BlockStmt, rv reflect.Value) diag.Diagnostics {
	tf, ok := s.tags.EnumLookup[b.GetBlockName()]
	if !ok {
		panic("checkEnum called with a non-enum block")
	}

	field := reflectutil.GetOrAlloc(rv, tf.EnumField)
	if field.Kind() != reflect.Slice {
		panic("checkEnum: enum field must be a slice kind, got " + field.Kind().String())
	}
	// NOTE: we do not need to store any values so we can always set len and cap to 1 and reuse the same slot
	field.Set(reflect.MakeSlice(field.Type(), 1, 1))

	elem := reflectutil.DeferencePointer(field.Index(0))

	return block(b, reflectutil.DeferencePointer(reflectutil.GetOrAlloc(elem, tf.BlockField)))
}

func checkStructAttr(s *structState, a *ast.AttributeStmt, rv reflect.Value) diag.Diagnostics {
	tf, ok := s.tags.TagLookup[a.Name.Name]
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

	var diags diag.Diagnostics

	switch expr := a.Value.(type) {
	case *ast.ArrayExpr:
		diags.Merge(typecheckArrayExpr(a.Name.Name, expr, reflectutil.GetOrAlloc(rv, tf)))
	case *ast.ObjectExpr:
		diags.Merge(typecheckObject(a.Name.Name, expr, reflectutil.GetOrAlloc(rv, tf)))
	case *ast.LiteralExpr:
		if d := typecheckLiteralExpr(a.Name.Name, expr, reflectutil.GetOrAlloc(rv, tf)); d != nil {
			diags.Add(*d)
		}
	default:
		// ignore rest for now.
	}

	if diags != nil {
		return diags
	}

	return nil
}

func typecheckArrayExpr(name string, expr *ast.ArrayExpr, rv reflect.Value) diag.Diagnostics {
	// NOTE: we do not need to store any values so we can always set len and cap to 1 and reuse the same slot.
	rv.Set(reflect.MakeSlice(rv.Type(), 1, 1))
	// Extract the expected item.
	expected := reflectutil.DeferencePointer(rv.Index(0))

	var diags diag.Diagnostics
	for _, e := range expr.Elements {
		switch expr := e.(type) {
		case *ast.LiteralExpr:
			if d := typecheckLiteralExpr(name, expr, expected); d != nil {
				diags.Add(*d)
			}
		case *ast.ArrayExpr:
			diags.Merge(typecheckArrayExpr(name, expr, expected))
		default:
			// ignore rest for now.
		}
	}

	if diags != nil {
		return diags
	}

	return nil
}

func typecheckObject(name string, expr *ast.ObjectExpr, rv reflect.Value) diag.Diagnostics {
	expected := reflectutil.DeferencePointer(reflect.New(rv.Type().Elem()))

	var diags diag.Diagnostics
	for _, f := range expr.Fields {
		switch expr := f.Value.(type) {
		case *ast.LiteralExpr:
			if d := typecheckLiteralExpr(name, expr, expected); d != nil {
				diags.Add(*d)
			}
		case *ast.ArrayExpr:
			diags.Merge(typecheckArrayExpr(name, expr, expected))
		case *ast.ObjectExpr:
			diags.Merge(typecheckObject(name, expr, expected))
		default:
			// ignore rest for now.
		}
	}

	if diags != nil {
		return diags
	}

	return nil
}

func typecheckLiteralExpr(name string, expr *ast.LiteralExpr, rv reflect.Value) *diag.Diagnostic {
	have, err := transform.ValueFromLiteral(expr.Value, expr.Kind)

	// We don't expect to get error here because parser always produce valid tokens.
	if err != nil {
		return &diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(expr).Position(),
			EndPos:   ast.EndPos(expr).Position(),
			Message:  fmt.Sprintf("unexpected err: %s", err),
		}
	}

	expected := value.AlloyType(rv.Type())
	if expected == value.TypeCapsule {
		ok, _ := value.TryCapsuleConvert(have, rv, expected)
		// FIXME(kalleep): We should probably unwrap the capsule type.
		if ok {
			return nil
		}

		return &diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(expr).Position(),
			EndPos:   ast.EndPos(expr).Position(),
			Message:  fmt.Sprintf("%q should be %s, got %s", name, expected, have.Type()),
		}
	}

	if have.Type() != expected {
		return &diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(expr).Position(),
			EndPos:   ast.EndPos(expr).Position(),
			Message:  fmt.Sprintf("%q should be %s, got %s", name, expected, have.Type()),
		}
	}

	return nil
}

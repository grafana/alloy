// Package vm provides a River expression evaluator.
package vm

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/river/ast"
	"github.com/grafana/river/diag"
	"github.com/grafana/river/internal/reflectutil"
	"github.com/grafana/river/internal/rivertags"
	"github.com/grafana/river/internal/stdlib"
	"github.com/grafana/river/internal/value"
)

// Evaluator evaluates River AST nodes into Go values. Each Evaluator is bound
// to a single AST node. To evaluate the node, call Evaluate.
type Evaluator struct {
	// node for the AST.
	//
	// Each Evaluator is bound to a single node to allow for future performance
	// optimizations, allowing for precomputing and storing the result of
	// anything that is constant.
	node ast.Node
}

// New creates a new Evaluator for the given AST node. The given node must be
// either an *ast.File, *ast.BlockStmt, ast.Body, or assignable to an ast.Expr.
func New(node ast.Node) *Evaluator {
	return &Evaluator{node: node}
}

// Evaluate evaluates the Evaluator's node into a River value and decodes that
// value into the Go value v.
//
// Each call to Evaluate may provide a different scope with new values for
// available variables. If a variable used by the Evaluator's node isn't
// defined in scope or any of the parent scopes, Evaluate will return an error.
func (vm *Evaluator) Evaluate(scope *Scope, v interface{}) (err error) {
	// Track a map that allows us to associate values with ast.Nodes so we can
	// return decorated error messages.
	assoc := make(map[value.Value]ast.Node)

	defer func() {
		if err != nil {
			// Decorate the error on return.
			err = makeDiagnostic(err, assoc)
		}
	}()

	switch node := vm.node.(type) {
	case *ast.BlockStmt, ast.Body:
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Pointer {
			panic(fmt.Sprintf("river/vm: expected pointer, got %s", rv.Kind()))
		}
		return vm.evaluateBlockOrBody(scope, assoc, node, rv)
	case *ast.File:
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Pointer {
			panic(fmt.Sprintf("river/vm: expected pointer, got %s", rv.Kind()))
		}
		return vm.evaluateBlockOrBody(scope, assoc, node.Body, rv)
	default:
		expr, ok := node.(ast.Expr)
		if !ok {
			panic(fmt.Sprintf("river/vm: unexpected value type %T", node))
		}
		val, err := vm.evaluateExpr(scope, assoc, expr)
		if err != nil {
			return err
		}
		return value.Decode(val, v)
	}
}

func (vm *Evaluator) evaluateBlockOrBody(scope *Scope, assoc map[value.Value]ast.Node, node ast.Node, rv reflect.Value) error {
	// Before decoding the block, we need to temporarily take the address of rv
	// to handle the case of it implementing the unmarshaler interface.
	if rv.CanAddr() {
		rv = rv.Addr()
	}

	if err, unmarshaled := vm.evaluateUnmarshalRiver(scope, assoc, node, rv); unmarshaled || err != nil {
		return err
	}

	if ru, ok := rv.Interface().(value.Defaulter); ok {
		ru.SetToDefault()
	}

	if err := vm.evaluateDecode(scope, assoc, node, rv); err != nil {
		return err
	}

	if ru, ok := rv.Interface().(value.Validator); ok {
		if err := ru.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (vm *Evaluator) evaluateUnmarshalRiver(scope *Scope, assoc map[value.Value]ast.Node, node ast.Node, rv reflect.Value) (error, bool) {
	if ru, ok := rv.Interface().(value.Unmarshaler); ok {
		return ru.UnmarshalRiver(func(v interface{}) error {
			rv := reflect.ValueOf(v)
			if rv.Kind() != reflect.Pointer {
				panic(fmt.Sprintf("river/vm: expected pointer, got %s", rv.Kind()))
			}
			return vm.evaluateBlockOrBody(scope, assoc, node, rv.Elem())
		}), true
	}

	return nil, false
}

func (vm *Evaluator) evaluateDecode(scope *Scope, assoc map[value.Value]ast.Node, node ast.Node, rv reflect.Value) error {
	// TODO(rfratto): the errors returned by this function are missing context to
	// be able to print line numbers. We need to return decorated error types.

	// Fully deference rv and allocate pointers as necessary.
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Interface {
		var anyMap map[string]interface{}
		into := reflect.MakeMap(reflect.TypeOf(anyMap))
		if err := vm.evaluateMap(scope, assoc, node, into); err != nil {
			return err
		}

		rv.Set(into)
		return nil
	} else if rv.Kind() == reflect.Map {
		return vm.evaluateMap(scope, assoc, node, rv)
	} else if rv.Kind() != reflect.Struct {
		panic(fmt.Sprintf("river/vm: can only evaluate blocks into structs, got %s", rv.Kind()))
	}

	ti := getCachedTagInfo(rv.Type())

	var stmts ast.Body
	switch node := node.(type) {
	case *ast.BlockStmt:
		// Decode the block label first.
		if err := vm.evaluateBlockLabel(node, ti.Tags, rv); err != nil {
			return err
		}
		stmts = node.Body
	case ast.Body:
		stmts = node
	default:
		panic(fmt.Sprintf("river/vm: unrecognized node type %T", node))
	}

	sd := structDecoder{
		VM:      vm,
		Scope:   scope,
		Assoc:   assoc,
		TagInfo: ti,
	}
	return sd.Decode(stmts, rv)
}

// evaluateMap evaluates a block or a body into a map.
func (vm *Evaluator) evaluateMap(scope *Scope, assoc map[value.Value]ast.Node, node ast.Node, rv reflect.Value) error {
	var stmts ast.Body

	switch node := node.(type) {
	case *ast.BlockStmt:
		if node.Label != "" {
			return diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: node.NamePos.Position(),
				EndPos:   node.LCurlyPos.Position(),
				Message:  fmt.Sprintf("block %q requires non-empty label", strings.Join(node.Name, ".")),
			}
		}
		stmts = node.Body
	case ast.Body:
		stmts = node
	default:
		panic(fmt.Sprintf("river/vm: unrecognized node type %T", node))
	}

	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}

	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case *ast.AttributeStmt:
			val, err := vm.evaluateExpr(scope, assoc, stmt.Value)
			if err != nil {
				// TODO(rfratto): get error as diagnostics.
				return err
			}

			target := reflect.New(rv.Type().Elem()).Elem()
			if err := value.Decode(val, target.Addr().Interface()); err != nil {
				// TODO(rfratto): get error as diagnostics.
				return err
			}
			rv.SetMapIndex(reflect.ValueOf(stmt.Name.Name), target)

		case *ast.BlockStmt:
			// TODO(rfratto): potentially relax this restriction where nested blocks
			// are permitted when decoding to a map.
			return diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(stmt).Position(),
				EndPos:   ast.EndPos(stmt).Position(),
				Message:  "nested blocks not supported here",
			}

		default:
			panic(fmt.Sprintf("river/vm: unrecognized node type %T", stmt))
		}
	}

	return nil
}

func (vm *Evaluator) evaluateBlockLabel(node *ast.BlockStmt, tfs []rivertags.Field, rv reflect.Value) error {
	var (
		labelField rivertags.Field
		foundField bool
	)
	for _, tf := range tfs {
		if tf.Flags&rivertags.FlagLabel != 0 {
			labelField = tf
			foundField = true
			break
		}
	}

	// Check for user errors first.
	//
	// We return parser.Error here to restrict the position of the error to just
	// the name. We might be able to clean this up in the future by extending
	// ValueError to have an explicit position.
	switch {
	case node.Label == "" && foundField: // No user label, but struct expects one
		return diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: node.NamePos.Position(),
			EndPos:   node.LCurlyPos.Position(),
			Message:  fmt.Sprintf("block %q requires non-empty label", strings.Join(node.Name, ".")),
		}
	case node.Label != "" && !foundField: // User label, but struct doesn't expect one
		return diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: node.NamePos.Position(),
			EndPos:   node.LCurlyPos.Position(),
			Message:  fmt.Sprintf("block %q does not support specifying labels", strings.Join(node.Name, ".")),
		}
	}

	if node.Label == "" {
		// no-op: no labels to set.
		return nil
	}

	var (
		field     = reflectutil.GetOrAlloc(rv, labelField)
		fieldType = field.Type()
	)
	if !reflect.TypeOf(node.Label).AssignableTo(fieldType) {
		// The Label struct field needs to be a string.
		panic(fmt.Sprintf("river/vm: cannot assign block label to non-string type %s", fieldType))
	}
	field.Set(reflect.ValueOf(node.Label))
	return nil
}

// prepareDecodeValue prepares v for decoding. Pointers will be fully
// dereferenced until finding a non-pointer value. nil pointers will be
// allocated.
func prepareDecodeValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	return v
}

func (vm *Evaluator) evaluateExpr(scope *Scope, assoc map[value.Value]ast.Node, expr ast.Expr) (v value.Value, err error) {
	defer func() {
		if v != value.Null {
			assoc[v] = expr
		}
	}()

	switch expr := expr.(type) {
	case *ast.LiteralExpr:
		return valueFromLiteral(expr.Value, expr.Kind)

	case *ast.BinaryExpr:
		lhs, err := vm.evaluateExpr(scope, assoc, expr.Left)
		if err != nil {
			return value.Null, err
		}
		rhs, err := vm.evaluateExpr(scope, assoc, expr.Right)
		if err != nil {
			return value.Null, err
		}
		return evalBinop(lhs, expr.Kind, rhs)

	case *ast.ArrayExpr:
		vals := make([]value.Value, len(expr.Elements))
		for i, element := range expr.Elements {
			val, err := vm.evaluateExpr(scope, assoc, element)
			if err != nil {
				return value.Null, err
			}
			vals[i] = val
		}
		return value.Array(vals...), nil

	case *ast.ObjectExpr:
		fields := make(map[string]value.Value, len(expr.Fields))
		for _, field := range expr.Fields {
			val, err := vm.evaluateExpr(scope, assoc, field.Value)
			if err != nil {
				return value.Null, err
			}
			fields[field.Name.Name] = val
		}
		return value.Object(fields), nil

	case *ast.IdentifierExpr:
		val, found := scope.Lookup(expr.Ident.Name)
		if !found {
			return value.Null, diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(expr).Position(),
				EndPos:   ast.EndPos(expr).Position(),
				Message:  fmt.Sprintf("identifier %q does not exist", expr.Ident.Name),
			}
		}
		return value.Encode(val), nil

	case *ast.AccessExpr:
		val, err := vm.evaluateExpr(scope, assoc, expr.Value)
		if err != nil {
			return value.Null, err
		}

		switch val.Type() {
		case value.TypeObject:
			res, ok := val.Key(expr.Name.Name)
			if !ok {
				return value.Null, diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					StartPos: ast.StartPos(expr.Name).Position(),
					EndPos:   ast.EndPos(expr.Name).Position(),
					Message:  fmt.Sprintf("field %q does not exist", expr.Name.Name),
				}
			}
			return res, nil
		default:
			return value.Null, value.Error{
				Value: val,
				Inner: fmt.Errorf("cannot access field %q on value of type %s", expr.Name.Name, val.Type()),
			}
		}

	case *ast.IndexExpr:
		val, err := vm.evaluateExpr(scope, assoc, expr.Value)
		if err != nil {
			return value.Null, err
		}
		idx, err := vm.evaluateExpr(scope, assoc, expr.Index)
		if err != nil {
			return value.Null, err
		}

		switch val.Type() {
		case value.TypeArray:
			// Arrays are indexed with a number.
			if idx.Type() != value.TypeNumber {
				return value.Null, value.TypeError{Value: idx, Expected: value.TypeNumber}
			}
			intIndex := int(idx.Int())

			if intIndex < 0 || intIndex >= val.Len() {
				return value.Null, value.Error{
					Value: idx,
					Inner: fmt.Errorf("index %d is out of range of array with length %d", intIndex, val.Len()),
				}
			}
			return val.Index(intIndex), nil

		case value.TypeObject:
			// Objects are indexed with a string.
			if idx.Type() != value.TypeString {
				return value.Null, value.TypeError{Value: idx, Expected: value.TypeString}
			}

			field, ok := val.Key(idx.Text())
			if !ok {
				// If a key doesn't exist in an object accessed with [], return null.
				return value.Null, nil
			}
			return field, nil

		default:
			return value.Null, value.Error{
				Value: val,
				Inner: fmt.Errorf("expected object or array, got %s", val.Type()),
			}
		}

	case *ast.ParenExpr:
		return vm.evaluateExpr(scope, assoc, expr.Inner)

	case *ast.UnaryExpr:
		val, err := vm.evaluateExpr(scope, assoc, expr.Value)
		if err != nil {
			return value.Null, err
		}
		return evalUnaryOp(expr.Kind, val)

	case *ast.CallExpr:
		funcVal, err := vm.evaluateExpr(scope, assoc, expr.Value)
		if err != nil {
			return funcVal, err
		}
		if funcVal.Type() != value.TypeFunction {
			return value.Null, value.TypeError{Value: funcVal, Expected: value.TypeFunction}
		}

		args := make([]value.Value, len(expr.Args))
		for i := 0; i < len(expr.Args); i++ {
			args[i], err = vm.evaluateExpr(scope, assoc, expr.Args[i])
			if err != nil {
				return value.Null, err
			}
		}
		return funcVal.Call(args...)

	default:
		panic(fmt.Sprintf("river/vm: unexpected ast.Expr type %T", expr))
	}
}

// A Scope exposes a set of variables available to use during evaluation.
type Scope struct {
	// Parent optionally points to a parent Scope containing more variable.
	// Variables defined in children scopes take precedence over variables of the
	// same name found in parent scopes.
	Parent *Scope

	// Variables holds the list of available variable names that can be used when
	// evaluating a node.
	//
	// Values in the Variables map should be considered immutable after passed to
	// Evaluate; maps and slices will be copied by reference for performance
	// optimizations.
	Variables map[string]interface{}
}

// Lookup looks up a named identifier from the scope, all of the scope's
// parents, and the stdlib.
func (s *Scope) Lookup(name string) (interface{}, bool) {
	// Traverse the scope first, then fall back to stdlib.
	for s != nil {
		if val, ok := s.Variables[name]; ok {
			return val, true
		}
		s = s.Parent
	}
	if ident, ok := stdlib.Identifiers[name]; ok {
		return ident, true
	}
	return nil, false
}

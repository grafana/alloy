package vm

import (
	"fmt"
	"strings"

	"github.com/grafana/river/ast"
	"github.com/grafana/river/diag"
	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/printer"
	"github.com/grafana/river/token/builder"
)

// makeDiagnostic tries to convert err into a diag.Diagnostic. err must be an
// error from the river/internal/value package, otherwise err will be returned
// unmodified.
func makeDiagnostic(err error, assoc map[value.Value]ast.Node) error {
	var (
		node    ast.Node
		expr    strings.Builder
		message string
		cause   value.Value

		// Until we find a node, we're not a literal error.
		literal = false
	)

	isValueError := value.WalkError(err, func(err error) {
		var val value.Value

		switch ne := err.(type) {
		case value.Error:
			message = ne.Error()
			val = ne.Value
		case value.TypeError:
			message = fmt.Sprintf("should be %s, got %s", ne.Expected, ne.Value.Type())
			val = ne.Value
		case value.MissingKeyError:
			message = fmt.Sprintf("does not have field named %q", ne.Missing)
			val = ne.Value
		case value.ElementError:
			fmt.Fprintf(&expr, "[%d]", ne.Index)
			val = ne.Value
		case value.FieldError:
			fmt.Fprintf(&expr, ".%s", ne.Field)
			val = ne.Value
		}

		cause = val

		if foundNode, ok := assoc[val]; ok {
			// If we just found a direct node, we can reset the expression buffer so
			// we don't unnecessarily print element and field accesses for we can see
			// directly in the file.
			if literal {
				expr.Reset()
			}

			node = foundNode
			literal = true
		} else {
			literal = false
		}
	})
	if !isValueError {
		return err
	}

	if node != nil {
		var nodeText strings.Builder
		if err := printer.Fprint(&nodeText, node); err != nil {
			// This should never panic; printer.Fprint only fails when given an
			// unexpected type, which we never do here.
			panic(err)
		}

		// Merge the node text with the expression together (which will be relative
		// accesses to the expression).
		message = fmt.Sprintf("%s%s %s", nodeText.String(), expr.String(), message)
	} else {
		message = fmt.Sprintf("%s %s", expr.String(), message)
	}

	// Render the underlying problematic value as a string.
	var valueText string
	if cause != value.Null {
		be := builder.NewExpr()
		be.SetValue(cause.Interface())
		valueText = string(be.Bytes())
	}
	if literal {
		// Hide the value if the node itself has the error we were worried about.
		valueText = ""
	}

	d := diag.Diagnostic{
		Severity: diag.SeverityLevelError,
		Message:  message,
		Value:    valueText,
	}
	if node != nil {
		d.StartPos = ast.StartPos(node).Position()
		d.EndPos = ast.EndPos(node).Position()
	}
	return d
}

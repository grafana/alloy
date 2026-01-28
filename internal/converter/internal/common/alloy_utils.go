package common

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/printer"
	"github.com/grafana/alloy/syntax/scanner"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/syntax/token/builder"
)

// NewBlockWithOverride generates a new [*builder.Block] using a hook to
// override specific types.
func NewBlockWithOverride(name []string, label string, args component.Arguments) *builder.Block {
	return NewBlockWithOverrideFn(name, label, args, GetAlloyTypesOverrideHook())
}

// NewBlockWithOverrideFn generates a new [*builder.Block] using a hook fn to
// override specific types.
func NewBlockWithOverrideFn(name []string, label string, args component.Arguments, fn builder.ValueOverrideHook) *builder.Block {
	block := builder.NewBlock(name, label)
	block.Body().SetValueOverrideHook(fn)
	block.Body().AppendFrom(args)
	return block
}

// GetAlloyTypesOverrideHook returns a hook for overriding the go value of
// specific go types for converting configs from one type to another.
func GetAlloyTypesOverrideHook() builder.ValueOverrideHook {
	return func(val any) any {
		switch value := val.(type) {
		case alloytypes.Secret:
			return string(value)
		case []alloytypes.Secret:
			secrets := make([]string, 0, len(value))
			for _, secret := range value {
				secrets = append(secrets, string(secret))
			}
			return secrets
		case map[string][]alloytypes.Secret:
			secrets := make(map[string][]string, len(value))
			for k, v := range value {
				secrets[k] = make([]string, 0, len(v))
				for _, secret := range v {
					secrets[k] = append(secrets[k], string(secret))
				}
			}
			return secrets
		case alloy_relabel.Regexp:
			return value.String()
		case []discovery.Target:
			return ConvertTargets{
				Targets: value,
			}
		default:
			return val
		}
	}
}

// LabelForParts generates a consistent component label for a set of parts
// delimited with an underscore.
func LabelForParts(parts ...any) string {
	var sParts []string
	for _, part := range parts {
		if part != "" {
			sParts = append(sParts, fmt.Sprintf("%v", part))
		}
	}
	return strings.Join(sParts, "_")
}

// LabelWithIndex generates a consistent component label for a set of parts
// delimited with an underscore and suffixed with the provided index. If the
// index is 0, the label is generated without the index.
func LabelWithIndex(index int, parts ...any) string {
	if index == 0 {
		return LabelForParts(parts...)
	}

	appendedIndex := index + 1
	return LabelForParts(append(parts, appendedIndex)...)
}

// PrettyPrint parses Alloy config and returns it in a standardize format.
// If PrettyPrint fails, the input is returned unmodified.
func PrettyPrint(in []byte) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Return early if there was no file.
	if len(in) == 0 {
		return in, diags
	}

	f, err := parser.ParseFile("", in)
	if err != nil {
		diags.Add(diag.SeverityLevelError, err.Error())
		return in, diags
	}

	var buf bytes.Buffer
	if err = printer.Fprint(&buf, f); err != nil {
		diags.Add(diag.SeverityLevelError, err.Error())
		return in, diags
	}

	// Add a trailing newline at the end of the file, which is omitted by Fprint.
	_, _ = buf.WriteString("\n")
	return buf.Bytes(), nil
}

func SanitizeIdentifierPanics(in string) string {
	out, err := scanner.SanitizeIdentifier(in)
	if err != nil {
		panic(err)
	}
	return out
}

// DefaultValue returns the default value for a given type. If *T implements
// syntax.Defaulter, a value will be returned with defaults applied. If *T does
// not implement syntax.Defaulter, the zero value of T is returned.
//
// T must not be a pointer type.
func DefaultValue[T any]() T {
	var val T
	if defaulter, ok := any(&val).(syntax.Defaulter); ok {
		defaulter.SetToDefault()
	}
	return val
}

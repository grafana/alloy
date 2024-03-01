// Package stdlib contains standard library functions exposed to River configs.
package stdlib

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/rivertypes"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// Identifiers holds a list of stdlib identifiers by name. All interface{}
// values are River-compatible values.
//
// Function identifiers are Go functions with exactly one non-error return
// value, with an optionally supported error return value as the second return
// value.
var Identifiers = map[string]interface{}{
	// See constants.go for the definition.
	"constants": constants,

	"env": os.Getenv,

	"nonsensitive": func(secret rivertypes.Secret) string {
		return string(secret)
	},

	// concat is implemented as a raw function so it can bypass allocations
	// converting arguments into []interface{}. concat is optimized to allow it
	// to perform well when it is in the hot path for combining targets from many
	// other blocks.
	"concat": value.RawFunction(func(funcValue value.Value, args ...value.Value) (value.Value, error) {
		if len(args) == 0 {
			return value.Array(), nil
		}

		// finalSize is the final size of the resulting concatenated array. We type
		// check our arguments while computing what finalSize will be.
		var finalSize int
		for i, arg := range args {
			if arg.Type() != value.TypeArray {
				return value.Null, value.ArgError{
					Function: funcValue,
					Argument: arg,
					Index:    i,
					Inner: value.TypeError{
						Value:    arg,
						Expected: value.TypeArray,
					},
				}
			}

			finalSize += arg.Len()
		}

		// Optimization: if there's only one array, we can just return it directly.
		// This is done *after* the previous loop to ensure that args[0] is a River
		// array.
		if len(args) == 1 {
			return args[0], nil
		}

		raw := make([]value.Value, 0, finalSize)
		for _, arg := range args {
			for i := 0; i < arg.Len(); i++ {
				raw = append(raw, arg.Index(i))
			}
		}

		return value.Array(raw...), nil
	}),

	"json_decode": func(in string) (interface{}, error) {
		var res interface{}
		err := json.Unmarshal([]byte(in), &res)
		if err != nil {
			return nil, err
		}
		return res, nil
	},

	"json_path": func(jsonString string, path string) (interface{}, error) {
		jsonPathExpr, err := jp.ParseString(path)
		if err != nil {
			return nil, err
		}

		jsonExpr, err := oj.ParseString(jsonString)
		if err != nil {
			return nil, err
		}

		return jsonPathExpr.Get(jsonExpr), nil
	},

	"coalesce": value.RawFunction(func(funcValue value.Value, args ...value.Value) (value.Value, error) {
		if len(args) == 0 {
			return value.Null, nil
		}

		for _, arg := range args {
			if arg.Type() == value.TypeNull {
				continue
			}

			if !arg.Reflect().IsZero() {
				if argType := value.RiverType(arg.Reflect().Type()); (argType == value.TypeArray || argType == value.TypeObject) && arg.Len() == 0 {
					continue
				}

				return arg, nil
			}
		}

		return args[len(args)-1], nil
	}),

	"format":      fmt.Sprintf,
	"join":        strings.Join,
	"replace":     strings.ReplaceAll,
	"split":       strings.Split,
	"to_lower":    strings.ToLower,
	"to_upper":    strings.ToUpper,
	"trim":        strings.Trim,
	"trim_prefix": strings.TrimPrefix,
	"trim_suffix": strings.TrimSuffix,
	"trim_space":  strings.TrimSpace,
}

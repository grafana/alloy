// Package stdlib contains standard library functions exposed to Alloy configs.
package stdlib

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
	"gopkg.in/yaml.v3"
)

// There identifiers are deprecated in favour of the namespaced ones.
var DeprecatedIdentifiers = map[string]interface{}{
	"env":           os.Getenv,
	"nonsensitive":  nonSensitive,
	"concat":        concat,
	"json_decode":   jsonDecode,
	"yaml_decode":   yamlDecode,
	"base64_decode": base64Decode,
	"format":        fmt.Sprintf,
	"join":          strings.Join,
	"replace":       strings.ReplaceAll,
	"split":         strings.Split,
	"to_lower":      strings.ToLower,
	"to_upper":      strings.ToUpper,
	"trim":          strings.Trim,
	"trim_prefix":   strings.TrimPrefix,
	"trim_suffix":   strings.TrimSuffix,
	"trim_space":    strings.TrimSpace,
}

// Identifiers holds a list of stdlib identifiers by name. All interface{}
// values are Alloy-compatible values.
//
// Function identifiers are Go functions with exactly one non-error return
// value, with an optionally supported error return value as the second return
// value.
var Identifiers = map[string]interface{}{
	"constants": constants,
	"coalesce":  coalesce,
	"json_path": jsonPath,

	// New stdlib functions
	"sys":      sys,
	"convert":  convert,
	"array":    array,
	"encoding": encoding,
	"string":   str,
}

func init() {
	// Adds the deprecatedIdentifiers to the map of valid identifiers.
	maps.Copy(Identifiers, DeprecatedIdentifiers)
}

var encoding = map[string]interface{}{
	"from_json":   jsonDecode,
	"from_yaml":   yamlDecode,
	"from_base64": base64Decode,
}

var str = map[string]interface{}{
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

var array = map[string]interface{}{
	"concat": concat,
}

var convert = map[string]interface{}{
	"nonsensitive": nonSensitive,
}

var sys = map[string]interface{}{
	"env": os.Getenv,
}

func nonSensitive(secret alloytypes.Secret) string {
	return string(secret)
}

// concat is implemented as a raw function so it can bypass allocations
// converting arguments into []interface{}. concat is optimized to allow it
// to perform well when it is in the hot path for combining targets from many
// other blocks.
var concat = value.RawFunction(func(funcValue value.Value, args ...value.Value) (value.Value, error) {
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
	// This is done *after* the previous loop to ensure that args[0] is an
	// Alloy array.
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
})

func jsonDecode(in string) (interface{}, error) {
	var res interface{}
	err := json.Unmarshal([]byte(in), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func yamlDecode(in string) (interface{}, error) {
	var res interface{}
	err := yaml.Unmarshal([]byte(in), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func base64Decode(in string) (interface{}, error) {
	decoded, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func jsonPath(jsonString string, path string) (interface{}, error) {
	jsonPathExpr, err := jp.ParseString(path)
	if err != nil {
		return nil, err
	}

	jsonExpr, err := oj.ParseString(jsonString)
	if err != nil {
		return nil, err
	}

	return jsonPathExpr.Get(jsonExpr), nil
}

var coalesce = value.RawFunction(func(funcValue value.Value, args ...value.Value) (value.Value, error) {
	if len(args) == 0 {
		return value.Null, nil
	}

	for _, arg := range args {
		if arg.Type() == value.TypeNull {
			continue
		}

		if !arg.Reflect().IsZero() {
			if argType := value.AlloyType(arg.Reflect().Type()); (argType == value.TypeArray || argType == value.TypeObject) && arg.Len() == 0 {
				continue
			}

			return arg, nil
		}
	}

	return args[len(args)-1], nil
})

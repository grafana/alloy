// Package stdlib contains standard library functions exposed to Alloy configs.
package stdlib

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
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
	"map":      mapFuncs,
	"encoding": encoding,
	"string":   str,
	"file":     file,
}

func init() {
	// Adds the deprecatedIdentifiers to the map of valid identifiers.
	maps.Copy(Identifiers, DeprecatedIdentifiers)
}

var file = map[string]interface{}{
	"path_join": filepath.Join,
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

var mapFuncs = map[string]interface{}{
	"inner_join": innerJoin,
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

func shouldJoinOld(left map[string]value.Value, right map[string]value.Value, conditions []string) bool {
	for _, c := range conditions {
		if !left[c].Equal(right[c]) {
			return false
		}
	}
	return true
}

func concatMapsOld(left map[string]value.Value, right map[string]value.Value) map[string]value.Value {
	res := make(map[string]value.Value)
	for k, v := range left {
		res[k] = v
	}
	for k, v := range right {
		res[k] = v
	}
	return res
}

// We can assume that conditions is an value.TypeArray,
// because it is checked in the innerJoin function.
func shouldJoin(left, right value.Value, conditions value.Value) bool {
	for i := 0; i < conditions.Len(); i++ {
		c := conditions.Index(i)
		// if c.Type() != value.TypeString {
		// 	//TODO: Throw error
		// 	panic("")
		// }
		condition := c.Text()

		// if left.Type() != value.TypeArray || right.Type() != value.TypeArray {
		// 	//TODO: Throw error
		// 	panic("")
		// }

		leftVal, ok := left.Key(condition)
		if !ok {
			//TODO: Throw error
			panic("")
		}

		rightVal, ok := right.Key(condition)
		if !ok {
			//TODO: Throw error
			panic("")
		}

		if !leftVal.Equal(rightVal) {
			return false
		}
	}
	return true
}

// TODO: Make strategy an enum
func concatMaps(left, right value.Value, strategy string, allowlist, denylist map[string]struct{}) value.Value {
	res := make(map[string]value.Value)

	for _, key := range left.Keys() {

		// Insert the value into the result map
		val, ok := left.Key(key)
		if !ok {
			//TODO: Throw error
			panic("")
		}
		res[key] = val
	}

	for _, key := range right.Keys() {
		if len(denylist) > 0 {
			// Make sure the key is not in the denylist
			_, found := denylist[key]
			if found {
				continue
			}
		}

		if len(allowlist) > 0 {
			// Make sure we only insert keys listed in the allowlist
			_, found := allowlist[key]
			if !found {
				continue
			}
		}

		val, ok := right.Key(key)
		if !ok {
			//TODO: Throw error
			panic("")
		}
		res[key] = val
	}

	return value.Object(res)
}

// TODO: What if conditions are empty?
// Inputs:
// 1. []map[string]string
// 2. []map[string]string
// 3. []string
// 4. string: update strategy
// 5. []string: key allowlist
// 6. []string: key denylist
var innerJoin = value.RawFunction(func(funcValue value.Value, args ...value.Value) (value.Value, error) {
	// left := make([]map[string]value.Value, 0)
	// right := make([]map[string]value.Value, 0)
	// conditions := make([]string, 0)

	//TODO: Should the last 3 params be optional?
	if len(args) < 3 {
		return value.Value{}, fmt.Errorf("inner_join: expected at lest 3 arguments, got %d", len(args))
	}

	for i := range []int{0, 1} {
		if args[i].Type() != value.TypeArray {
			return value.Null, value.ArgError{
				Function: funcValue,
				Argument: args[i],
				Index:    i,
				Inner: value.TypeError{
					Value:    args[i],
					Expected: value.TypeArray,
				},
			}
		}
		for j := 0; j < args[i].Len(); j++ {
			if args[i].Index(j).Type() != value.TypeObject {
				return value.Null, value.ArgError{
					Function: funcValue,
					Argument: args[i].Index(j),
					Index:    j,
					Inner: value.TypeError{
						Value:    args[i].Index(j),
						Expected: value.TypeObject,
					},
				}
			}
		}
	}

	//TODO: Preallocate memory?
	res := make([]value.Value, 0, 0)

	conditions := args[2]

	var strategy string
	if len(args) >= 4 {
		strategy = args[3].Text()
	}

	allowlist := make(map[string]struct{}, args[4].Len())
	for i := 0; i < args[4].Len(); i++ {
		allowlist[args[4].Index(i).Text()] = struct{}{}
	}

	denylist := make(map[string]struct{}, args[5].Len())
	for i := 0; i < args[5].Len(); i++ {
		denylist[args[5].Index(i).Text()] = struct{}{}
	}

	for i := 0; i < args[0].Len(); i++ {
		for j := 0; j < args[1].Len(); j++ {
			left := args[0].Index(i)
			right := args[1].Index(j)
			if shouldJoin(left, right, conditions) {
				res = append(res, concatMaps(left, right, strategy, allowlist, denylist))
			}
		}
	}

	return value.Array(res...), nil
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

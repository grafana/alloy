// Package stdlib contains standard library functions exposed to Alloy configs.
package stdlib

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/internal/value"
)

// ExperimentalIdentifiers contains the full name (namespace + identifier's name) of stdlib
// identifiers that are considered "experimental".
var ExperimentalIdentifiers = map[string]bool{
	"array.combine_maps": true,
	"array.group_by":     true,
}

// DeprecatedIdentifiers are deprecated in favour of the namespaced ones.
var DeprecatedIdentifiers = map[string]any{
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
var Identifiers = map[string]any{
	"constants": constants,
	"coalesce":  coalesce,
	"json_path": jsonPath,

	// New stdlib functions
	"sys":      sys,
	"convert":  convert,
	"array":    array,
	"encoding": encoding,
	"string":   str,
	"file":     file,
}

func init() {
	// Adds the deprecatedIdentifiers to the map of valid identifiers.
	maps.Copy(Identifiers, DeprecatedIdentifiers)
}

var file = map[string]any{
	"path_join": filepath.Join,
}

var encoding = map[string]any{
	"from_json":      jsonDecode,
	"from_yaml":      yamlDecode,
	"from_base64":    base64Decode,
	"from_URLbase64": base64URLDecode,
	"to_json":        jsonEncode,
	"to_base64":      base64Encode,
	"to_URLbase64":   base64URLEncode,
	"url_encode":     urlEncode,
	"url_decode":     urlDecode,
}

var str = map[string]any{
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

// groupBy takes an array of objects, a key to group by, and a boolean to determine
// whether to drop objects missing the key. It returns an array of objects containing
// the key value and grouped items.
var groupBy = value.RawFunction(func(funcValue value.Value, args ...value.Value) (value.Value, error) {
	if len(args) != 3 {
		return value.Null, fmt.Errorf("group_by: expected 3 arguments, got %d", len(args))
	}

	if args[0].Type() != value.TypeArray {
		return value.Null, value.ArgError{
			Function: funcValue,
			Argument: args[0],
			Index:    0,
			Inner: value.TypeError{
				Value:    args[0],
				Expected: value.TypeArray,
			},
		}
	}

	if args[1].Type() != value.TypeString {
		return value.Null, value.ArgError{
			Function: funcValue,
			Argument: args[1],
			Index:    1,
			Inner: value.TypeError{
				Value:    args[1],
				Expected: value.TypeString,
			},
		}
	}

	if args[2].Type() != value.TypeBool {
		return value.Null, value.ArgError{
			Function: funcValue,
			Argument: args[2],
			Index:    2,
			Inner: value.TypeError{
				Value:    args[2],
				Expected: value.TypeBool,
			},
		}
	}

	key := args[1].Text()
	dropMissing := args[2].Bool()

	groups := make(map[string][]value.Value)
	for i := 0; i < args[0].Len(); i++ {
		item := args[0].Index(i)
		if item.Type() != value.TypeObject {
			obj, ok := item.TryConvertToObject()
			if !ok {
				return value.Null, value.ArgError{
					Function: funcValue,
					Argument: item,
					Index:    i,
					Inner: value.TypeError{
						Value:    item,
						Expected: value.TypeObject,
					},
				}
			}
			item = value.Object(obj)
		}

		val, hasKey := item.Key(key)
		if !hasKey {
			if dropMissing {
				continue
			}
			// Add to empty value group if not dropping
			groups[""] = append(groups[""], item)
			continue
		}

		// Only accept string values for the key field
		if val.Type() != value.TypeString {
			return value.Null, value.ArgError{
				Function: funcValue,
				Argument: val,
				Index:    i,
				Inner: value.TypeError{
					Value:    val,
					Expected: value.TypeString,
				},
			}
		}

		valStr := val.Text()
		groups[valStr] = append(groups[valStr], item)
	}

	result := make([]value.Value, 0, len(groups))
	for val, items := range groups {
		groupObj := map[string]value.Value{
			key:     value.String(val),
			"items": value.Array(items...),
		}
		result = append(result, value.Object(groupObj))
	}

	return value.Array(result...), nil
})

var array = map[string]any{
	"concat":       concat,
	"combine_maps": combineMaps,
	"group_by":     groupBy,
}

var convert = map[string]any{
	"nonsensitive": nonSensitive,
}

var sys = map[string]any{
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

// This function assumes that the types of the value.Value objects are correct.
func shouldJoin(left value.Value, right value.Value, conditions value.Value) bool {
	for i := 0; i < conditions.Len(); i++ {
		condition := conditions.Index(i).Text()

		leftVal, ok := left.Key(condition)
		if !ok {
			return false
		}

		rightVal, ok := right.Key(condition)
		if !ok {
			return false
		}

		if !leftVal.Equal(rightVal) {
			return false
		}
	}
	return true
}

// Merge two maps.
// If a key exists in both maps, the value from the right map will be used.
func concatMaps(left, right value.Value) (value.Value, error) {
	res := make(map[string]value.Value)

	for _, key := range left.Keys() {
		val, ok := left.Key(key)
		if !ok {
			return value.Null, fmt.Errorf("concatMaps: key %s not found in left map while iterating - this should never happen", key)
		}
		res[key] = val
	}

	for _, key := range right.Keys() {
		val, ok := right.Key(key)
		if !ok {
			return value.Null, fmt.Errorf("concatMaps: key %s not found in right map while iterating - this should never happen", key)
		}
		res[key] = val
	}

	return value.Object(res), nil
}

// Inputs:
// args[0]: []map[string]string: lhs array
// args[1]: []map[string]string: rhs array
// args[2]: []string:            merge conditions
// args[3]: bool:                (optional) retain unmatched elements from the lhs array
var combineMaps = value.RawFunction(func(funcValue value.Value, args ...value.Value) (value.Value, error) {
	if len(args) != 3 && len(args) != 4 {
		return value.Value{}, fmt.Errorf("combine_maps: expected 3 or 4 arguments, got %d", len(args))
	}

	// Validate args[0] and args[1]
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
			elem := args[i].Index(j)
			// Check if elements are objects or are convertible to objects.
			if elem.Type() != value.TypeObject {
				if _, ok := elem.TryConvertToObject(); !ok {
					return value.Null, value.ArgError{
						Function: funcValue,
						Argument: elem,
						Index:    j,
						Inner: value.TypeError{
							Value:    elem,
							Expected: value.TypeObject,
						},
					}
				}
			}
		}
	}

	// Validate args[2]
	if args[2].Type() != value.TypeArray {
		return value.Null, value.ArgError{
			Function: funcValue,
			Argument: args[2],
			Index:    2,
			Inner: value.TypeError{
				Value:    args[2],
				Expected: value.TypeArray,
			},
		}
	}
	if args[2].Len() == 0 {
		return value.Null, value.ArgError{
			Function: funcValue,
			Argument: args[2],
			Index:    2,
			Inner:    fmt.Errorf("combine_maps: merge conditions must not be empty"),
		}
	}

	// Validate args[3]
	passthroughLHS := false
	if len(args) == 4 {
		if args[3].Type() != value.TypeBool {
			return value.Null, value.ArgError{
				Function: funcValue,
				Argument: args[3],
				Index:    3,
				Inner: value.TypeError{
					Value:    args[3],
					Expected: value.TypeBool,
				},
			}
		}
		passthroughLHS = args[3].Bool()
	}

	convertIfNeeded := func(v value.Value) value.Value {
		if v.Type() != value.TypeObject {
			obj, _ := v.TryConvertToObject() // no need to check result as arguments were validated earlier.
			return value.Object(obj)
		}
		return v
	}

	// We cannot preallocate the size of the result array, because we don't know
	// how well the merge is going to go. If none of the merge conditions are met,
	// the result array will be empty.
	res := []value.Value{}
	// However, if passthroughLHS is set to true, then we know the size of the result array.
	if passthroughLHS {
		res = make([]value.Value, 0, args[0].Len())
	}

	for i := 0; i < args[0].Len(); i++ {
		for j := 0; j < args[1].Len(); j++ {
			left := convertIfNeeded(args[0].Index(i))
			right := convertIfNeeded(args[1].Index(j))

			if shouldJoin(left, right, args[2]) {
				val, err := concatMaps(left, right)
				if err != nil {
					return value.Null, err
				}
				res = append(res, val)
			} else if passthroughLHS {
				res = append(res, args[0].Index(i))
			}
		}
	}

	return value.Array(res...), nil
})

func jsonDecode(in string) (any, error) {
	var res any
	err := json.Unmarshal([]byte(in), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func yamlDecode(in string) (any, error) {
	var res any
	err := yaml.Unmarshal([]byte(in), &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func base64Decode(in string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func base64URLDecode(in string) ([]byte, error) {
	decoded, err := base64.URLEncoding.DecodeString(in)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func base64URLEncode(in string) (string, error) {
	encoded := base64.URLEncoding.EncodeToString([]byte(in))
	return encoded, nil
}

func base64Encode(in string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(in))
	return encoded, nil
}

func urlEncode(in string) (string, error) {
	return url.QueryEscape(in), nil
}

func urlDecode(in string) (string, error) {
	return url.QueryUnescape(in)
}

func jsonEncode(in any) (string, error) {
	v, ok := in.(map[string]any)
	if !ok {
		return "", fmt.Errorf("jsonEncode only supports map")
	}
	res, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func jsonPath(jsonString string, path string) ([]any, error) {
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
			argType := arg.Type()
			// Check if it's a capsule that can be converted into an object and the object is empty.
			if obj, ok := arg.TryConvertToObject(); ok && len(obj) == 0 {
				continue
			}
			// Check if it's an array or an object that's empty.
			if (argType == value.TypeArray || argType == value.TypeObject) && arg.Len() == 0 {
				continue
			}

			// Else we found a non-empty argument.
			return arg, nil
		}
	}

	// Return the last arg if all are empty.
	return args[len(args)-1], nil
})

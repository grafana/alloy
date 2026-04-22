package utils

import "github.com/grafana/alloy/syntax/encoding/alloyjson"

// MarshalBodyToString marshals the provided value to a JSON string, safely accounting for nil values.
func MarshalBodyToString(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := alloyjson.MarshalBody(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

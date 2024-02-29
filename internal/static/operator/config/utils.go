package config

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/fatih/structs"
	jsonnet "github.com/google/go-jsonnet"
	gragent "github.com/grafana/agent/internal/static/operator/apis/monitoring/v1alpha1"
	"sigs.k8s.io/yaml"
)

func unmarshalYAML(i []interface{}) (interface{}, error) {
	text, ok := i[0].(string)
	if !ok {
		return nil, jsonnet.RuntimeError{Msg: "unmarshalYAML text argument must be a string"}
	}
	var v interface{}
	err := yaml.Unmarshal([]byte(text), &v)
	if err != nil {
		return nil, jsonnet.RuntimeError{Msg: err.Error()}
	}
	return v, nil
}

// trimMap recursively deletes fields from m whose value is nil.
func trimMap(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}

		if next, ok := v.(map[string]interface{}); ok {
			trimMap(next)
		}

		if arr, ok := v.([]interface{}); ok {
			m[k] = trimSlice(arr)
		}
	}
}

func trimSlice(s []interface{}) []interface{} {
	res := make([]interface{}, 0, len(s))

	for _, e := range s {
		if e == nil {
			continue
		}

		if next, ok := e.([]interface{}); ok {
			e = trimSlice(next)
		}

		if next, ok := e.(map[string]interface{}); ok {
			trimMap(next)
		}

		res = append(res, e)
	}

	return res
}

// intoStages converts a yaml slice of stages into a Jsonnet array.
func intoStages(i []interface{}) (interface{}, error) {
	text, ok := i[0].(string)
	if !ok {
		return nil, jsonnet.RuntimeError{Msg: "text argument not string"}
	}

	// The way this works is really, really gross. We only need any of this
	// because Kubernetes CRDs can't recursively define types, which we need
	// for the match stage.
	//
	// 1. Convert YAML -> map[string]interface{}
	// 2. Convert map[string]interface{} -> JSON
	// 3. Convert JSON -> []*grafana.PipelineStageSpec
	// 4. Convert []*grafana.PipelineStageSpec into []interface{}, where
	//    each interface{} has the type information lost so marshaling it
	//    again to JSON doesn't break anything.
	var raw interface{}
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return nil, jsonnet.RuntimeError{
			Msg: fmt.Sprintf("failed to unmarshal stages: %s", err.Error()),
		}
	}

	bb, err := json.Marshal(raw)
	if err != nil {
		return nil, jsonnet.RuntimeError{
			Msg: fmt.Sprintf("failed to unmarshal stages: %s", err.Error()),
		}
	}

	var ps []*gragent.PipelineStageSpec
	if err := json.Unmarshal(bb, &ps); err != nil {
		return nil, jsonnet.RuntimeError{
			Msg: fmt.Sprintf("failed to unmarshal stages: %s", err.Error()),
		}
	}

	// Then we need to convert each into their raw types.
	rawPS := make([]interface{}, 0, len(ps))
	for _, stage := range ps {
		bb, err := json.Marshal(structs.Map(stage))
		if err != nil {
			return nil, jsonnet.RuntimeError{
				Msg: fmt.Sprintf("failed to unmarshal stages: %s", err.Error()),
			}
		}

		var v interface{}
		if err := json.Unmarshal(bb, &v); err != nil {
			return nil, jsonnet.RuntimeError{
				Msg: fmt.Sprintf("failed to unmarshal stages: %s", err.Error()),
			}
		}

		rawPS = append(rawPS, v)
	}
	return rawPS, nil
}

var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// SanitizeLabelName sanitizes a label name for Prometheus.
func SanitizeLabelName(name string) string {
	return invalidLabelCharRE.ReplaceAllString(name, "_")
}

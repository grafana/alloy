package common

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/syntax/token/builder"
)

const (
	Equals = iota
	NotEquals
	DeepEquals
	NotDeepEquals
	NotYAMLMarshallEquals
)

// ValidateSupported will return a diagnostic error if the validationType
// specified results in a match for value1 and value2.
//
// For example, if using validationType Equals and value1 is equal to value2,
// then a diagnostic error will be returned.
func ValidateSupported(validationType int, value1 any, value2 any, name string, message string) diag.Diagnostics {
	var diags diag.Diagnostics
	var isInvalid bool

	switch validationType {
	case Equals:
		isInvalid = value1 == value2
	case NotEquals:
		isInvalid = value1 != value2
	case DeepEquals:
		isInvalid = reflect.DeepEqual(value1, value2)
	case NotDeepEquals:
		isInvalid = !reflect.DeepEqual(value1, value2)
	case NotYAMLMarshallEquals:
		v1yaml, err := yaml.Marshal(value1)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("Error marshalling value1: %v", err))
		}
		v2yaml, err := yaml.Marshal(value2)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("Error marshalling value2: %v", err))
		}
		isInvalid = string(v1yaml) != string(v2yaml)
	default:
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("Invalid converter validation type was requested: %d.", validationType))
	}

	if isInvalid {
		if message != "" {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s config: %s", name, message))
		} else {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s config.", name))
		}
	}

	return diags
}

// ValidateNodes will look at the final nodes and ensure that there are no
// duplicate labels.
func ValidateNodes(f *builder.File) diag.Diagnostics {
	var diags diag.Diagnostics

	nodes := f.Body().Nodes()
	labels := make(map[string]string, len(nodes))
	for _, node := range nodes {
		switch n := node.(type) {
		case *builder.Block:
			label := strings.Join(n.Name, ".")
			if n.Label != "" {
				label += "." + n.Label
			}
			if _, ok := labels[label]; ok {
				diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("duplicate label after conversion %q. this is due to how valid Alloy labels are assembled and can be avoided by updating named properties in the source config.", label))
			} else {
				labels[label] = label
			}
		}
	}

	return diags
}

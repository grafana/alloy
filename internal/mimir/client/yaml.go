package client

import (
	"bytes"

	"go.yaml.in/yaml/v3"
)

// marshalYAML encodes v as YAML with a 2-space indent. With the default 4-space
// indent, a block scalar in a sequence item that needs an indentation indicator,
// such as a rule expression starting with a blank line or with leading spaces, is
// written with an indicator that does not match the indentation of its content,
// and the result can't be parsed.
func marshalYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

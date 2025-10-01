package jsonschema

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

type Metadata struct {
	Exports   *Schema
	Arguments *Schema
}

func ParseMetadata(data []byte) (*Metadata, error) {
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	exportsData, err := json.Marshal(m["exports"])
	if err != nil {
		return nil, err
	}
	var exportSchema Schema
	if err := json.Unmarshal(exportsData, &exportSchema); err != nil {
		return nil, err
	}

	argumentsData, err := json.Marshal(m["arguments"])
	if err != nil {
		return nil, err
	}
	var argumentsSchema Schema
	if err := json.Unmarshal(argumentsData, &argumentsSchema); err != nil {
		return nil, err
	}

	return &Metadata{Exports: &exportSchema, Arguments: &argumentsSchema}, nil
}

package metadata

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Metadata struct {
	Arguments *Schema `yaml:"arguments,omitempty"`
	Exports   *Schema `yaml:"exports,omitempty"`
}

// SchemaProperty represents a property in the YAML schema
type Schema struct {
	Ref                  string             `yaml:"$ref,omitempty"`
	Description          string             `yaml:"description,omitempty"`
	Type                 string             `yaml:"type,omitempty"`
	AllOf                []*Schema          `yaml:"allOf,omitempty"` // Squashed args and blocks
	Items                *Schema            `yaml:"items,omitempty"`
	Required             []string           `yaml:"required,omitempty"`
	Alloy                AlloyOverrides     `yaml:"alloy,omitempty"`
	Default              any                `yaml:"default,omitempty"`
	Properties           map[string]*Schema `yaml:"properties,omitempty"`
	Definitions          map[string]*Schema `yaml:"$defs,omitempty"`
	AdditionalProperties *Schema            `yaml:"additionalProperties,omitempty"`
}

// AlloyOverrides represents the alloy-specific configuration
type AlloyOverrides struct {
	Type            string `yaml:"type"`
	TypeOverride    string `yaml:"type_override,omitempty"`
	DefaultOverride string `yaml:"default_override,omitempty"`
}

func FromPath(ymlPath string) (*Metadata, error) {
	data, err := os.ReadFile(ymlPath)
	if err != nil {
		return nil, err
	}

	var schema Metadata
	err = yaml.Unmarshal(data, &schema)
	if err != nil {
		return nil, err
	}

	return &schema, nil
}

// TODO: Use generics to reuse code with the previous function
func FromPath2(ymlPath string) (*Schema, error) {
	data, err := os.ReadFile(ymlPath)
	if err != nil {
		return nil, err
	}

	var schema Schema
	err = yaml.Unmarshal(data, &schema)
	if err != nil {
		return nil, err
	}

	return &schema, nil
}

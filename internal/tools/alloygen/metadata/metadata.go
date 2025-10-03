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
	Description          string             `yaml:"description"`
	Type                 string             `yaml:"type"`
	Items                *Schema            `yaml:"items,omitempty"`
	Required             []string           `yaml:"required,omitempty"`
	Alloy                AlloyConfig        `yaml:"alloy,omitempty"`
	Default              any                `yaml:"default,omitempty"`
	Properties           map[string]*Schema `yaml:"properties,omitempty"`
	AdditionalProperties *Schema            `yaml:"additionalProperties"`
}

// AlloyConfig represents the alloy-specific configuration
type AlloyConfig struct {
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

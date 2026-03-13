// Package jsonschema provides loading and merging of component metadata YAML schemas.
//
// TODO: In the future this package may be extracted out of docs_args_generator
// if other packages need to read or merge the same schema format.
package jsonschema

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// unmarshalFromPath reads a YAML file at ymlPath and unmarshals it into *T.
func unmarshalFromPath[T any](ymlPath string) (*T, error) {
	data, err := os.ReadFile(ymlPath)
	if err != nil {
		return nil, err
	}
	var v T
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// LoadMetadata reads a schema file at ymlPath, merges all subschemas (allOf $refs), and returns the metadata.
// If the file does not exist, it returns (nil, nil). On parse or merge errors it returns an error.
func LoadMetadata(ymlPath string) (*Metadata, error) {
	if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	meta, err := unmarshalFromPath[Metadata](ymlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	ymlPathDir := filepath.Dir(ymlPath)
	if err := mergeSubschemas(ymlPathDir, meta.Arguments); err != nil {
		return nil, fmt.Errorf("failed to merge argument subschemas: %w", err)
	}
	if err := mergeSubschemas(ymlPathDir, meta.Exports); err != nil {
		return nil, fmt.Errorf("failed to merge export subschemas: %w", err)
	}

	return meta, nil
}

// mergeSubschemas resolves and merges subschemas referenced via allOf into the given schema.
// ymlPathDir is the directory used to resolve relative $ref paths (e.g. subschema/schema.yml).
// schema is the parent schema that will be updated in place; its allOf entries are loaded
// from the filesystem and their definitions merged into schema.Properties.
func mergeSubschemas(ymlPathDir string, schema *Schema) error {
	if schema == nil {
		return nil
	}

	for _, prop := range schema.AllOf {
		// TODO: Support refs which are not files
		if prop.Ref != "" {
			referencePath := filepath.Join(ymlPathDir, prop.Ref)
			log.Printf("Processing YAML subschema: %s", referencePath)

			parsedProp, err := unmarshalFromPath[Schema](referencePath)
			if err != nil {
				return fmt.Errorf("failed to parse schema file: %w", err)
			}

			if err := mergeSubschemas(ymlPathDir, parsedProp); err != nil {
				return err
			}

			for name, prop := range parsedProp.Definitions {
				schema.Properties[name] = prop
			}
		}
	}

	return nil
}

type Metadata struct {
	Arguments *Schema `yaml:"arguments,omitempty"`
	Exports   *Schema `yaml:"exports,omitempty"`
}

// Schema represents a property in the YAML schema
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

// IsBlock reports whether the schema property represents a block.
func (s *Schema) IsBlock() bool {
	if s.Type != "object" {
		return false
	}

	// The property is an object, but the schema explicitly said it's not a block
	// TODO: Make "block" an enum
	if s.Alloy.Type != "" && s.Alloy.Type != "block" {
		return false
	}

	return true
}

// ToAlloyType returns the Alloy type string for the schema property.
func (s *Schema) ToAlloyType() string {
	if s.Alloy.TypeOverride != "" {
		return s.Alloy.TypeOverride
	}

	switch s.Type {
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	default:
		return s.Type
	}
}

// DetermineDefault returns the default value display for the schema property.
func (s *Schema) DetermineDefault() string {
	if s.Alloy.DefaultOverride != "" {
		return s.Alloy.DefaultOverride
	}

	if s.Type == "string" {
		if str, ok := s.Default.(string); ok {
			return fmt.Sprintf("%q", str)
		}
	}

	if s.Default != nil {
		return fmt.Sprintf("%v", s.Default)
	}
	return "" // Empty default
}

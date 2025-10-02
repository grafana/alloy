package jsonschema

import "encoding/json"

type Schema struct {
	ID     string  `json:"$id,omitempty"`     // Public identifier for the schema.
	Schema string  `json:"$schema,omitempty"` // URI indicating the specification the schema conforms to.
	Format *string `json:"format,omitempty"`  // Format hint for string data, e.g., "email" or "date-time".
	Type   Type    `json:"type,omitempty"`    // Can be a single type or an array of types.

	// Boolean JSON Schemas, see https://json-schema.org/draft/2020-12/json-schema-core#name-boolean-json-schemas
	Boolean *bool `json:"-"` // Boolean schema, used for quick validation.

	Properties           *SchemaMap `json:"properties,omitempty"`           // Definitions of properties for object types.
	AdditionalProperties *Schema    `json:"additionalProperties,omitempty"` // Can be a boolean or a schema, controls additional properties handling.
	PropertyNames        *Schema    `json:"propertyNames,omitempty"`        // Can be a boolean or a schema, controls property names validation.

	Items       *Schema  `json:"items,omitempty"`       // Schema for items in an array.
	Enum        []any    `json:"enum,omitempty"`        // Enumerated values for the property.
	Required    []string `json:"required,omitempty"`    // List of required property names for object types.
	Title       *string  `json:"title,omitempty"`       // A short summary of the schema.
	Description *string  `json:"description,omitempty"` // A detailed description of the purpose of the schema.
	Default     any      `json:"default,omitempty"`     // Default value of the instance.
	Deprecated  *bool    `json:"deprecated,omitempty"`  // Indicates that the schema is deprecated.

	// Alloy is custom fields used in various code generation tasks
	Alloy Alloy `json:"alloy"`

	// FIXME: handle references, see https://json-schema.org/draft/2020-12/json-schema-core#ref

	// FIXME: subschemas with logical keywords, see https://json-schema.org/draft/2020-12/json-schema-core#name-keywords-for-applying-subsch

	// FIXME: subschemas conditionally, see https://json-schema.org/draft/2020-12/json-schema-core#name-keywords-for-applying-subsche

	// FIXME: subschemas to array keywords, see https://json-schema.org/draft/2020-12/json-schema-core#name-keywords-for-applying-subschem

	// FIXME: Numeric validation keywords, see https://json-schema.org/draft/2020-12/json-schema-validation#section-6.2

	// FIXME: String validation keywords, see https://json-schema.org/draft/2020-12/json-schema-validation#section-6.3

	// FIXME: Array validation keywords, see https://json-schema.org/draft/2020-12/json-schema-validation#section-6.4

	// FIXME: Object validation keywords, see https://json-schema.org/draft/2020-12/json-schema-validation#section-6.5

	// FIXME: https://json-schema.org/draft/2020-12/json-schema-core#name-unevaluatedproperties

	// FIXME: Content validation keywords, see https://json-schema.org/draft/2020-12/json-schema-validation#name-a-vocabulary-for-the-conten

	// FIXME: Applying subschemas to objects keywords, see https://json-schema.org/draft/2020-12/json-schema-core#name-keywords-for-applying-subschemas

	// FIXME: Any validation keywords, see https://json-schema.org/draft/2020-12/json-schema-validation#section-6.1

	// FIXME: Meta-data for schema and instance description, see https://json-schema.org/draft/2020-12/json-schema-validation#name-a-vocabulary-for-basic-meta
}

// UnmarshalJSON handles unmarshaling JSON data into the Schema type.
func (s *Schema) UnmarshalJSON(data []byte) error {
	// First try to parse as a boolean
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		s.Boolean = &b
		return nil
	}

	// If not a boolean, parse as a normal struct
	type Alias Schema
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	*s = Schema(alias)
	return nil
}

type Type []string

// UnmarshalJSON customizes the JSON deserialization into SchemaType.
func (r *Type) UnmarshalJSON(data []byte) error {
	var singleType string
	if err := json.Unmarshal(data, &singleType); err == nil {
		*r = Type{singleType}
		return nil
	}

	var multiType []string
	if err := json.Unmarshal(data, &multiType); err != nil {
		return err
	}

	*r = Type(multiType)
	return nil
}

type SchemaMap map[string]*Schema

type Alloy struct {
	Type            string `yaml:"type"`
	TypeOverride    string `json:"type_override"`
	TypeSource      string `json:"type_source"`
	DefaultOverride string `yaml:"default_override,omitempty"`
}

func (s *Schema) IsObject() bool {
	return len(s.Type) == 1 && s.Type[0] == "object"
}

func (s *Schema) IsString() bool {
	return len(s.Type) == 1 && s.Type[0] == "string"
}

func (s *Schema) IsArray() bool {
	return len(s.Type) == 1 && s.Type[0] == "array"
}

func (s *Schema) GoType() string {
	if s.Alloy.TypeOverride != "" {
		return s.Alloy.TypeOverride
	}

	if s.IsString() {
		return "string"
	}

	// FIXME: handle array of arrays and array of objects
	if s.IsArray() {
		inner := s.Items.GoType()
		return "[]" + inner
	}
	return ""
}

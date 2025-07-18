package repl

// IntrospectionData represents the top-level response from a GraphQL introspection query
type IntrospectionData struct {
	Data struct {
		Schema struct {
			QueryType        *TypeRef    `json:"queryType"`
			MutationType     *TypeRef    `json:"mutationType"`
			SubscriptionType *TypeRef    `json:"subscriptionType"`
			Types            []FullType  `json:"types"`
			Directives       []Directive `json:"directives"`
		} `json:"__schema"`
	} `json:"data"`
}

// FullType represents a complete GraphQL type with all its metadata
type FullType struct {
	Kind          string       `json:"kind"`
	Name          *string      `json:"name"`
	Description   *string      `json:"description"`
	Fields        []Field      `json:"fields"`
	InputFields   []InputValue `json:"inputFields"`
	Interfaces    []TypeRef    `json:"interfaces"`
	EnumValues    []EnumValue  `json:"enumValues"`
	PossibleTypes []TypeRef    `json:"possibleTypes"`
}

// Field represents a field in a GraphQL type
type Field struct {
	Name              string       `json:"name"`
	Description       *string      `json:"description"`
	Args              []InputValue `json:"args"`
	Type              TypeRef      `json:"type"`
	IsDeprecated      bool         `json:"isDeprecated"`
	DeprecationReason *string      `json:"deprecationReason"`
}

// InputValue represents an input value (argument or input field)
type InputValue struct {
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Type         TypeRef `json:"type"`
	DefaultValue *string `json:"defaultValue"`
}

// TypeRef represents a reference to a GraphQL type (with support for nested types)
type TypeRef struct {
	Kind   string   `json:"kind"`
	Name   *string  `json:"name"`
	OfType *TypeRef `json:"ofType"`
}

// EnumValue represents a value in a GraphQL enum
type EnumValue struct {
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	IsDeprecated      bool    `json:"isDeprecated"`
	DeprecationReason *string `json:"deprecationReason"`
}

// Directive represents a GraphQL directive
type Directive struct {
	Name        string       `json:"name"`
	Description *string      `json:"description"`
	Locations   []string     `json:"locations"`
	Args        []InputValue `json:"args"`
}

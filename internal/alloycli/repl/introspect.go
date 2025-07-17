package repl

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/service/graphql"
)

// Cache for introspection data
var (
	introspectionCache struct {
		data      *IntrospectionData
		timestamp time.Time
		mutex     sync.Mutex
	}
	cacheExpiration = 5 * time.Minute
)

// Introspect executes the introspection query and returns the parsed response
func Introspect(gqlClient *graphql.GraphQlClient) (*IntrospectionData, error) {
	introspectionCache.mutex.Lock()
	defer introspectionCache.mutex.Unlock()

	// Check if we have a valid cached result
	if introspectionCache.data != nil && time.Since(introspectionCache.timestamp) < cacheExpiration {
		return introspectionCache.data, nil
	}

	response, err := gqlClient.Execute(introspectionQuery)
	if err != nil {
		return nil, err
	}

	var introspectionData IntrospectionData
	err = json.Unmarshal(response.Raw, &introspectionData)
	if err != nil {
		return nil, err
	}

	// Update cache
	introspectionCache.data = &introspectionData
	introspectionCache.timestamp = time.Now()

	return introspectionCache.data, nil
}

// GetQueryFields is a helper function to extract just the query type fields
func (iq *IntrospectionData) GetQueryFields() []Field {
	if iq.Data.Schema.QueryType == nil {
		return []Field{}
	}

	// Find the Query type in the types array
	for _, t := range iq.Data.Schema.Types {
		if t.Name != nil && *t.Name == *iq.Data.Schema.QueryType.Name {
			return t.Fields
		}
	}

	return []Field{}
}

// GetFieldsAtPath returns the available fields at the given path depth
// parentPath represents the nested field path (e.g., ["users", "profile", "address"])
func (iq *IntrospectionData) GetFieldsAtPath(parentPath []string) []Field {
	if len(parentPath) == 0 {
		return iq.GetQueryFields()
	}

	// Create a map for quick type lookup
	typeMap := make(map[string]FullType)
	for _, t := range iq.Data.Schema.Types {
		if t.Name != nil {
			typeMap[*t.Name] = t
		}
	}

	// Start with query fields
	currentFields := iq.GetQueryFields()

	// Navigate through each segment of the path
	for _, segment := range parentPath {
		var targetField *Field
		for _, field := range currentFields {
			if field.Name == segment {
				targetField = &field
				break
			}
		}

		if targetField == nil {
			return []Field{}
		}

		// Resolve the type of this field
		typeName := resolveUnderlyingTypeName(targetField.Type)
		if typeName == "" {
			return []Field{}
		}

		// Look up the type definition
		targetType, exists := typeMap[typeName]
		if !exists {
			return []Field{}
		}

		// Only object types have fields
		if targetType.Kind != "OBJECT" {
			return []Field{}
		}

		currentFields = targetType.Fields
	}

	return currentFields
}

// resolveUnderlyingTypeName gets the base type name, unwrapping NON_NULL and LIST wrappers
func resolveUnderlyingTypeName(typeRef TypeRef) string {
	if typeRef.Kind == "NON_NULL" || typeRef.Kind == "LIST" {
		if typeRef.OfType != nil {
			return resolveUnderlyingTypeName(*typeRef.OfType)
		}
	}
	if typeRef.Name != nil {
		return *typeRef.Name
	}
	return ""
}

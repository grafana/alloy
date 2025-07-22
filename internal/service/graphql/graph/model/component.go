package model

import (
	"fmt"
	"reflect"

	"github.com/grafana/alloy/internal/component"
)

type Component struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Health    Health `json:"health"`
	Arguments string `json:"arguments"`
	Exports   string `json:"exports"`
	DebugInfo string `json:"debugInfo"`
}

// argumentsToMap converts arguments to a map of field names to string values
func asMap(args any) map[string]string {
	if args == nil {
		return nil
	}

	val := reflect.ValueOf(args)
	typ := reflect.TypeOf(args)

	// Handle pointers
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Struct {
		return map[string]string{"": fmt.Sprintf("%v", args)}
	}

	result := make(map[string]string)
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		fieldName := fieldType.Name
		fieldValue := fmt.Sprintf("%v", field.Interface())
		result[fieldName] = fieldValue
	}

	return result
}

func NewComponent(comp *component.Info) Component {
	return Component{
		ID:   comp.ID.String(),
		Name: comp.ComponentName,
		Health: Health{
			Message:     comp.Health.Message,
			LastUpdated: comp.Health.UpdateTime,
		},
		Arguments: fmt.Sprintf("%v", asMap(comp.Arguments)),
		Exports:   fmt.Sprintf("%v", asMap(comp.Exports)),
		DebugInfo: fmt.Sprintf("%v", asMap(comp.DebugInfo)),
	}
}

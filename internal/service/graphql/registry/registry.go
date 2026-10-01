package registry

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	"github.com/grafana/alloy/syntax"
)

func Definitions() []model.ComponentDefinition {
	names := component.AllNames()
	definitions := make([]model.ComponentDefinition, 0, len(names))
	for _, name := range names {
		reg, ok := component.Get(name)
		if !ok {
			continue
		}
		definitions = append(definitions, DefinitionFromRegistration(reg))
	}
	return definitions
}

func DefinitionsFromRegistrations(registrations []component.Registration) []model.ComponentDefinition {
	definitions := make([]model.ComponentDefinition, 0, len(registrations))
	for _, reg := range registrations {
		definitions = append(definitions, DefinitionFromRegistration(reg))
	}
	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})
	return definitions
}

func DefinitionFromRegistrations(registrations []component.Registration, name string) *model.ComponentDefinition {
	for _, reg := range registrations {
		if reg.Name == name {
			definition := DefinitionFromRegistration(reg)
			return &definition
		}
	}
	return nil
}

func Definition(name string) *model.ComponentDefinition {
	reg, ok := component.Get(name)
	if !ok {
		return nil
	}
	definition := DefinitionFromRegistration(reg)
	return &definition
}

func DefinitionFromRegistration(reg component.Registration) model.ComponentDefinition {
	return model.ComponentDefinition{
		Name:        reg.Name,
		Stability:   strings.Trim(reg.Stability.String(), `"`),
		IsCommunity: reg.Community,
		Metadata:    MetadataFromRegistration(reg),
		Arguments:   SchemaFields(reg.Args),
		Exports:     SchemaFields(reg.Exports),
		Validation:  ValidationMetadata(reg.Args),
	}
}

func MetadataFromRegistration(reg component.Registration) model.ComponentMetadata {
	return model.ComponentMetadata{
		Accepts: inferTypeNames(reg.Args),
		Exports: inferTypeNames(reg.Exports),
	}
}

func InstancesForDefinition(definition model.ComponentDefinition, instances []model.Component, moduleID string) []model.Component {
	var components []model.Component
	for _, instance := range instances {
		if instance.Name != definition.Name {
			continue
		}
		if moduleID != "" && instance.ModuleID != moduleID {
			continue
		}
		components = append(components, instance)
	}
	return components
}

func SchemaFields(value any) []model.ComponentSchemaField {
	if value == nil {
		return nil
	}

	ty := derefType(reflect.TypeOf(value))
	if ty.Kind() != reflect.Struct {
		return nil
	}

	return schemaFieldsForStruct(ty, nil)
}

func ValidationMetadata(value any) model.ComponentValidationMetadata {
	if value == nil {
		return model.ComponentValidationMetadata{}
	}

	ty := reflect.TypeOf(value)
	ptrTy := ty
	if ptrTy.Kind() != reflect.Pointer {
		ptrTy = reflect.PointerTo(ty)
	}

	return model.ComponentValidationMetadata{
		HasCustomValidator:   implementsEither(ty, ptrTy, reflect.TypeOf((*syntax.Validator)(nil)).Elem()),
		HasCustomDefaults:    implementsEither(ty, ptrTy, reflect.TypeOf((*syntax.Defaulter)(nil)).Elem()),
		HasCustomUnmarshaler: implementsEither(ty, ptrTy, reflect.TypeOf((*syntax.Unmarshaler)(nil)).Elem()),
	}
}

func schemaFieldsForStruct(ty reflect.Type, parentPath []string) []model.ComponentSchemaField {
	var fields []model.ComponentSchemaField
	for _, field := range reflect.VisibleFields(ty) {
		if field.Anonymous {
			continue
		}
		tag, ok := field.Tag.Lookup("alloy")
		if !ok || !field.IsExported() {
			continue
		}

		parsed, ok := parseAlloyTag(tag)
		if !ok {
			continue
		}

		if parsed.kind == model.ComponentSchemaFieldKindSquash {
			fields = append(fields, schemaFieldsForStruct(derefType(field.Type), parentPath)...)
			continue
		}

		name := parsed.name
		if name == "" && parsed.kind == model.ComponentSchemaFieldKindLabel {
			name = lowerFirst(field.Name)
		}
		path := append(append([]string{}, parentPath...), strings.Split(name, ".")...)

		fieldType := derefType(field.Type)
		isRepeated := fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array
		elementType := fieldType
		if isRepeated {
			elementType = derefType(fieldType.Elem())
		}

		schemaField := model.ComponentSchemaField{
			Name:       name,
			Path:       path,
			Kind:       parsed.kind,
			IsRequired: !parsed.optional,
			IsRepeated: isRepeated,
			GoType:     field.Type.String(),
			AlloyType:  alloyType(field.Type),
			Children:   nil,
		}

		if parsed.kind == model.ComponentSchemaFieldKindBlock && elementType.Kind() == reflect.Struct {
			schemaField.Children = schemaFieldsForStruct(elementType, path)
		}

		fields = append(fields, schemaField)
	}
	return fields
}

type parsedTag struct {
	name     string
	kind     model.ComponentSchemaFieldKind
	optional bool
}

func parseAlloyTag(tag string) (parsedTag, bool) {
	parts := strings.Split(tag, ",")
	if len(parts) < 2 {
		return parsedTag{}, false
	}

	parsed := parsedTag{name: parts[0]}
	for _, option := range parts[1:] {
		switch option {
		case "attr":
			parsed.kind = model.ComponentSchemaFieldKindAttribute
		case "block":
			parsed.kind = model.ComponentSchemaFieldKindBlock
		case "enum":
			parsed.kind = model.ComponentSchemaFieldKindEnum
		case "label":
			parsed.kind = model.ComponentSchemaFieldKindLabel
		case "squash":
			parsed.kind = model.ComponentSchemaFieldKindSquash
		case "optional":
			parsed.optional = true
		}
	}

	return parsed, parsed.kind != ""
}

func alloyType(ty reflect.Type) string {
	deref := derefType(ty)
	switch {
	case deref == reflect.TypeOf(time.Duration(0)):
		return "duration"
	case deref.Kind() == reflect.Slice || deref.Kind() == reflect.Array:
		return "list(" + alloyType(deref.Elem()) + ")"
	case deref.Kind() == reflect.Func:
		return "unsupported"
	case deref.Kind() == reflect.Struct:
		return "object"
	case deref.Kind() == reflect.Interface:
		return "any"
	}

	switch deref.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "number"
	case reflect.Float32, reflect.Float64:
		return "number"
	default:
		return deref.String()
	}
}

var signalTypes = []struct {
	name        string
	packagePath string
	typeName    string
}{
	{"Targets", "github.com/grafana/alloy/internal/component/discovery", "Target"},
	{"Loki `LogsReceiver`", "github.com/grafana/alloy/internal/component/common/loki", "LogsReceiver"},
	{"Prometheus `MetricsReceiver`", "github.com/prometheus/prometheus/storage", "Appendable"},
	{"Pyroscope `ProfilesReceiver`", "github.com/grafana/alloy/internal/component/pyroscope", "Appendable"},
	{"OpenTelemetry `otelcol.Consumer`", "github.com/grafana/alloy/internal/component/otelcol", "Consumer"},
}

func inferTypeNames(value any) []string {
	var names []string
	for _, signalType := range signalTypes {
		if hasType(reflect.TypeOf(value), signalType.packagePath, signalType.typeName, map[reflect.Type]bool{}) {
			names = append(names, signalType.name)
		}
	}
	sort.Strings(names)
	return names
}

func hasType(ty reflect.Type, packagePath, typeName string, visited map[reflect.Type]bool) bool {
	if ty == nil {
		return false
	}
	ty = derefType(ty)
	if ty.PkgPath() == packagePath && ty.Name() == typeName {
		return true
	}
	if visited[ty] {
		return false
	}
	visited[ty] = true

	if ty.Kind() == reflect.Slice || ty.Kind() == reflect.Array {
		return hasType(ty.Elem(), packagePath, typeName, visited)
	}
	if ty.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < ty.NumField(); i++ {
		if hasType(ty.Field(i).Type, packagePath, typeName, visited) {
			return true
		}
	}
	return false
}

func derefType(ty reflect.Type) reflect.Type {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	return ty
}

func implementsEither(ty reflect.Type, ptrTy reflect.Type, iface reflect.Type) bool {
	return ty.Implements(iface) || ptrTy.Implements(iface)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

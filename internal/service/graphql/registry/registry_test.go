package registry_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	"github.com/grafana/alloy/internal/service/graphql/registry"
)

func TestDefinitionsAreSortedAndIncludeMetadata(t *testing.T) {
	definitions := registry.DefinitionsFromRegistrations([]component.Registration{
		{
			Name:      "test.accepts",
			Stability: featuregate.StabilityGenerallyAvailable,
			Args:      acceptsTargetsArgs{},
		},
		{
			Name:      "test.exports",
			Stability: featuregate.StabilityPublicPreview,
			Community: true,
			Exports:   exportsTargets{},
		},
	})

	require.Len(t, definitions, 2)
	require.Equal(t, "test.accepts", definitions[0].Name)
	require.Equal(t, "generally-available", definitions[0].Stability)
	require.False(t, definitions[0].IsCommunity)
	require.Equal(t, []string{"Targets"}, definitions[0].Metadata.Accepts)
	require.Equal(t, "test.exports", definitions[1].Name)
	require.Equal(t, "public-preview", definitions[1].Stability)
	require.True(t, definitions[1].IsCommunity)
	require.Equal(t, []string{"Targets"}, definitions[1].Metadata.Exports)
}

func TestDefinitionReturnsNilWhenMissing(t *testing.T) {
	definition := registry.DefinitionFromRegistrations([]component.Registration{
		{Name: "prometheus.scrape", Args: schemaArgs{}},
	}, "prometheus.remote_write")

	require.Nil(t, definition)
}

func TestDefinitionInstancesFilterByComponentNameAndModuleID(t *testing.T) {
	definition := registry.DefinitionFromRegistration(component.Registration{Name: "prometheus.scrape", Args: schemaArgs{}})
	instances := registry.InstancesForDefinition(definition, []model.Component{
		{ID: "prometheus.scrape.root", Name: "prometheus.scrape"},
		{ID: "module_a/prometheus.scrape.module", ModuleID: "module_a", Name: "prometheus.scrape"},
		{ID: "module_a/prometheus.remote_write.module", ModuleID: "module_a", Name: "prometheus.remote_write"},
	}, "module_a")

	require.Len(t, instances, 1)
	require.Equal(t, "module_a/prometheus.scrape.module", instances[0].ID)
}

func TestSchemaFieldsReflectAttributesBlocksEnumsLabelsSlicesAndRequiredness(t *testing.T) {
	fields := registry.SchemaFields(schemaArgs{})

	require.Contains(t, fields, model.ComponentSchemaField{
		Name:       "targets",
		Path:       []string{"targets"},
		Kind:       model.ComponentSchemaFieldKindAttribute,
		IsRequired: true,
		IsRepeated: true,
		GoType:     "[]string",
		AlloyType:  "list(string)",
	})
	require.Contains(t, fields, model.ComponentSchemaField{
		Name:       "interval",
		Path:       []string{"interval"},
		Kind:       model.ComponentSchemaFieldKindAttribute,
		IsRequired: false,
		IsRepeated: false,
		GoType:     "time.Duration",
		AlloyType:  "duration",
	})

	block := findField(t, fields, "endpoint")
	require.Equal(t, model.ComponentSchemaFieldKindBlock, block.Kind)
	require.Equal(t, []string{"endpoint"}, block.Path)
	require.False(t, block.IsRequired)
	require.Equal(t, "registry_test.endpointBlock", block.GoType)
	require.Equal(t, "object", block.AlloyType)
	require.Contains(t, block.Children, model.ComponentSchemaField{
		Name:       "url",
		Path:       []string{"endpoint", "url"},
		Kind:       model.ComponentSchemaFieldKindAttribute,
		IsRequired: true,
		IsRepeated: false,
		GoType:     "string",
		AlloyType:  "string",
	})

	label := findField(t, fields, "label")
	require.Equal(t, model.ComponentSchemaFieldKindLabel, label.Kind)
	require.Equal(t, []string{"label"}, label.Path)

	enum := findField(t, fields, "mode")
	require.Equal(t, model.ComponentSchemaFieldKindEnum, enum.Kind)
	require.True(t, enum.IsRepeated)
}

func TestValidationMetadataDetectsCustomMethods(t *testing.T) {
	metadata := registry.ValidationMetadata(validatingArgs{})

	require.True(t, metadata.HasCustomValidator)
	require.True(t, metadata.HasCustomDefaults)
	require.True(t, metadata.HasCustomUnmarshaler)
}

func TestSchemaReflectionFallsBackForUnsupportedShapes(t *testing.T) {
	fields := registry.SchemaFields(unsupportedArgs{})

	require.Len(t, fields, 1)
	require.Equal(t, "bad", fields[0].Name)
	require.Equal(t, "func()", fields[0].GoType)
	require.Equal(t, "unsupported", fields[0].AlloyType)
}

func findField(t *testing.T, fields []model.ComponentSchemaField, name string) model.ComponentSchemaField {
	t.Helper()
	for _, field := range fields {
		if field.Name == name {
			return field
		}
	}
	require.Failf(t, "field not found", "missing field %q in %#v", name, fields)
	return model.ComponentSchemaField{}
}

type acceptsTargetsArgs struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type exportsTargets struct {
	Targets []discovery.Target `alloy:"targets,attr"`
}

type schemaArgs struct {
	Targets  []string       `alloy:"targets,attr"`
	Interval time.Duration  `alloy:"interval,attr,optional"`
	Endpoint endpointBlock  `alloy:"endpoint,block,optional"`
	Label    string         `alloy:",label"`
	Mode     []modeSelector `alloy:"mode,enum"`
}

type endpointBlock struct {
	URL string `alloy:"url,attr"`
}

type modeSelector struct {
	Fast *struct{} `alloy:"fast,block,optional"`
	Slow *struct{} `alloy:"slow,block,optional"`
}

type validatingArgs struct{}

func (validatingArgs) Validate() error { return nil }
func (*validatingArgs) SetToDefault()  {}
func (*validatingArgs) UnmarshalAlloy(func(any) error) error {
	return errors.New("not called")
}

type unsupportedArgs struct {
	Bad func() `alloy:"bad,attr"`
}

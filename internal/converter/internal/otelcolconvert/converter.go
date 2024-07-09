package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/token/builder"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

// ComponentConverter represents a converter which converts an OpenTelemetry
// Collector component into an Alloy component.
type ComponentConverter interface {
	// Factory should return the factory for the OpenTelemetry Collector
	// component.
	Factory() component.Factory

	// InputComponentName should return the name of the Alloy component where
	// other Alloy components forward OpenTelemetry data to.
	//
	// For example, a converter which emits a chain of components
	// (otelcol.receiver.prometheus -> prometheus.remote_write) should return
	// "otelcol.receiver.prometheus", which is the first component that receives
	// OpenTelemetry data in the chain.
	//
	// Converters which emit components that do not receive data from other
	// components must return an empty string.
	InputComponentName() string

	// ConvertAndAppend should convert the provided OpenTelemetry Collector
	// component configuration into Alloy configuration and append the result to
	// [state.Body]. Implementations are expected to append configuration where
	// all required arguments are set and all optional arguments are set to the
	// values from the input configuration or the Alloy default.
	//
	// ConvertAndAppend may be called more than once with the same component used
	// in different pipelines. Use [state.AlloyComponentLabel] to get a guaranteed
	// unique Alloy component label for the current state.
	ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics
}

// List of component converters. This slice is appended to by init functions in
// other files.
var converters []ComponentConverter

// State represents the State of the conversion. The State tracks:
//
//   - The OpenTelemetry Collector config being converted.
//   - The current OpenTelemetry Collector pipelines being converted.
//   - The current OpenTelemetry Collector component being converted.
type State struct {
	cfg   *otelcol.Config // Input config.
	file  *builder.File   // Output file.
	group *pipelineGroup  // Current pipeline group being converted.

	// converterLookup maps a converter key to the associated converter instance.
	converterLookup map[converterKey]ComponentConverter

	// extensionLookup maps OTel extensions to Alloy component IDs.
	extensionLookup map[component.ID]componentID

	componentID          component.InstanceID // ID of the current component being converted.
	componentConfig      component.Config     // Config of the current component being converted.
	componentLabelPrefix string               // Prefix for the label of the current component being converted.
}

type converterKey struct {
	Kind component.Kind
	Type component.Type
}

// Body returns the body of the file being generated. Implementations of
// [componentConverter] should use this to append components.
func (state *State) Body() *builder.Body { return state.file.Body() }

// AlloyComponentLabel returns the unique Alloy label for the OpenTelemetry
// Component component being converted. It is safe to use this label to create
// multiple Alloy components in a chain.
func (state *State) AlloyComponentLabel() string {
	return state.alloyLabelForComponent(state.componentID)
}

// alloyLabelForComponent returns the unique Alloy label for the given
// OpenTelemetry Collector component.
func (state *State) alloyLabelForComponent(c component.InstanceID) string {
	const defaultLabel = "default"

	// We need to prove that it's possible to statelessly compute the label for a
	// Alloy component just by using the group name and the otelcol component name:
	//
	// 1. OpenTelemetry Collector components are created once per pipeline, where
	//    the pipeline must have a unique key (a combination of telemetry type and
	//    an optional ID).
	//
	// 2. OpenTelemetry components must not appear in a pipeline more than once.
	//    Multiple references to receiver and exporter components get
	//    deduplicated, and multiple references to processor components gets
	//    rejected.
	//
	// 3. There is no other mechanism which constructs an OpenTelemetry
	//    receiver, processor, or exporter component.
	//
	// 4. Extension components are created once per service and are agnostic to
	//    pipelines.
	//
	// Considering the points above, the combination of group name and component
	// name is all that's needed to form a unique label for a single input
	// config.

	var (
		groupName     = state.group.Name
		componentName = c.ID.Name()
	)

	// We want to make the component label as idiomatic as possible. If both the
	// group and component name are empty, we'll name it "default," aligning
	// with standard Alloy naming conventions.
	//
	// Otherwise, we'll replace empty group and component names with "default"
	// and concatenate them with an underscore.
	unsanitizedLabel := state.componentLabelPrefix
	if unsanitizedLabel != "" {
		unsanitizedLabel += "_"
	}
	switch {
	case groupName == "" && componentName == "":
		unsanitizedLabel += defaultLabel

	default:
		if groupName == "" {
			groupName = defaultLabel
		}
		if componentName == "" {
			componentName = defaultLabel
		}
		unsanitizedLabel += fmt.Sprintf("%s_%s", groupName, componentName)
	}

	return common.SanitizeIdentifierPanics(unsanitizedLabel)
}

// Next returns the set of Alloy component IDs for a given data type that the
// current component being converted should forward data to.
func (state *State) Next(c component.InstanceID, dataType component.DataType) []componentID {
	instances := state.nextInstances(c, dataType)

	var ids []componentID

	for _, instance := range instances {
		key := converterKey{
			Kind: instance.Kind,
			Type: instance.ID.Type(),
		}

		// Look up the converter associated with the instance and retrieve the name
		// of the Alloy component expected to receive data.
		converter, found := state.converterLookup[key]
		if !found {
			panic(fmt.Sprintf("otelcolconvert: no component name found for converter key %v", key))
		}
		componentName := converter.InputComponentName()
		if componentName == "" {
			panic(fmt.Sprintf("otelcolconvert: converter %T returned empty component name", converter))
		}

		componentLabel := state.alloyLabelForComponent(instance)

		ids = append(ids, componentID{
			Name:  strings.Split(componentName, "."),
			Label: componentLabel,
		})
	}

	if len(ids) == 0 {
		return nil
	}
	return ids
}

func (state *State) nextInstances(c component.InstanceID, dataType component.DataType) []component.InstanceID {
	switch dataType {
	case component.DataTypeMetrics:
		return state.group.NextMetrics(c)
	case component.DataTypeLogs:
		return state.group.NextLogs(c)
	case component.DataTypeTraces:
		return state.group.NextTraces(c)

	default:
		panic(fmt.Sprintf("otelcolconvert: unknown data type %q", dataType))
	}
}

func (state *State) LookupExtension(id component.ID) componentID {
	cid, ok := state.extensionLookup[id]
	if !ok {
		panic(fmt.Sprintf("no component name found for extension %q", id.Name()))
	}
	return cid
}

type componentID struct {
	Name  []string
	Label string
}

func (id componentID) String() string {
	return strings.Join([]string{
		strings.Join(id.Name, "."),
		id.Label,
	}, ".")
}
